package service

import (
	"context"

	v1 "shared-device-saas/api/storage/v1"
	"shared-device-saas/app/storage/internal/biz"
	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/errors"
)

// ExpressService 快递服务
type ExpressService struct {
	v1.UnimplementedExpressServiceServer
	uc           *biz.ExpressUsecase
	courierUc    *biz.CourierUsecase
}

// NewExpressService 创建快递服务
func NewExpressService(uc *biz.ExpressUsecase, courierUc *biz.CourierUsecase) *ExpressService {
	return &ExpressService{uc: uc, courierUc: courierUc}
}

// CreateExpress 创建寄件订单（派单 + 生成取件码）
func (s *ExpressService) CreateExpress(ctx context.Context, req *v1.CreateExpressRequest) (*v1.CreateExpressReply, error) {
	// 从Token获取用户ID
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	// 构建订单实体
	order := &biz.ExpressOrder{
		SenderName:    req.SenderName,
		SenderAddress: req.SenderAddress,

		ReceiverName:     req.ReceiverName,
		ReceiverPhone:    req.ReceiverPhone,
		ReceiverProvince: req.ReceiverProvince,
		ReceiverCity:     req.ReceiverCity,
		ReceiverDistrict: req.ReceiverDistrict,
		ReceiverAddress:  req.ReceiverAddress,

		ItemType:   req.ItemType,
		ItemWeight: req.ItemWeight,
		ItemRemark: req.ItemRemark,

		ExpressType:     req.ExpressType,
		PickupTimeStart: req.PickupTimeStart,
		PickupTimeEnd:   req.PickupTimeEnd,
	}

	// 调用biz层创建订单（派单 + 生成取件码）
	created, dispatchResult, err := s.uc.CreateExpress(ctx, userID, order)
	if err != nil {
		return nil, err
	}

	// 构建响应
	reply := &v1.CreateExpressReply{
		OrderId:     created.ID,
		Status:      created.Status,
		ShortCode:   created.ShortCode,
		AssignedTime: created.AssignedTime,
	}

	// 添加派单信息
	if dispatchResult != nil {
		reply.CourierId = dispatchResult.CourierID
		reply.CourierName = maskName(dispatchResult.CourierName)
		reply.CourierPhone = "" // 快递员电话不直接返回给用户，通过短信通知
		reply.MatchLevel = int32(dispatchResult.MatchLevel)
		reply.Message = "派单成功，快递员将在指定时间上门取件"
	} else {
		reply.MatchLevel = 0
		reply.Message = "该区域暂无快递员，订单待人工分配"
	}

	return reply, nil
}

