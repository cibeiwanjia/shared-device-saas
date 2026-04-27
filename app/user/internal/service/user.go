package service

import (
	"context"
	"strconv"
	"strings"

	v1 "shared-device-saas/api/user/v1"
	"shared-device-saas/app/user/internal/biz"
	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
)

// UserService 统一用户服务（实现 proto 定义的 UserServiceServer）
type UserService struct {
	v1.UnimplementedUserServiceServer

	uc         *biz.UserUsecase
	orderUC    *biz.OrderUsecase
	uploadUC   *biz.UploadUsecase
	walletUC   *biz.WalletUsecase
	rechargeUC *biz.RechargeUsecase
	stationUC  *biz.StationUsecase
	log        *log.Helper
}

// NewUserService 创建统一用户服务
func NewUserService(
	uc *biz.UserUsecase,
	orderUC *biz.OrderUsecase,
	uploadUC *biz.UploadUsecase,
	walletUC *biz.WalletUsecase,
	rechargeUC *biz.RechargeUsecase,
	stationUC *biz.StationUsecase,
	logger log.Logger,
) *UserService {
	return &UserService{
		uc:         uc,
		orderUC:    orderUC,
		uploadUC:   uploadUC,
		walletUC:   walletUC,
		rechargeUC: rechargeUC,
		stationUC:  stationUC,
		log:        log.NewHelper(logger),
	}
}

// ========================================
// 用户认证相关（proto 定义的 RPC 方法）
// ========================================

// Register 用户注册（手机号 + 验证码 + 密码）
func (s *UserService) Register(ctx context.Context, req *v1.RegisterRequest) (*v1.RegisterReply, error) {
	id, err := s.uc.Register(ctx, req.Phone, req.SmsCode, req.Password, req.Nickname, req.InviteCode)
	if err != nil {
		return nil, err
	}
	return &v1.RegisterReply{Id: id}, nil
}

// Login 账号密码登录
func (s *UserService) Login(ctx context.Context, req *v1.LoginByPwdRequest) (*v1.LoginReply, error) {
	user, tokenPair, err := s.uc.LoginByPwd(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}
	return &v1.LoginReply{
		Id: user.ID, Username: user.Username, Phone: maskPhone(user.Phone),
		Nickname: user.Nickname, Avatar: user.Avatar,
		AccessToken: tokenPair.AccessToken, RefreshToken: tokenPair.RefreshToken, ExpiresIn: tokenPair.ExpiresIn,
	}, nil
}

// SendSmsCode 发送短信验证码
func (s *UserService) SendSmsCode(ctx context.Context, req *v1.SendSmsRequest) (*v1.SendSmsReply, error) {
	expire, err := s.uc.SendSms(ctx, req.Phone)
	if err != nil {
		return nil, err
	}
	return &v1.SendSmsReply{Success: true, Expire: expire}, nil
}

// LoginBySms 短信验证码登录（自动注册）
func (s *UserService) LoginBySms(ctx context.Context, req *v1.LoginBySmsRequest) (*v1.LoginReply, error) {
	user, tokenPair, err := s.uc.LoginBySms(ctx, req.Phone, req.Code)
	if err != nil {
		return nil, err
	}
	return &v1.LoginReply{
		Id: user.ID, Username: user.Username, Phone: maskPhone(user.Phone),
		Nickname: user.Nickname, Avatar: user.Avatar,
		AccessToken: tokenPair.AccessToken, RefreshToken: tokenPair.RefreshToken, ExpiresIn: tokenPair.ExpiresIn,
	}, nil
}

// RefreshToken 刷新 Token
func (s *UserService) RefreshToken(ctx context.Context, req *v1.RefreshTokenRequest) (*v1.LoginReply, error) {
	tokenPair, err := s.uc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	return &v1.LoginReply{
		AccessToken: tokenPair.AccessToken, RefreshToken: tokenPair.RefreshToken, ExpiresIn: tokenPair.ExpiresIn,
	}, nil
}

