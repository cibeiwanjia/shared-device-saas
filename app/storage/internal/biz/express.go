package biz

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	v1 "shared-device-saas/api/storage/v1"
	"shared-device-saas/pkg/userclient"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// ============================================
// 错误定义
// ============================================

var (
	ErrExpressNotFound      = errors.NotFound(v1.ErrorReason_EXPRESS_NOT_FOUND.String(), "订单不存在")
	ErrInvalidPickupCode    = errors.BadRequest(v1.ErrorReason_INVALID_PICKUP_CODE.String(), "取件码无效")
	ErrPickupCodeExpired    = errors.BadRequest(v1.ErrorReason_PICKUP_CODE_EXPIRED.String(), "取件码已过期")
	ErrExpressAlreadyPicked = errors.BadRequest(v1.ErrorReason_EXPRESS_ALREADY_DELIVERED.String(), "订单已签收")
	ErrExpressCannotCancel  = errors.BadRequest(v1.ErrorReason_EXPRESS_CANNOT_CANCEL.String(), "只有待上门取件的订单可以取消")
	ErrPermissionDenied     = errors.Forbidden(v1.ErrorReason_PERMISSION_DENIED.String(), "无权操作该订单")
)

// ============================================
// 状态定义
// ============================================

const (
	StatusPendingPickup = 101 // 待上门取件（已派单）
	StatusPickedUp      = 102 // 已揽收
	StatusInTransit     = 103 // 运输中
	StatusAtStation     = 104 // 已到达驿站（生成取件码）
	StatusDelivered     = 105 // 已签收
	StatusCancelled     = 106 // 已取消
	StatusManualAssign  = 107 // 待人工分配（无匹配快递员）
	StatusTimeoutPickup = 108 // 超时未取件
	StatusVerifyFailed  = 109 // 核验失败（审计记录）
)

// StatusText 状态文本
func StatusText(status int32) string {
	switch status {
	case StatusPendingPickup:
		return "待上门取件"
	case StatusPickedUp:
		return "已揽收"
	case StatusInTransit:
		return "运输中"
	case StatusAtStation:
		return "已到达驿站"
	case StatusDelivered:
		return "已签收"
	case StatusCancelled:
		return "已取消"
	case StatusManualAssign:
		return "待人工分配"
	case StatusTimeoutPickup:
		return "超时未取件"
	case StatusVerifyFailed:
		return "核验失败"
	default:
		return "未知状态"
	}
}

// ExpressTypeText 快递类型文本
func ExpressTypeText(expressType int32) string {
	switch expressType {
	case 1:
		return "普通"
	case 2:
		return "加急"
	default:
		return "未知"
	}
}

// ============================================
// ExpressOrder 聚合根
// ============================================

// ExpressOrder 快递订单实体
type ExpressOrder struct {
	ID          string // MongoDB ObjectID.Hex()
	UserID      string // 寄件用户ID（Token解析）
	SenderPhone string // 寄件人电话

	// 寄件人
	SenderName    string // 寄件人姓名
	SenderAddress string // 寄件人地址

	// 收件人
	ReceiverName     string // 收件人姓名
	ReceiverPhone    string // 冗余存储，取件查询用
	ReceiverProvince string // 收件人省份
	ReceiverCity     string // 收件人城市
	ReceiverDistrict string // 收件人区县
	ReceiverAddress  string // 收件人地址

	// 物品
	ItemType   string  // 物品类型
	ItemWeight float64 // 物品重量
	ItemRemark string  // 物品备注

	// 快递
	ExpressType     int32  // 快递类型
	PickupTimeStart string // 上�件时间开始 RFC3339
	PickupTimeEnd   string // 上�件时间结束 RFC3339

	// 状态
	Status     int32  // 订单状态
	PickupCode string // 取件码（6位数字）- 用于驿站取件
	ExpireTime string // 取件码过期时间 RFC3339

	// 派单相关（Phase 2 新增）
	CourierID       string // 分配的快递员ID
	ShortCode       string // 寄件核验码（6位随机）- 上门取件时核验
	ShortCodeUsed   bool   // 核验码是否已使用
	AssignedTime    string // 派单时间 RFC3339
	TimeoutNotified bool   // 超时通知标记

	// 扩展
	StationID    string // 驿站ID
	CabinetID    string // 柜子ID
	GridID       string // 柜格ID（快递柜模式）
	DeliveryType int32  // 投递类型：1=驿站 2=快递柜
	Trace        []TraceItem // 物流轨迹数组

	// 时间
	CreateTime string // 创建时间，格式 RFC3339
	UpdateTime string // 更新时间，格式 RFC3339
}

