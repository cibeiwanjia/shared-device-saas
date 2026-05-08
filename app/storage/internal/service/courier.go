package service

import (
	"context"

	v1 "shared-device-saas/api/storage/v1"
	"shared-device-saas/app/storage/internal/biz"
	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// CourierService 快递员服务
type CourierService struct {
	v1.UnimplementedCourierServiceServer
	courierUsecase *biz.CourierUsecase
	log            *log.Helper
}

// NewCourierService 创建快递员服务
func NewCourierService(courierUsecase *biz.CourierUsecase, logger log.Logger) *CourierService {
	return &CourierService{
		courierUsecase: courierUsecase,
		log:            log.NewHelper(logger),
	}
}

// ============================================
// 1. 用户申请成为快递员
// ============================================

func (s *CourierService) ApplyCourier(ctx context.Context, req *v1.ApplyCourierRequest) (*v1.ApplyCourierReply, error) {
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	courier, err := s.courierUsecase.ApplyCourier(ctx, userID, req.RealName, req.IdCard, req.Phone, req.IntentAreas)
	if err != nil {
		return nil, err
	}

	return &v1.ApplyCourierReply{
		CourierId: courier.ID,
		Status:    courier.Status,
		Message:   "申请已提交，请等待审核",
	}, nil
}

// ============================================
// 2. 快递员查看自身信息
// ============================================

func (s *CourierService) GetCourierInfo(ctx context.Context, req *v1.GetCourierInfoRequest) (*v1.GetCourierInfoReply, error) {
	userID := getUserIDFromContext(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	courier, zones, err := s.courierUsecase.GetCourierInfo(ctx, userID)
	if err != nil {
		return nil, err
	}

	zoneInfos := make([]*v1.ZoneInfo, len(zones))
	for i, zone := range zones {
		zoneInfos[i] = &v1.ZoneInfo{
			ZoneId:     zone.ID,
			Name:       zone.Name,
			Street:     zone.Street,
			HouseStart: int32(zone.HouseStart),
			HouseEnd:   int32(zone.HouseEnd),
			Keywords:   zone.Keywords,
		}
	}

	return &v1.GetCourierInfoReply{
		CourierId:     courier.ID,
		UserId:        courier.UserID,
		RealName:      courier.RealName,
		Phone:         maskPhone(courier.Phone),
		Status:        courier.Status,
		StatusText:    biz.CourierStatusText(courier.Status),
		Zones:         zoneInfos,
		PendingCount:  int32(courier.PendingCount),
		CreateTime:    courier.CreateTime,
		UpdateTime:    courier.UpdateTime,
		RejectReason:  courier.RejectReason,
	}, nil
}

// ============================================
// 3. 管理员查询快递员列表
// ============================================

func (s *CourierService) GetCourierList(ctx context.Context, req *v1.GetCourierListRequest) (*v1.GetCourierListReply, error) {
	adminID := getUserIDFromContext(ctx)
	if adminID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	couriers, total, err := s.courierUsecase.GetCourierList(ctx, adminID, req.Status, page, pageSize)
	if err != nil {
		return nil, err
	}

	list := make([]*v1.CourierItem, len(couriers))
	for i, c := range couriers {
		list[i] = &v1.CourierItem{
			CourierId:    c.ID,
			UserId:       c.UserID,
			RealName:     c.RealName,
			IdCard:       c.IdCard,
			Phone:        c.Phone,
			Status:       c.Status,
			StatusText:   biz.CourierStatusText(c.Status),
			IntentAreas:  c.IntentAreas,
			ZoneIds:      c.ZoneIds,
			CreateTime:   c.CreateTime,
			RejectReason: c.RejectReason,
		}
	}

	return &v1.GetCourierListReply{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ============================================
// 4. 管理员审核快递员申请
// ============================================

func (s *CourierService) ApproveCourier(ctx context.Context, req *v1.ApproveCourierRequest) (*v1.ApproveCourierReply, error) {
	adminID := getUserIDFromContext(ctx)
	if adminID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	courier, err := s.courierUsecase.ApproveCourier(ctx, adminID, req.CourierId, req.Approved, req.Reason)
	if err != nil {
		return nil, err
	}

	message := "审核通过"
	if !req.Approved {
		message = "审核拒绝"
	}

	return &v1.ApproveCourierReply{
		CourierId: courier.ID,
		Status:    courier.Status,
		Message:   message,
	}, nil
}

// ============================================
// 5. 管理员分配片区
// ============================================

func (s *CourierService) AssignZone(ctx context.Context, req *v1.AssignZoneRequest) (*v1.AssignZoneReply, error) {
	adminID := getUserIDFromContext(ctx)
	if adminID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	courier, zones, err := s.courierUsecase.AssignZone(ctx, adminID, req.CourierId, req.ZoneIds)
	if err != nil {
		return nil, err
	}

	zoneInfos := make([]*v1.ZoneInfo, len(zones))
	for i, zone := range zones {
		zoneInfos[i] = &v1.ZoneInfo{
			ZoneId:     zone.ID,
			Name:       zone.Name,
			Street:     zone.Street,
			HouseStart: int32(zone.HouseStart),
			HouseEnd:   int32(zone.HouseEnd),
			Keywords:   zone.Keywords,
		}
	}

	return &v1.AssignZoneReply{
		CourierId: courier.ID,
		Zones:     zoneInfos,
		Message:   "片区分配成功",
	}, nil
}

// ============================================
// 6. 管理员创建片区
// ============================================

func (s *CourierService) CreateZone(ctx context.Context, req *v1.CreateZoneRequest) (*v1.CreateZoneReply, error) {
	adminID := getUserIDFromContext(ctx)
	if adminID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	zone, err := s.courierUsecase.CreateZone(ctx, adminID, req.Name, req.Street, int(req.HouseStart), int(req.HouseEnd), req.Keywords)
	if err != nil {
		return nil, err
	}

	return &v1.CreateZoneReply{
		ZoneId:  zone.ID,
		Name:    zone.Name,
		Message: "片区创建成功",
	}, nil
}

// ============================================
// 7. 管理员查询片区列表
// ============================================

func (s *CourierService) GetZoneList(ctx context.Context, req *v1.GetZoneListRequest) (*v1.GetZoneListReply, error) {
	adminID := getUserIDFromContext(ctx)
	if adminID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	zones, total, err := s.courierUsecase.GetZoneList(ctx, adminID, req.Street, page, pageSize)
	if err != nil {
		return nil, err
	}

	list := make([]*v1.ZoneItem, len(zones))
	for i, z := range zones {
		list[i] = &v1.ZoneItem{
			ZoneId:     z.ID,
			Name:       z.Name,
			Street:     z.Street,
			HouseStart: int32(z.HouseStart),
			HouseEnd:   int32(z.HouseEnd),
			Keywords:   z.Keywords,
			CourierId:  z.CourierId,
			Status:     z.Status,
			CreateTime: z.CreateTime,
		}
	}

	return &v1.GetZoneListReply{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ============================================
// 辅助方法
// ============================================

func getUserIDFromContext(ctx context.Context) string {
	return auth.GetUserID(ctx)
}