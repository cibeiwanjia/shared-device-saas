package service

import (
	"context"
	"time"

	v1 "shared-device-saas/api/storage/v1"
	"shared-device-saas/app/storage/internal/biz"
	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/errors"
)

// StationService 驿站服务
type StationService struct {
	v1.UnimplementedStationServiceServer
	stationRepo biz.StationRepo
}

// NewStationService 创建驿站服务
func NewStationService(stationRepo biz.StationRepo) *StationService {
	return &StationService{stationRepo: stationRepo}
}

// ListStations 驿站列表
func (s *StationService) ListStations(ctx context.Context, req *v1.ListStationsRequest) (*v1.ListStationsReply, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	stations, total, err := s.stationRepo.ListAll(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}

	list := make([]*v1.StationItem, 0, len(stations))
	for _, st := range stations {
		list = append(list, &v1.StationItem{
			Id:         st.ID,
			Name:       st.Name,
			Address:    st.Address,
			Lng:        st.Lng,
			Lat:        st.Lat,
			Status:     st.Status,
			StatusText: biz.StationStatusText(st.Status),
			CreateTime: st.CreateTime,
			UpdateTime: st.UpdateTime,
		})
	}

	return &v1.ListStationsReply{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetStation 驿站详情
func (s *StationService) GetStation(ctx context.Context, req *v1.GetStationRequest) (*v1.GetStationReply, error) {
	station, err := s.stationRepo.FindByID(ctx, req.Id)
	if err != nil || station == nil {
		return nil, errors.New(404, "STATION_NOT_FOUND", "驿站不存在")
	}

	return &v1.GetStationReply{
		Id:         station.ID,
		Name:       station.Name,
		Address:    station.Address,
		Lng:        station.Lng,
		Lat:        station.Lat,
		Status:     station.Status,
		StatusText: biz.StationStatusText(station.Status),
		CreateTime: station.CreateTime,
		UpdateTime: station.UpdateTime,
	}, nil
}

// CreateStation 创建驿站（管理员）
func (s *StationService) CreateStation(ctx context.Context, req *v1.CreateStationRequest) (*v1.CreateStationReply, error) {
	// 权限校验
	adminID := auth.GetUserID(ctx)
	if adminID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	now := time.Now().Format(time.RFC3339)
	station := &biz.Station{
		Name:       req.Name,
		Address:    req.Address,
		Lng:        req.Lng,
		Lat:        req.Lat,
		Status:     biz.StationStatusActive,
		CreateTime: now,
		UpdateTime: now,
	}

	created, err := s.stationRepo.Create(ctx, station)
	if err != nil {
		return nil, err
	}

	return &v1.CreateStationReply{
		Id:      created.ID,
		Name:    created.Name,
		Message: "驿站创建成功",
	}, nil
}

// UpdateStation 更新驿站（管理员）
func (s *StationService) UpdateStation(ctx context.Context, req *v1.UpdateStationRequest) (*v1.UpdateStationReply, error) {
	// 权限校验
	adminID := auth.GetUserID(ctx)
	if adminID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	station, err := s.stationRepo.FindByID(ctx, req.Id)
	if err != nil || station == nil {
		return nil, errors.New(404, "STATION_NOT_FOUND", "驿站不存在")
	}

	now := time.Now().Format(time.RFC3339)
	if req.Name != "" {
		station.Name = req.Name
	}
	if req.Address != "" {
		station.Address = req.Address
	}
	station.Lng = req.Lng
	station.Lat = req.Lat
	if req.Status != "" {
		station.Status = req.Status
	}
	station.UpdateTime = now

	updated, err := s.stationRepo.Update(ctx, station)
	if err != nil {
		return nil, err
	}

	return &v1.UpdateStationReply{
		Id:      updated.ID,
		Name:    updated.Name,
		Status:  updated.Status,
		Message: "驿站更新成功",
	}, nil
}