// TraceItem 物流轨迹项
type TraceItem struct {
	Status int32  // 订单状态
	Time   string // 时间 RFC3339
	Desc   string // 状态描述
}

// VerificationLog 核验审计日志实体
type VerificationLog struct {
	ID             string // MongoDB ObjectID.Hex()
	OrderID        string // 订单ID
	CourierID      string // 核验快递员ID
	InputShortCode string // 输入的取件码
	Result         string // success/failed
	FailReason     string // code_mismatch/courier_mismatch/code_used/order_invalid
	Timestamp      string // RFC3339
}

// ============================================
// ExpressRepo 仓储接口
// ============================================

// ExpressRepo 快递订单仓储接口
type ExpressRepo interface {
	Create(context.Context, *ExpressOrder) (*ExpressOrder, error)
	FindByID(context.Context, string) (*ExpressOrder, error)
	FindByPickupCode(context.Context, string) (*ExpressOrder, error)
	FindBySenderPhone(context.Context, string, int32, int32) ([]*ExpressOrder, int64, error)
	FindByReceiverPhone(context.Context, string, []int32, int32, int32) ([]*ExpressOrder, int64, error)
	Update(context.Context, *ExpressOrder) (*ExpressOrder, error)
	// Phase 2 新增
	FindByStatus(context.Context, int32, int32, int32) ([]*ExpressOrder, int64, error)     // 查询指定状态订单
	FindByCourierID(context.Context, string, int32, int32) ([]*ExpressOrder, int64, error) // 快递员待揽收列表
	FindTimeoutOrders(context.Context, string) ([]*ExpressOrder, error)                    // 超时订单扫描
	// Phase 3 新增
	UpdateStatus(context.Context, string, int32) (*ExpressOrder, error)         // 原子更新状态
	AppendTrace(context.Context, string, *TraceItem) (*ExpressOrder, error)     // 原子追加轨迹
	UpdateDeliveryInfo(context.Context, string, int32, string, string, string) (*ExpressOrder, error) // 更新投递信息
}

// VerificationLogRepo 核验审计日志仓储接口
type VerificationLogRepo interface {
	Create(context.Context, *VerificationLog) (*VerificationLog, error)
	FindByOrderID(context.Context, string) ([]*VerificationLog, error) // 查询订单核验历史
}

// ============================================
// ExpressUsecase 用例
// ============================================

// ExpressUsecase 快递用例
type ExpressUsecase struct {
	repo          ExpressRepo            // 快递订单仓储
	verifyLogRepo VerificationLogRepo    // 核验审计日志仓储
	courierRepo   CourierRepo            // 快递员仓储
	zoneRepo      ZoneRepo               // 片区仓储
	stationRepo   StationRepo            // 驿站仓储
	cabinetRepo   CabinetRepo            // 快递柜仓储
	gridRepo      CabinetGridRepo        // 柜格仓储
	dispatchSvc   *DispatchService       // 派单服务
	userClient    *userclient.UserClient // 用户服务客户端
	log           *log.Helper            // 日志助手
}

// NewExpressUsecase 创建快递用例
func NewExpressUsecase(
	repo ExpressRepo,                  // 快递订单仓储
	verifyLogRepo VerificationLogRepo, // 核验审计日志仓储
	courierRepo CourierRepo,           // 快递员仓储
	zoneRepo ZoneRepo,                 // 片区仓储
	stationRepo StationRepo,           // 驿站仓储
	cabinetRepo CabinetRepo,           // 快递柜仓储
	gridRepo CabinetGridRepo,          // 柜格仓储
	dispatchSvc *DispatchService,      // 派单服务
	userClient *userclient.UserClient, // 用户服务客户端
	logger log.Logger,                 // 日志记录器
) *ExpressUsecase {
	return &ExpressUsecase{
		repo:          repo,                  // 快递订单仓储
		verifyLogRepo: verifyLogRepo,         // 核验审计日志仓储
		courierRepo:   courierRepo,           // 快递员仓储
		zoneRepo:      zoneRepo,              // 片区仓储
		stationRepo:   stationRepo,           // 驿站仓储
		cabinetRepo:   cabinetRepo,           // 快递柜仓储
		gridRepo:      gridRepo,              // 柜格仓储
		dispatchSvc:   dispatchSvc,           // 派单服务
		userClient:    userClient,            // 用户服务客户端
		log:           log.NewHelper(logger), // 日志助手
	}
}