// GetMe 获取当前登录用户信息
func (s *UserService) GetMe(ctx context.Context, req *v1.GetMeRequest) (*v1.GetMeReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, biz.ErrInvalidToken
	}
	user, err := s.uc.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &v1.GetMeReply{
		Id: user.ID, Username: user.Username, Phone: maskPhone(user.Phone),
		Nickname: user.Nickname, Avatar: user.Avatar, Role: user.Role, CreateTime: user.CreateTime,
	}, nil
}

// GetUser 获取用户信息
func (s *UserService) GetUser(ctx context.Context, req *v1.GetUserRequest) (*v1.GetUserReply, error) {
	user, err := s.uc.GetUser(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.GetUserReply{
		Id: user.ID, Username: user.Username, Phone: maskPhone(user.Phone),
		Nickname: user.Nickname, Avatar: user.Avatar, CreateTime: user.CreateTime,
	}, nil
}

// UpdateUser 更新用户信息
func (s *UserService) UpdateUser(ctx context.Context, req *v1.UpdateUserRequest) (*v1.UpdateUserReply, error) {
	user, err := s.uc.UpdateUser(ctx, req.Id, req.Nickname, req.Avatar)
	if err != nil {
		return nil, err
	}
	return &v1.UpdateUserReply{Id: user.ID, Nickname: user.Nickname, Avatar: user.Avatar}, nil
}

// Logout 退出登录
func (s *UserService) Logout(ctx context.Context, req *v1.LogoutRequest) (*v1.LogoutReply, error) {
	success, message, err := s.uc.Logout(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	return &v1.LogoutReply{Success: success, Message: message}, nil
}

// ========================================
// 订单相关（proto 定义的 RPC 方法）
// ========================================

// CreateOrder 创建订单（source=3/4 时自动走库存校验事务）
func (s *UserService) CreateOrder(ctx context.Context, req *v1.CreateOrderRequest) (*v1.CreateOrderReply, error) {
	tenantID := auth.GetTenantID(ctx)
	userID := auth.GetUserID(ctx)

	// 充电宝/快递柜走库存校验路径
	if req.Source == 3 || req.Source == 4 {
		return s.createOrderWithInventory(ctx, tenantID, userID, req)
	}

	// 其他来源直接创建订单
	order, err := s.orderUC.CreateOrder(ctx, tenantID, userID,
		req.Source, req.OrderType, req.TotalAmount, req.Currency,
		req.Title, req.Description, req.ExtraJson,
	)
	if err != nil {
		return nil, err
	}
	return &v1.CreateOrderReply{Order: orderToProto(order)}, nil
}

// createOrderWithInventory 库存校验 + 创建订单（充电宝/快递柜专用）
// 库存参数通过 extra_json 传入：cabinet_id, cell_type, cell_id, station_id
func (s *UserService) createOrderWithInventory(ctx context.Context, tenantID int64, userID string, req *v1.CreateOrderRequest) (*v1.CreateOrderReply, error) {
	order := &biz.Order{
		TenantID:    tenantID,
		UserID:      userID,
		OrderNo:     biz.GenerateOrderNo(req.Source),
		Source:      req.Source,
		OrderType:   req.OrderType,
		Status:      biz.OrderStatusPending,
		TotalAmount: req.TotalAmount,
		Currency:    req.Currency,
		PaymentMethod: 0,
		Title:       req.Title,
		Description: req.Description,
		ExtraJSON:   req.ExtraJson,
	}
	if order.Currency == "" {
		order.Currency = "CNY"
	}

	// 从 extra_json 解析库存参数（短期方案，后续 proto 增加字段后替换）
	var cabinetID, cellID, stationID int64
	var cellType int32
	if req.Source == 4 {
		// 快递柜：解析 cabinet_id, cell_type, cell_id
		cabinetID = jsonFieldInt64(req.ExtraJson, "cabinet_id")
		cellType = jsonFieldInt32(req.ExtraJson, "cell_type")
		cellID = jsonFieldInt64(req.ExtraJson, "cell_id")
	} else if req.Source == 3 {
		// 充电宝：解析 station_id
		stationID = jsonFieldInt64(req.ExtraJson, "station_id")
	}

	alloc, err := s.orderUC.CreateOrderWithInventory(ctx, order, cabinetID, cellType, cellID, stationID)
	if err != nil {
		return nil, err
	}

	// 将分配结果追加到 extra_json
	if alloc != nil {
		// TODO: 将 alloc 信息合并到 order.ExtraJSON
		s.log.Infof("Inventory allocated: type=%s id=%d no=%s device=%s",
			alloc.ResourceType, alloc.ResourceID, alloc.ResourceNo, alloc.DeviceName)
	}

	return &v1.CreateOrderReply{Order: orderToProto(order)}, nil
}

// GetOrder 查询单个订单
func (s *UserService) GetOrder(ctx context.Context, req *v1.GetOrderRequest) (*v1.GetOrderReply, error) {
	tenantID := auth.GetTenantID(ctx)
	order, err := s.orderUC.GetOrder(ctx, tenantID, req.OrderNo)
	if err != nil {
		return nil, err
	}
	return &v1.GetOrderReply{Order: orderToProto(order)}, nil
}

// ListOrders 多维筛选+游标分页查询订单列表
func (s *UserService) ListOrders(ctx context.Context, req *v1.ListOrdersRequest) (*v1.ListOrdersReply, error) {
	tenantID := auth.GetTenantID(ctx)
	userID := auth.GetUserID(ctx)

	filter := &biz.OrderFilter{
		CreatedAfter:  req.CreatedAfter,
		CreatedBefore: req.CreatedBefore,
		Source:        req.Source,
		Status:        req.Status,
		MinAmount:     req.MinAmount,
		MaxAmount:     req.MaxAmount,
		PaymentMethod: req.PaymentMethod,
	}
	sort := &biz.OrderSort{
		Field:     req.SortField,
		Direction: req.SortDirection,
	}

	result, err := s.orderUC.ListOrders(ctx, tenantID, userID, filter, sort, int(req.Limit), req.Cursor)
	if err != nil {
		return nil, err
	}

	orders := make([]*v1.OrderInfo, len(result.Orders))
	for i, o := range result.Orders {
		orders[i] = orderToProto(o)
	}
	return &v1.ListOrdersReply{
		Orders:     orders,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
		TotalCount: result.TotalCount,
	}, nil
}

// CancelOrder 取消订单
func (s *UserService) CancelOrder(ctx context.Context, req *v1.CancelOrderRequest) (*v1.CancelOrderReply, error) {
	tenantID := auth.GetTenantID(ctx)
	order, err := s.orderUC.CancelOrder(ctx, tenantID, req.OrderNo)
	if err != nil {
		return nil, err
	}
	return &v1.CancelOrderReply{Order: orderToProto(order)}, nil
}

// PayOrder 发起支付
func (s *UserService) PayOrder(ctx context.Context, req *v1.PayOrderRequest) (*v1.PayOrderReply, error) {
	tenantID := auth.GetTenantID(ctx)
	payParams, err := s.orderUC.PayOrder(ctx, tenantID, req.OrderNo, req.PaymentMethod)
	if err != nil {
		return nil, err
	}
	return &v1.PayOrderReply{
		OrderNo:   req.OrderNo,
		PayParams: payParams,
	}, nil
}

// HandleCallback 支付回调入口
func (s *UserService) HandleCallback(ctx context.Context, req *v1.CallbackRequest) (*v1.CallbackReply, error) {
	channel := int32(0)
	if req.Channel == "wechat" || req.Channel == "1" {
		channel = 1
	} else if req.Channel == "alipay" || req.Channel == "2" {
		channel = 2
	}
	if channel == 0 {
		return nil, biz.ErrInvalidPayChannel
	}

	if err := s.orderUC.HandlePaymentCallback(ctx, channel, req.Body, ""); err != nil {
		return nil, err
	}
	return &v1.CallbackReply{Success: true, Message: "ok"}, nil
}

// orderToProto 将 biz.Order 转为 proto OrderInfo
func orderToProto(o *biz.Order) *v1.OrderInfo {
	return &v1.OrderInfo{
		Id:            o.ID,
		TenantId:      o.TenantID,
		UserId:        o.UserID,
		OrderNo:       o.OrderNo,
		Source:        o.Source,
		OrderType:     o.OrderType,
		Status:        o.Status,
		TotalAmount:   o.TotalAmount,
		Currency:      o.Currency,
		PaymentMethod: o.PaymentMethod,
		Title:         o.Title,
		Description:   o.Description,
		ExtraJson:     o.ExtraJSON,
		PaidAt:        o.PaidAt,
		CreatedAt:     o.CreatedAt,
		UpdatedAt:     o.UpdatedAt,
	}
}

// ========================================
// 站点相关（proto 定义的 RPC 方法）
// ========================================

// SearchNearbyStations 搜索附近站点
func (s *UserService) SearchNearbyStations(ctx context.Context, req *v1.SearchNearbyStationsRequest) (*v1.SearchNearbyStationsReply, error) {
	stations, totalCount, err := s.stationUC.SearchNearby(ctx, req.Lng, req.Lat, int(req.Radius), int(req.Limit))
	if err != nil {
		return nil, err
	}

	infos := make([]*v1.StationInfo, len(stations))
	for i, st := range stations {
		infos[i] = stationToProto(st)
	}

	return &v1.SearchNearbyStationsReply{
		Stations:   infos,
		TotalCount: totalCount,
	}, nil
}

// GetStation 获取站点详情
func (s *UserService) GetStation(ctx context.Context, req *v1.GetStationRequest) (*v1.GetStationReply, error) {
	station, grids, err := s.stationUC.GetStation(ctx, req.StationId)
	if err != nil {
		return nil, err
	}

	info := stationToProto(station)
	if len(grids) > 0 {
		info.Grids = make([]*v1.GridInfo, len(grids))
		for i, g := range grids {
			info.Grids[i] = &v1.GridInfo{
				Id:           g.ID,
				StationId:    g.StationID,
				GridNo:       g.GridNo,
				GridType:     g.GridType,
				Status:       g.Status,
				PricePerHour: g.PricePerHour,
			}
		}
	}

	return &v1.GetStationReply{Station: info}, nil
}

// stationToProto 将 biz.Station 转为 proto StationInfo
func stationToProto(s *biz.Station) *v1.StationInfo {
	return &v1.StationInfo{
		Id:             s.ID,
		Name:           s.Name,
		Address:        s.Address,
		Lat:            s.Lat,
		Lng:            s.Lng,
		Province:       s.Province,
		City:           s.City,
		District:       s.District,
		Status:         s.Status,
		Distance:       s.Distance,
		AvailableGrids: s.AvailableGrids,
		TotalGrids:     s.TotalGrids,
		MinPrice:       s.MinPrice,
	}
}

// maskPhone 手机号脱敏
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

// jsonFieldInt64 从 JSON 字符串中提取 int64 字段值（简单解析，不依赖外部库）
func jsonFieldInt64(jsonStr, key string) int64 {
	if jsonStr == "" {
		return 0
	}
	// 简单查找 "key":value 模式
	pattern := `"` + key + `":`
	idx := strings.Index(jsonStr, pattern)
	if idx < 0 {
		return 0
	}
	rest := jsonStr[idx+len(pattern):]
	// 跳过空格
	rest = strings.TrimLeft(rest, " ")
	// 读取数字
	var numStr string
	for _, c := range rest {
		if (c >= '0' && c <= '9') || c == '-' {
			numStr += string(c)
		} else {
			break
		}
	}
	if numStr == "" {
		return 0
	}
	v, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// jsonFieldInt32 从 JSON 字符串中提取 int32 字段值
func jsonFieldInt32(jsonStr, key string) int32 {
	return int32(jsonFieldInt64(jsonStr, key))
}
