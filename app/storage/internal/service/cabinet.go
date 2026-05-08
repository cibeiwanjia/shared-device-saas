package service

import (
	"context"
	"time"

	v1 "shared-device-saas/api/storage/v1"
	"shared-device-saas/app/storage/internal/biz"
	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/errors"
)

// CabinetService 快递柜服务
type CabinetService struct {
	v1.UnimplementedCabinetServiceServer
	cabinetRepo biz.CabinetRepo
	gridRepo    biz.CabinetGridRepo
}

// NewCabinetService 创建快递柜服务
func NewCabinetService(cabinetRepo biz.CabinetRepo, gridRepo biz.CabinetGridRepo) *CabinetService {
	return &CabinetService{cabinetRepo: cabinetRepo, gridRepo: gridRepo}
}

// ListCabinets 快递柜列表
func (s *CabinetService) ListCabinets(ctx context.Context, req *v1.ListCabinetsRequest) (*v1.ListCabinetsReply, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	cabinets, total, err := s.cabinetRepo.ListAll(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}

	list := make([]*v1.CabinetItem, 0, len(cabinets))
	for _, c := range cabinets {
		// 查询柜格统计（可选优化：缓存或聚合查询）
		grids, _ := s.gridRepo.FindByCabinet(ctx, c.ID)
		totalGrids := len(grids)
		availableGrids := 0
		for _, g := range grids {
			if g.Status == biz.GridStatusIdle {
				availableGrids++
			}
		}

		list = append(list, &v1.CabinetItem{
			Id:             c.ID,
			Name:           c.Name,
			Address:        c.Address,
			Lng:            c.Lng,
			Lat:            c.Lat,
			Status:         c.Status,
			StatusText:     biz.CabinetStatusText(c.Status),
			TotalGrids:     int32(totalGrids),
			AvailableGrids: int32(availableGrids),
			CreateTime:     c.CreateTime,
			UpdateTime:     c.UpdateTime,
		})
	}

	return &v1.ListCabinetsReply{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetCabinet 快递柜详情
func (s *CabinetService) GetCabinet(ctx context.Context, req *v1.GetCabinetRequest) (*v1.GetCabinetReply, error) {
	cabinet, err := s.cabinetRepo.FindByID(ctx, req.Id)
	if err != nil || cabinet == nil {
		return nil, errors.New(404, "CABINET_NOT_FOUND", "快递柜不存在")
	}

	// 查询柜格列表
	grids, _ := s.gridRepo.FindByCabinet(ctx, cabinet.ID)
	gridItems := make([]*v1.GridItem, 0, len(grids))
	for _, g := range grids {
		gridItems = append(gridItems, &v1.GridItem{
			Id:         g.ID,
			CabinetId:  g.CabinetID,
			GridNo:     g.GridNo,
			Size:       g.Size,
			Status:     g.Status,
			StatusText: biz.GridStatusText(g.Status),
			OrderId:    g.OrderID,
			CreateTime: g.CreateTime,
			UpdateTime: g.UpdateTime,
		})
	}

	return &v1.GetCabinetReply{
		Id:         cabinet.ID,
		Name:       cabinet.Name,
		Address:    cabinet.Address,
		Lng:        cabinet.Lng,
		Lat:        cabinet.Lat,
		Status:     cabinet.Status,
		StatusText: biz.CabinetStatusText(cabinet.Status),
		Grids:      gridItems,
		CreateTime: cabinet.CreateTime,
		UpdateTime: cabinet.UpdateTime,
	}, nil
}

// CreateCabinet 创建快递柜（管理员）
func (s *CabinetService) CreateCabinet(ctx context.Context, req *v1.CreateCabinetRequest) (*v1.CreateCabinetReply, error) {
	adminID := auth.GetUserID(ctx)
	if adminID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	now := time.Now().Format(time.RFC3339)
	cabinet := &biz.Cabinet{
		Name:       req.Name,
		Address:    req.Address,
		Lng:        req.Lng,
		Lat:        req.Lat,
		Status:     biz.CabinetStatusOnline,
		CreateTime: now,
		UpdateTime: now,
	}

	created, err := s.cabinetRepo.Create(ctx, cabinet)
	if err != nil {
		return nil, err
	}

	return &v1.CreateCabinetReply{
		Id:      created.ID,
		Name:    created.Name,
		Message: "快递柜创建成功",
	}, nil
}

// UpdateCabinet 更新快递柜（管理员）
func (s *CabinetService) UpdateCabinet(ctx context.Context, req *v1.UpdateCabinetRequest) (*v1.UpdateCabinetReply, error) {
	adminID := auth.GetUserID(ctx)
	if adminID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	cabinet, err := s.cabinetRepo.FindByID(ctx, req.Id)
	if err != nil || cabinet == nil {
		return nil, errors.New(404, "CABINET_NOT_FOUND", "快递柜不存在")
	}

	now := time.Now().Format(time.RFC3339)
	if req.Name != "" {
		cabinet.Name = req.Name
	}
	if req.Address != "" {
		cabinet.Address = req.Address
	}
	cabinet.Lng = req.Lng
	cabinet.Lat = req.Lat
	if req.Status != "" {
		cabinet.Status = req.Status
	}
	cabinet.UpdateTime = now

	updated, err := s.cabinetRepo.Update(ctx, cabinet)
	if err != nil {
		return nil, err
	}

	return &v1.UpdateCabinetReply{
		Id:      updated.ID,
		Name:    updated.Name,
		Status:  updated.Status,
		Message: "快递柜更新成功",
	}, nil
}

// ListGrids 柜格列表
func (s *CabinetService) ListGrids(ctx context.Context, req *v1.ListGridsRequest) (*v1.ListGridsReply, error) {
	grids, err := s.gridRepo.FindByCabinet(ctx, req.CabinetId)
	if err != nil {
		return nil, err
	}

	cabinet, _ := s.cabinetRepo.FindByID(ctx, req.CabinetId)
	cabinetName := ""
	if cabinet != nil {
		cabinetName = cabinet.Name
	}

	list := make([]*v1.GridItem, 0, len(grids))
	for _, g := range grids {
		list = append(list, &v1.GridItem{
			Id:         g.ID,
			CabinetId:  g.CabinetID,
			GridNo:     g.GridNo,
			Size:       g.Size,
			Status:     g.Status,
			StatusText: biz.GridStatusText(g.Status),
			OrderId:    g.OrderID,
			CreateTime: g.CreateTime,
			UpdateTime: g.UpdateTime,
		})
	}

	return &v1.ListGridsReply{
		List:        list,
		CabinetId:   req.CabinetId,
		CabinetName: cabinetName,
	}, nil
}

// GetAvailableGrids 空闲柜格列表
func (s *CabinetService) GetAvailableGrids(ctx context.Context, req *v1.GetAvailableGridsRequest) (*v1.GetAvailableGridsReply, error) {
	grids, err := s.gridRepo.FindAvailable(ctx, req.CabinetId)
	if err != nil {
		return nil, err
	}

	// 按大小过滤
	if req.Size != "" {
		filtered := make([]*biz.CabinetGrid, 0)
		for _, g := range grids {
			if g.Size == req.Size {
				filtered = append(filtered, g)
			}
		}
		grids = filtered
	}

	cabinet, _ := s.cabinetRepo.FindByID(ctx, req.CabinetId)
	cabinetName := ""
	if cabinet != nil {
		cabinetName = cabinet.Name
	}

	list := make([]*v1.GridItem, 0, len(grids))
	for _, g := range grids {
		list = append(list, &v1.GridItem{
			Id:         g.ID,
			CabinetId:  g.CabinetID,
			GridNo:     g.GridNo,
			Size:       g.Size,
			Status:     g.Status,
			StatusText: biz.GridStatusText(g.Status),
			OrderId:    g.OrderID,
			CreateTime: g.CreateTime,
			UpdateTime: g.UpdateTime,
		})
	}

	return &v1.GetAvailableGridsReply{
		List:        list,
		CabinetId:   req.CabinetId,
		CabinetName: cabinetName,
	}, nil
}

// ReleaseGrid 释放柜格（管理员/运维）
func (s *CabinetService) ReleaseGrid(ctx context.Context, req *v1.ReleaseGridRequest) (*v1.ReleaseGridReply, error) {
	adminID := auth.GetUserID(ctx)
	if adminID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	grid, err := s.gridRepo.ReleaseGrid(ctx, req.GridId)
	if err != nil {
		return nil, err
	}

	return &v1.ReleaseGridReply{
		Success: true,
		GridId:  grid.ID,
		GridNo:  grid.GridNo,
		Status:  grid.Status,
		Message: "柜格释放成功",
	}, nil
}