// ============================================
// 1. 创建寄件订单（派单 + 生成取件码）
// ============================================

func (uc *ExpressUsecase) CreateExpress(ctx context.Context, userID string, req *ExpressOrder) (*ExpressOrder, *DispatchResult, error) {
	// 1. 从user服务获取用户手机号
	userInfo, err := uc.userClient.GetUserById(ctx, userID)
	if err != nil {
		uc.log.Errorf("GetUserById failed: %v", err)
		return nil, nil, err
	}

	// 2. 设置默认值
	now := time.Now().Format(time.RFC3339) // 格式化当前时间为 RFC3339 格式
	req.UserID = userID                    // 设置用户ID
	req.SenderPhone = userInfo.Phone       // 冗余存储寄件人电话
	req.CreateTime = now                   // 设置创建时间
	req.UpdateTime = now                   // 设置更新时间

	// 3. 执行派单算法
	dispatchResult, err := uc.dispatchSvc.Dispatch(ctx, req.SenderAddress)
	if err != nil {
		uc.log.Errorf("Dispatch failed: %v", err)
		return nil, nil, err
	}

	// 4. 根据派单结果设置订单状态
	if dispatchResult != nil {
		// 派单成功
		req.Status = StatusPendingPickup               // 设置订单状态为待揽件
		req.CourierID = dispatchResult.CourierID       // 设置快递员ID
		req.ShortCode = dispatchResult.ShortCode       // 设置取件码
		req.ShortCodeUsed = false                      // 设置取件码未使用
		req.AssignedTime = dispatchResult.AssignedTime // 设置派单时间
		req.TimeoutNotified = false                    // 设置超时通知未发送
	} else {
		// 无匹配快递员，标记为待人工分配
		req.Status = StatusManualAssign // 设置订单状态为待人工分配
		req.CourierID = ""
		req.ShortCode = ""
		uc.log.Warnf("No courier matched, order marked as manual assign: userId=%s", userID)
	}

	// 5. 创建订单
	order, err := uc.repo.Create(ctx, req)
	if err != nil {
		uc.log.Errorf("Create express order failed: %v", err)
		return nil, nil, err
	}

	// 6. 更新快递员待揽收统计（仅用于展示）
	if dispatchResult != nil {
		courier, err := uc.courierRepo.FindByID(ctx, dispatchResult.CourierID)
		if err == nil && courier != nil {
			courier.PendingCount += 1
			courier.UpdateTime = now
			uc.courierRepo.Update(ctx, courier)
		}
	}

	uc.log.Infof("Express order created: id=%s, userId=%s, status=%d, courierId=%s", order.ID, userID, order.Status, order.CourierID)
	return order, dispatchResult, nil
}

// ============================================
// 2. 我的快递列表
// ============================================

func (uc *ExpressUsecase) ListExpress(ctx context.Context, userID string, listType int32, page, pageSize int32) ([]*ExpressOrder, int64, error) {
	// 1. 从user服务获取用户手机号
	userInfo, err := uc.userClient.GetUserById(ctx, userID)
	if err != nil {
		uc.log.Errorf("GetUserById failed: %v", err)
		return nil, 0, err
	}

	// 2. 根据类型查询
	if listType == 1 {
		// 待收件：receiverPhone = user.phone, status IN (103, 104)
		statuses := []int32{StatusInTransit, StatusAtStation}
		return uc.repo.FindByReceiverPhone(ctx, userInfo.Phone, statuses, page, pageSize)
	} else {
		// 已寄出：senderPhone = user.phone
		return uc.repo.FindBySenderPhone(ctx, userInfo.Phone, page, pageSize)
	}
}

// ============================================
// 3. 订单详情
// ============================================

func (uc *ExpressUsecase) GetExpress(ctx context.Context, userID, orderID string) (*ExpressOrder, error) {
	// 1. 查询订单
	order, err := uc.repo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, ErrExpressNotFound
	}

	// 2. 权限校验：只能查看自己相关的订单
	userInfo, err := uc.userClient.GetUserById(ctx, userID)
	if err != nil {
		return nil, err
	}

	if order.SenderPhone != userInfo.Phone && order.ReceiverPhone != userInfo.Phone {
		return nil, ErrPermissionDenied
	}

	return order, nil
}