// ListExpress 我的快递列表
func (s *ExpressService) ListExpress(ctx context.Context, req *v1.ListExpressRequest) (*v1.ListExpressReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
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

	orders, total, err := s.uc.ListExpress(ctx, userID, req.Type, page, pageSize)
	if err != nil {
		return nil, err
	}

	list := make([]*v1.ExpressItem, 0, len(orders))
	for _, o := range orders {
		list = append(list, &v1.ExpressItem{
			Id:             o.ID,
			Status:         o.Status,
			StatusText:     biz.StatusText(o.Status),
			ReceiverName:   o.ReceiverName,
			ReceiverPhone:  maskPhone(o.ReceiverPhone),
			ReceiverAddress: o.ReceiverAddress,
			PickupCode:     o.PickupCode,
			CreateTime:     o.CreateTime,
			UpdateTime:     o.UpdateTime,
		})
	}

	return &v1.ListExpressReply{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetExpress 订单详情
func (s *ExpressService) GetExpress(ctx context.Context, req *v1.GetExpressRequest) (*v1.GetExpressReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	order, err := s.uc.GetExpress(ctx, userID, req.Id)
	if err != nil {
		return nil, err
	}

	reply := &v1.GetExpressReply{
		Id:               order.ID,
		Status:           order.Status,
		StatusText:       biz.StatusText(order.Status),
		SenderName:       order.SenderName,
		SenderPhone:      maskPhone(order.SenderPhone),
		SenderAddress:    order.SenderAddress,
		ReceiverName:     order.ReceiverName,
		ReceiverPhone:    maskPhone(order.ReceiverPhone),
		ReceiverProvince: order.ReceiverProvince,
		ReceiverCity:     order.ReceiverCity,
		ReceiverDistrict: order.ReceiverDistrict,
		ReceiverAddress:  order.ReceiverAddress,
		ItemType:         order.ItemType,
		ItemWeight:       order.ItemWeight,
		ItemRemark:       order.ItemRemark,
		ExpressType:      order.ExpressType,
		ExpressTypeText:  biz.ExpressTypeText(order.ExpressType),
		PickupTimeStart:  order.PickupTimeStart,
		PickupTimeEnd:    order.PickupTimeEnd,
		PickupCode:       order.PickupCode,
		ExpireTime:       order.ExpireTime,
		CreateTime:       order.CreateTime,
		UpdateTime:       order.UpdateTime,
		// Phase 2 新增
		CourierId:     order.CourierID,
		ShortCode:     order.ShortCode,
		AssignedTime:  order.AssignedTime,
	}

	// 添加快递员信息（如果已派单）
	if order.CourierID != "" {
		// 查询快递员信息（TODO: 缓存优化）
		reply.CourierName = "" // 不直接返回，通过短信通知
		reply.CourierPhone = ""
	}

	return reply, nil
}

// PickupExpress 确认取件（驿站取件）
func (s *ExpressService) PickupExpress(ctx context.Context, req *v1.PickupExpressRequest) (*v1.PickupExpressReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	order, err := s.uc.PickupExpress(ctx, userID, req.PickupCode)
	if err != nil {
		return nil, err
	}

	return &v1.PickupExpressReply{
		Success: true,
		OrderId: order.ID,
		Message: "取件成功",
	}, nil
}

// CancelExpress 取消订单
func (s *ExpressService) CancelExpress(ctx context.Context, req *v1.CancelExpressRequest) (*v1.CancelExpressReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	err := s.uc.CancelExpress(ctx, userID, req.Id)
	if err != nil {
		return nil, err
	}

	return &v1.CancelExpressReply{
		Success: true,
		Message: "取消成功",
	}, nil
}

// ============================================
// Phase 2 新增 RPC
// ============================================

// VerifyPickup 快递员上门核验
func (s *ExpressService) VerifyPickup(ctx context.Context, req *v1.VerifyPickupRequest) (*v1.VerifyPickupReply, error) {
	// 从Token获取快递员ID
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	// 查询快递员信息
	courier, _, err := s.courierUc.GetCourierInfo(ctx, userID)
	if err != nil {
		return nil, errors.New(403, "COURIER_NOT_FOUND", "快递员不存在")
	}

	// 调用核验逻辑
	order, err := s.uc.VerifyPickup(ctx, courier.ID, req.OrderId, req.InputShortCode)
	if err != nil {
		return &v1.VerifyPickupReply{
			Success:     false,
			OrderId:     req.OrderId,
			OrderStatus: 0,
			Message:     err.Error(),
		}, nil
	}

	return &v1.VerifyPickupReply{
		Success:     true,
		OrderId:     order.ID,
		OrderStatus: order.Status,
		Message:     "核验成功，订单已揽收",
	}, nil
}

// GetCourierPendingOrders 快递员待揽收列表
func (s *ExpressService) GetCourierPendingOrders(ctx context.Context, req *v1.GetCourierPendingOrdersRequest) (*v1.GetCourierPendingOrdersReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
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

	orders, total, err := s.uc.GetCourierPendingOrders(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	list := make([]*v1.PendingOrderItem, 0, len(orders))
	for _, o := range orders {
		list = append(list, &v1.PendingOrderItem{
			OrderId:         o.ID,
			SenderName:      maskName(o.SenderName),
			SenderPhone:     maskPhone(o.SenderPhone),
			SenderAddress:   o.SenderAddress,
			PickupTimeStart: o.PickupTimeStart,
			PickupTimeEnd:   o.PickupTimeEnd,
			AssignedTime:    o.AssignedTime,
			CreateTime:      o.CreateTime,
		})
	}

	return &v1.GetCourierPendingOrdersReply{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// CancelTimeoutOrder 取消超时订单
func (s *ExpressService) CancelTimeoutOrder(ctx context.Context, req *v1.CancelTimeoutOrderRequest) (*v1.CancelTimeoutOrderReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	err := s.uc.CancelTimeoutOrder(ctx, userID, req.OrderId)
	if err != nil {
		return nil, err
	}

	return &v1.CancelTimeoutOrderReply{
		Success: true,
		Message: "订单已取消",
	}, nil
}

// CheckCoverage 区域覆盖检查
func (s *ExpressService) CheckCoverage(ctx context.Context, req *v1.CheckCoverageRequest) (*v1.CheckCoverageReply, error) {
	hasCoverage, nearestStation := s.uc.CheckCoverage(ctx, req.PickupAddress)

	message := "该区域有快递员覆盖"
	if !hasCoverage {
		message = "该区域暂无快递员覆盖"
		if nearestStation != "" {
			message = "该区域暂无快递员覆盖，建议前往最近的驿站寄件"
		}
	}

	return &v1.CheckCoverageReply{
		HasCoverage:   hasCoverage,
		NearestStation: nearestStation,
		Message:       message,
	}, nil
}

// ManualAssignOrder 管理员人工派单
func (s *ExpressService) ManualAssignOrder(ctx context.Context, req *v1.ManualAssignOrderRequest) (*v1.ManualAssignOrderReply, error) {
	adminID := auth.GetUserID(ctx)
	if adminID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	order, err := s.uc.ManualAssignOrder(ctx, adminID, req.OrderId, req.CourierId)
	if err != nil {
		return nil, err
	}

	// 查询快递员信息
	courier, _, err := s.courierUc.GetCourierInfo(ctx, order.CourierID)
	if err != nil || courier == nil {
		courier = &biz.Courier{RealName: "未知"}
	}

	return &v1.ManualAssignOrderReply{
		OrderId:      order.ID,
		CourierId:    order.CourierID,
		CourierName:  courier.RealName,
		ShortCode:    order.ShortCode,
		AssignedTime: order.AssignedTime,
		Message:      "人工派单成功",
	}, nil
}

// ============================================
// 辅助方法
// ============================================

// maskPhone 手机号脱敏
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

// maskName 姓名脱敏
func maskName(name string) string {
	if len(name) <= 1 {
		return name
	}
	return name[:1] + "**"
}

// ============================================
// Phase 3 新增 RPC：投递闭环
// ============================================

// UpdateOrderStatus 更新订单状态
func (s *ExpressService) UpdateOrderStatus(ctx context.Context, req *v1.UpdateOrderStatusRequest) (*v1.UpdateOrderStatusReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	order, err := s.uc.UpdateOrderStatus(ctx, req.OrderId, req.Status, req.Remark)
	if err != nil {
		return nil, err
	}

	return &v1.UpdateOrderStatusReply{
		Success:     true,
		OrderId:     order.ID,
		Status:      order.Status,
		StatusText:  biz.StatusText(order.Status),
		Message:     "状态更新成功",
	}, nil
}

// DeliverToStation 驿站投递
func (s *ExpressService) DeliverToStation(ctx context.Context, req *v1.DeliverToStationRequest) (*v1.DeliverToStationReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	order, err := s.uc.DeliverToStation(ctx, req.OrderId, req.StationId)
	if err != nil {
		return nil, err
	}

	// 查询驿站信息
	station, err := s.uc.GetStation(ctx, req.StationId)
	if err != nil || station == nil {
		station = &biz.Station{Name: "未知驿站", Address: ""}
	}

	return &v1.DeliverToStationReply{
		Success:        true,
		OrderId:        order.ID,
		PickupCode:     order.PickupCode,
		StationName:    station.Name,
		StationAddress: station.Address,
		Status:         order.Status,
		Message:        "投递成功",
	}, nil
}

// PickupFromStation 驿站取件
func (s *ExpressService) PickupFromStation(ctx context.Context, req *v1.PickupFromStationRequest) (*v1.PickupFromStationReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	order, err := s.uc.PickupFromStation(ctx, req.PickupCode)
	if err != nil {
		return nil, err
	}

	return &v1.PickupFromStationReply{
		Success: true,
		OrderId: order.ID,
		Status:  order.Status,
		Message: "取件成功",
	}, nil
}

// DeliverToCabinet 快递柜投递
func (s *ExpressService) DeliverToCabinet(ctx context.Context, req *v1.DeliverToCabinetRequest) (*v1.DeliverToCabinetReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	order, grid, err := s.uc.DeliverToCabinet(ctx, req.OrderId, req.CabinetId, req.GridId, req.GridSize)
	if err != nil {
		return nil, err
	}

	// 查询快递柜信息
	cabinet, err := s.uc.GetCabinet(ctx, req.CabinetId)
	if err != nil || cabinet == nil {
		cabinet = &biz.Cabinet{Name: "未知快递柜", Address: ""}
	}

	return &v1.DeliverToCabinetReply{
		Success:        true,
		OrderId:        order.ID,
		GridId:         grid.ID,
		GridNo:         grid.GridNo,
		PickupCode:     order.PickupCode,
		CabinetName:    cabinet.Name,
		CabinetAddress: cabinet.Address,
		Status:         order.Status,
		Message:        "投递成功",
	}, nil
}

// PickupFromCabinet 快递柜取件
func (s *ExpressService) PickupFromCabinet(ctx context.Context, req *v1.PickupFromCabinetRequest) (*v1.PickupFromCabinetReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	order, grid, err := s.uc.PickupFromCabinet(ctx, req.PickupCode)
	if err != nil {
		return nil, err
	}

	return &v1.PickupFromCabinetReply{
		Success: true,
		OrderId: order.ID,
		GridId:  grid.ID,
		Status:  order.Status,
		Message: "取件成功",
	}, nil
}

// GetTrace 查询物流轨迹
func (s *ExpressService) GetTrace(ctx context.Context, req *v1.GetTraceRequest) (*v1.GetTraceReply, error) {
	trace, err := s.uc.GetTrace(ctx, req.OrderId)
	if err != nil {
		return nil, err
	}

	items := make([]*v1.TraceItem, 0, len(trace))
	for _, t := range trace {
		items = append(items, &v1.TraceItem{
			Status: t.Status,
			Time:   t.Time,
			Desc:   t.Desc,
		})
	}

	return &v1.GetTraceReply{
		OrderId: req.OrderId,
		Trace:   items,
	}, nil
}

// AppendTrace 追加轨迹项
func (s *ExpressService) AppendTrace(ctx context.Context, req *v1.AppendTraceRequest) (*v1.AppendTraceReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, errors.New(403, "PERMISSION_DENIED", "未登录")
	}

	err := s.uc.AppendTrace(ctx, req.OrderId, req.Status, req.Desc)
	if err != nil {
		return nil, err
	}

	return &v1.AppendTraceReply{
		Success:  true,
		OrderId:  req.OrderId,
		Message:  "轨迹追加成功",
	}, nil
}