// ============================================
// 4. 确认取件
// ============================================

func (uc *ExpressUsecase) PickupExpress(ctx context.Context, userID string, pickupCode string) (*ExpressOrder, error) {
	// 1. 根据取件码查询订单
	order, err := uc.repo.FindByPickupCode(ctx, pickupCode)
	if err != nil || order == nil {
		return nil, ErrInvalidPickupCode
	}

	// 2. 校验状态
	if order.Status != StatusAtStation {
		return nil, ErrExpressAlreadyPicked
	}

	// 3. 校验取件码过期
	if order.ExpireTime != "" {
		expireTime, err := time.Parse(time.RFC3339, order.ExpireTime)
		if err == nil && time.Now().After(expireTime) {
			return nil, ErrPickupCodeExpired
		}
	}

	// 4. 权限校验：receiverPhone必须匹配
	userInfo, err := uc.userClient.GetUserById(ctx, userID)
	if err != nil {
		return nil, err
	}

	if order.ReceiverPhone != userInfo.Phone {
		return nil, ErrPermissionDenied
	}

	// 5. 更新状态为已签收
	order.Status = StatusDelivered
	order.UpdateTime = time.Now().Format(time.RFC3339)
	order.PickupCode = "" // 清空取件码

	updated, err := uc.repo.Update(ctx, order)
	if err != nil {
		uc.log.Errorf("Update express order failed: %v", err)
		return nil, err
	}

	uc.log.Infof("Express picked up: id=%s, userId=%s", updated.ID, userID)
	return updated, nil
}

// ============================================
// 5. 取消订单
// ============================================

func (uc *ExpressUsecase) CancelExpress(ctx context.Context, userID, orderID string) error {
	// 1. 查询订单
	order, err := uc.repo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return ErrExpressNotFound
	}

	// 2. 校验状态：只有待上门取件(101)可取消
	if order.Status != StatusPendingPickup {
		return ErrExpressCannotCancel
	}

	// 3. 权限校验：只有寄件人可以取消
	userInfo, err := uc.userClient.GetUserById(ctx, userID)
	if err != nil {
		return err
	}

	if order.SenderPhone != userInfo.Phone {
		return ErrPermissionDenied
	}

	// 4. 更新状态为已取消
	order.Status = StatusCancelled
	order.UpdateTime = time.Now().Format(time.RFC3339)

	_, err = uc.repo.Update(ctx, order)
	if err != nil {
		uc.log.Errorf("Cancel express order failed: %v", err)
		return err
	}

	uc.log.Infof("Express cancelled: id=%s, userId=%s", orderID, userID)
	return nil
}

// ============================================
// 6. 生成取件码（内部方法）
// ============================================

// GeneratePickupCode 生成6位随机取件码
func GeneratePickupCode() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

// ============================================
// Phase 2 新增方法
// ============================================

// 7. 快递员上门核验（双重校验）
func (uc *ExpressUsecase) VerifyPickup(ctx context.Context, courierID, orderID, inputShortCode string) (*ExpressOrder, error) {
	// 1. 查询订单
	order, err := uc.repo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, errors.New(404, "EXPRESS_NOT_FOUND", "订单不存在")
	}

	// 2. 校验订单状态（只有 101 待上门取件 可核验）
	if order.Status != StatusPendingPickup {
		return nil, errors.New(400, "ORDER_STATUS_INVALID", "订单状态异常，无法核验")
	}

	// 3. 校验快递员身份（双重校验第一层）
	if order.CourierID != courierID {
		// 记录核验失败日志
		uc.createVerifyLog(ctx, orderID, courierID, inputShortCode, "failed", "courier_mismatch")
		return nil, errors.New(403, "COURIER_MISMATCH", "非本订单派单快递员，无法核验")
	}

	// 4. 校验取件核验码（双重校验第二层）
	if order.ShortCode != inputShortCode {
		// 记录核验失败日志
		uc.createVerifyLog(ctx, orderID, courierID, inputShortCode, "failed", "code_mismatch")
		return nil, errors.New(400, "SHORT_CODE_MISMATCH", "取件码错误，请核实后重试")
	}

	// 5. 校验核验码是否已使用
	if order.ShortCodeUsed {
		uc.createVerifyLog(ctx, orderID, courierID, inputShortCode, "failed", "code_used")
		return nil, errors.New(409, "SHORT_CODE_USED", "取件码已失效，订单已完成揽收")
	}

	// 6. 核验成功处理
	order.Status = StatusPickedUp
	order.ShortCode = "" // 置空，防止二次使用
	order.ShortCodeUsed = true
	order.UpdateTime = time.Now().Format(time.RFC3339)

	updated, err := uc.repo.Update(ctx, order)
	if err != nil {
		uc.log.Errorf("Update order after verify failed: %v", err)
		return nil, err
	}

	// 7. 更新快递员待揽收统计（核验成功，减少待揽收数）
	courier, err := uc.courierRepo.FindByID(ctx, courierID)
	if err == nil && courier != nil {
		courier.PendingCount -= 1
		if courier.PendingCount < 0 {
			courier.PendingCount = 0 // 防止负数
		}
		courier.UpdateTime = time.Now().Format(time.RFC3339)
		uc.courierRepo.Update(ctx, courier)
	}

	// 8. 记录核验成功日志
	uc.createVerifyLog(ctx, orderID, courierID, inputShortCode, "success", "")

	uc.log.Infof("Pickup verified: orderId=%s, courierId=%s", orderID, courierID)
	return updated, nil
}

// createVerifyLog 创建核验审计日志
func (uc *ExpressUsecase) createVerifyLog(ctx context.Context, orderID, courierID, inputCode, result, failReason string) {
	logEntry := &VerificationLog{
		OrderID:        orderID,
		CourierID:      courierID,
		InputShortCode: inputCode,
		Result:         result,
		FailReason:     failReason,
		Timestamp:      time.Now().Format(time.RFC3339),
	}
	uc.verifyLogRepo.Create(ctx, logEntry)
}

// 8. 快递员待揽收列表
func (uc *ExpressUsecase) GetCourierPendingOrders(ctx context.Context, userID string, page, pageSize int32) ([]*ExpressOrder, int64, error) {
	// 1. 根据用户ID查询快递员
	courier, err := uc.courierRepo.FindByUserID(ctx, userID)
	if err != nil || courier == nil {
		return nil, 0, errors.New(404, "COURIER_NOT_FOUND", "快递员不存在")
	}

	// 2. 校验快递员状态
	if courier.Status != CourierStatusActive {
		return nil, 0, errors.New(400, "COURIER_NOT_ACTIVE", "快递员状态异常")
	}

	// 3. 查询待揽收订单
	orders, total, err := uc.repo.FindByCourierID(ctx, courier.ID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

// 10. 取消超时订单
func (uc *ExpressUsecase) CancelTimeoutOrder(ctx context.Context, userID, orderID string) error {
	// 1. 查询订单
	order, err := uc.repo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return errors.New(404, "EXPRESS_NOT_FOUND", "订单不存在")
	}

	// 2. 校验订单状态（只有 108 超时未取件 可取消）
	if order.Status != StatusTimeoutPickup {
		return errors.New(400, "ORDER_STATUS_INVALID", "订单状态异常，无法取消")
	}

	// 3. 权限校验：只有寄件人可以取消
	userInfo, err := uc.userClient.GetUserById(ctx, userID)
	if err != nil {
		return err
	}
	if order.SenderPhone != userInfo.Phone {
		return errors.New(403, "PERMISSION_DENIED", "无权操作该订单")
	}

	// 4. 更新订单状态
	order.Status = StatusCancelled
	order.ShortCode = "" // 取件码失效
	order.UpdateTime = time.Now().Format(time.RFC3339)

	_, err = uc.repo.Update(ctx, order)
	if err != nil {
		return err
	}

	uc.log.Infof("Timeout order cancelled: orderId=%s, userId=%s", orderID, userID)
	return nil
}

// 11. 区域覆盖检查
func (uc *ExpressUsecase) CheckCoverage(ctx context.Context, address string) (bool, string) {
	hasCoverage, nearestStation := uc.dispatchSvc.CheckCoverage(ctx, address)
	return hasCoverage, nearestStation
}

// 12. 管理员人工派单
func (uc *ExpressUsecase) ManualAssignOrder(ctx context.Context, adminID, orderID, courierID string) (*ExpressOrder, error) {
	// 1. 权限校验（管理员）
	// TODO: 添加管理员权限校验

	// 2. 查询订单
	order, err := uc.repo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, errors.New(404, "EXPRESS_NOT_FOUND", "订单不存在")
	}

	// 3. 校验订单状态（只有 107 待人工分配 可人工派单）
	if order.Status != StatusManualAssign {
		return nil, errors.New(400, "ORDER_STATUS_INVALID", "订单状态异常，无需人工派单")
	}

	// 4. 校验快递员状态
	courier, err := uc.courierRepo.FindByID(ctx, courierID)
	if err != nil || courier == nil {
		return nil, errors.New(404, "COURIER_NOT_FOUND", "快递员不存在")
	}
	if courier.Status != CourierStatusActive {
		return nil, errors.New(400, "COURIER_NOT_ACTIVE", "快递员状态异常")
	}

	// 5. 更新订单
	order.Status = StatusPendingPickup                   // 设置订单状态为待取件
	order.CourierID = courierID                          // 设置快递员ID为派单快递员
	order.ShortCode = GenerateShortCode()                // 生成新的取件码
	order.ShortCodeUsed = false                          // 取件码未使用
	order.AssignedTime = time.Now().Format(time.RFC3339) // 设置派单时间
	order.TimeoutNotified = false                        // 设置超时通知未发送
	order.UpdateTime = time.Now().Format(time.RFC3339)   // 更新时间

	updated, err := uc.repo.Update(ctx, order) // 更新订单
	if err != nil {
		return nil, err
	}

	// 6. 更新快递员待揽收统计
	courier.PendingCount += 1
	courier.UpdateTime = time.Now().Format(time.RFC3339)
	uc.courierRepo.Update(ctx, courier)

	uc.log.Infof("Order manually assigned: orderId=%s, courierId=%s, by=%s", orderID, courierID, adminID)
	return updated, nil
}

// ============================================
// Phase 3 新增方法：投递闭环
// ============================================

// 状态流转规则：只能正向顺序流转
var statusFlowMap = map[int32]int32{
	StatusPickedUp:  StatusInTransit,  // 102 → 103
	StatusInTransit: StatusAtStation,  // 103 → 104
	StatusAtStation: StatusDelivered,  // 104 → 105
}

// validateStatusTransition 校验状态流转是否合法
func validateStatusTransition(currentStatus, targetStatus int32) error {
	// 允许的特殊流转
	if currentStatus == StatusPendingPickup && targetStatus == StatusPickedUp {
		return nil // 101 → 102（上门核验）
	}

	// 检查正常顺序流转
	nextStatus, ok := statusFlowMap[currentStatus]
	if !ok {
		return errors.New(400, "ORDER_STATUS_INVALID", "当前状态不允许流转")
	}
	if targetStatus != nextStatus {
		return errors.New(400, "ORDER_STATUS_INVALID", "状态流转不合法，只能正向顺序流转")
	}
	return nil
}

// 13. 更新订单状态（102→103→104）
func (uc *ExpressUsecase) UpdateOrderStatus(ctx context.Context, orderID string, targetStatus int32, remark string) (*ExpressOrder, error) {
	// 1. 查询订单
	order, err := uc.repo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, ErrExpressNotFound
	}

	// 2. 校验状态流转
	if err := validateStatusTransition(order.Status, targetStatus); err != nil {
		return nil, err
	}

	// 3. 更新状态
	now := time.Now().Format(time.RFC3339)
	updated, err := uc.repo.UpdateStatus(ctx, orderID, targetStatus)
	if err != nil {
		uc.log.Errorf("UpdateStatus failed: %v", err)
		return nil, err
	}

	// 4. 追加轨迹
	traceItem := &TraceItem{
		Status: targetStatus,
		Time:   now,
		Desc:   fmt.Sprintf("%s%s", StatusText(targetStatus), remark),
	}
	uc.repo.AppendTrace(ctx, orderID, traceItem)

	uc.log.Infof("Order status updated: orderId=%s, status=%d→%d", orderID, order.Status, targetStatus)
	return updated, nil
}

// 14. 驿站投递
func (uc *ExpressUsecase) DeliverToStation(ctx context.Context, orderID, stationID string) (*ExpressOrder, error) {
	// 1. 查询订单
	order, err := uc.repo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, ErrExpressNotFound
	}

	// 2. 校验状态（必须是 103 运输中）
	if order.Status != StatusInTransit {
		return nil, errors.New(400, "ORDER_STATUS_INVALID", "订单状态异常，无法投递")
	}

	// 3. 校验驿站
	station, err := uc.stationRepo.FindByID(ctx, stationID)
	if err != nil || station == nil {
		return nil, errors.New(404, "STATION_NOT_FOUND", "驿站不存在")
	}
	if station.Status != StationStatusActive {
		return nil, errors.New(400, "STATION_CLOSED", "驿站已停业")
	}

	// 4. 生成取件码
	pickupCode := GeneratePickupCode()
	now := time.Now().Format(time.RFC3339)

	// 5. 更新订单（原子操作）
	updated, err := uc.repo.UpdateDeliveryInfo(ctx, orderID, StatusAtStation, stationID, "", pickupCode)
	if err != nil {
		uc.log.Errorf("UpdateDeliveryInfo failed: %v", err)
		return nil, err
	}

	// 6. 追加轨迹
	traceItem := &TraceItem{
		Status: StatusAtStation,
		Time:   now,
		Desc:   fmt.Sprintf("已到达驿站：%s，取件码：%s", station.Name, pickupCode),
	}
	uc.repo.AppendTrace(ctx, orderID, traceItem)

	uc.log.Infof("Order delivered to station: orderId=%s, stationId=%s, pickupCode=%s", orderID, stationID, pickupCode)
	return updated, nil
}

// 15. 快递柜投递
func (uc *ExpressUsecase) DeliverToCabinet(ctx context.Context, orderID, cabinetID, gridID, gridSize string) (*ExpressOrder, *CabinetGrid, error) {
	// 1. 查询订单
	order, err := uc.repo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, nil, ErrExpressNotFound
	}

	// 2. 校验状态（必须是 103 运输中）
	if order.Status != StatusInTransit {
		return nil, nil, errors.New(400, "ORDER_STATUS_INVALID", "订单状态异常，无法投递")
	}

	// 3. 校验快递柜
	cabinet, err := uc.cabinetRepo.FindByID(ctx, cabinetID)
	if err != nil || cabinet == nil {
		return nil, nil, errors.New(404, "CABINET_NOT_FOUND", "快递柜不存在")
	}
	if cabinet.Status != CabinetStatusOnline {
		return nil, nil, errors.New(400, "CABINET_OFFLINE", "快递柜已离线")
	}

	// 4. 锁定柜格
	var grid *CabinetGrid
	if gridID != "" {
		// 指定柜格
		grid, err = uc.gridRepo.LockGrid(ctx, gridID, orderID)
		if err != nil {
			return nil, nil, errors.New(409, "GRID_NOT_AVAILABLE", "柜格不可用")
		}
	} else {
		// 自动分配柜格
		availableGrids, err := uc.gridRepo.FindAvailable(ctx, cabinetID)
		if err != nil || len(availableGrids) == 0 {
			return nil, nil, errors.New(409, "NO_AVAILABLE_GRID", "无空闲柜格")
		}
		// 根据大小选择（如果指定了大小）
		for _, g := range availableGrids {
			if gridSize == "" || g.Size == gridSize {
				grid, err = uc.gridRepo.LockGrid(ctx, g.ID, orderID)
				if err == nil {
					break
				}
			}
		}
		if grid == nil {
			return nil, nil, errors.New(409, "NO_AVAILABLE_GRID", "无符合条件的空闲柜格")
		}
	}

	// 5. 生成取件码
	pickupCode := GeneratePickupCode()
	now := time.Now().Format(time.RFC3339)

	// 6. 更新订单
	updated, err := uc.repo.UpdateDeliveryInfo(ctx, orderID, StatusAtStation, "", grid.ID, pickupCode)
	if err != nil {
		// 回滚：释放柜格
		uc.gridRepo.ReleaseGrid(ctx, grid.ID)
		uc.log.Errorf("UpdateDeliveryInfo failed: %v", err)
		return nil, nil, err
	}
	updated.DeliveryType = 2 // 快递柜模式
	updated.CabinetID = cabinetID

	// 7. 追加轨迹
	traceItem := &TraceItem{
		Status: StatusAtStation,
		Time:   now,
		Desc:   fmt.Sprintf("已投递至快递柜：%s %s格，取件码：%s", cabinet.Name, grid.GridNo, pickupCode),
	}
	uc.repo.AppendTrace(ctx, orderID, traceItem)

	uc.log.Infof("Order delivered to cabinet: orderId=%s, cabinetId=%s, gridId=%s, pickupCode=%s", orderID, cabinetID, grid.ID, pickupCode)
	return updated, grid, nil
}

// 16. 驿站取件
func (uc *ExpressUsecase) PickupFromStation(ctx context.Context, pickupCode string) (*ExpressOrder, error) {
	// 1. 根据取件码查询订单
	order, err := uc.repo.FindByPickupCode(ctx, pickupCode)
	if err != nil || order == nil {
		return nil, ErrInvalidPickupCode
	}

	// 2. 校验状态
	if order.Status != StatusAtStation {
		return nil, ErrExpressAlreadyPicked
	}

	// 3. 校验投递类型（必须是驿站模式）
	if order.DeliveryType != 1 {
		return nil, errors.New(400, "INVALID_DELIVERY_TYPE", "该订单不在驿站")
	}

	// 4. 更新状态为已签收
	now := time.Now().Format(time.RFC3339)
	order.Status = StatusDelivered
	order.PickupCode = ""
	order.UpdateTime = now

	updated, err := uc.repo.Update(ctx, order)
	if err != nil {
		uc.log.Errorf("Update order failed: %v", err)
		return nil, err
	}

	// 5. 追加轨迹
	traceItem := &TraceItem{
		Status: StatusDelivered,
		Time:   now,
		Desc:   "已签收（驿站取件）",
	}
	uc.repo.AppendTrace(ctx, order.ID, traceItem)

	uc.log.Infof("Order picked up from station: orderId=%s", order.ID)
	return updated, nil
}

// 17. 快递柜取件
func (uc *ExpressUsecase) PickupFromCabinet(ctx context.Context, pickupCode string) (*ExpressOrder, *CabinetGrid, error) {
	// 1. 根据取件码查询订单
	order, err := uc.repo.FindByPickupCode(ctx, pickupCode)
	if err != nil || order == nil {
		return nil, nil, ErrInvalidPickupCode
	}

	// 2. 校验状态
	if order.Status != StatusAtStation {
		return nil, nil, ErrExpressAlreadyPicked
	}

	// 3. 校验投递类型（必须是快递柜模式）
	if order.DeliveryType != 2 {
		return nil, nil, errors.New(400, "INVALID_DELIVERY_TYPE", "该订单不在快递柜")
	}

	// 4. 释放柜格
	grid, err := uc.gridRepo.ReleaseGrid(ctx, order.GridID)
	if err != nil {
		uc.log.Warnf("Release grid failed: %v", err)
	}

	// 5. 更新状态为已签收
	now := time.Now().Format(time.RFC3339)
	order.Status = StatusDelivered
	order.PickupCode = ""
	order.GridID = ""
	order.UpdateTime = now

	updated, err := uc.repo.Update(ctx, order)
	if err != nil {
		uc.log.Errorf("Update order failed: %v", err)
		return nil, nil, err
	}

	// 6. 追加轨迹
	traceItem := &TraceItem{
		Status: StatusDelivered,
		Time:   now,
		Desc:   fmt.Sprintf("已签收（快递柜取件，格口%s已释放）", grid.GridNo),
	}
	uc.repo.AppendTrace(ctx, order.ID, traceItem)

	uc.log.Infof("Order picked up from cabinet: orderId=%s, gridId=%s", order.ID, order.GridID)
	return updated, grid, nil
}

// 18. 查询物流轨迹
func (uc *ExpressUsecase) GetTrace(ctx context.Context, orderID string) ([]TraceItem, error) {
	order, err := uc.repo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, ErrExpressNotFound
	}
	return order.Trace, nil
}

// 19. 追加轨迹项（内部方法）
func (uc *ExpressUsecase) AppendTrace(ctx context.Context, orderID string, status int32, desc string) error {
	now := time.Now().Format(time.RFC3339)
	traceItem := &TraceItem{
		Status: status,
		Time:   now,
		Desc:   desc,
	}
	_, err := uc.repo.AppendTrace(ctx, orderID, traceItem)
	return err
}

// 20. 查询驿站（辅助方法）
func (uc *ExpressUsecase) GetStation(ctx context.Context, stationID string) (*Station, error) {
	return uc.stationRepo.FindByID(ctx, stationID)
}

// 21. 查询快递柜（辅助方法）
func (uc *ExpressUsecase) GetCabinet(ctx context.Context, cabinetID string) (*Cabinet, error) {
	return uc.cabinetRepo.FindByID(ctx, cabinetID)
}

// 22. 查询柜格（辅助方法）
func (uc *ExpressUsecase) GetGrid(ctx context.Context, gridID string) (*CabinetGrid, error) {
	return uc.gridRepo.FindByID(ctx, gridID)
}
