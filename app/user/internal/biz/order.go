package biz

import (
	"context"
	"fmt"
	"time"

	"shared-device-saas/pkg/payment"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// 订单错误定义
var (
	ErrOrderNotFound     = errors.NotFound("ORDER_NOT_FOUND", "订单不存在")
	ErrOrderCannotCancel = errors.BadRequest("ORDER_CANNOT_CANCEL", "订单当前状态不允许取消")
	ErrInvalidPayChannel = errors.BadRequest("INVALID_PAY_CHANNEL", "不支持的支付渠道")
)

// 订单状态常量
const (
	OrderStatusPending   int32 = 1 // 待支付
	OrderStatusPaid      int32 = 2 // 已支付
	OrderStatusCompleted int32 = 3 // 已完成
	OrderStatusCancelled int32 = 4 // 已取消
	OrderStatusRefunded  int32 = 5 // 已退款
)

// Order 订单领域实体
type Order struct {
	ID            string
	TenantID      int64 // 租户
	UserID        string // 用户
	OrderNo       string
	Source        int32 // 1=门票 2=共享单车 3=充电宝 4=智能快递柜
	OrderType     string
	Status        int32 // 1=待支付 2=已支付 3=已完成 4=已取消 5=已退款
	TotalAmount   int32 // 金额（分）
	Currency      string
	PaymentMethod int32 // 0=未支付 1=微信 2=支付宝 3=钱包余额
	Title         string
	Description   string
	ExtraJSON     string
	PaidAt        int64
	CreatedAt     int64
	UpdatedAt     int64
}

// OrderFilter 订单筛选条件
type OrderFilter struct {
	CreatedAfter  string
	CreatedBefore string
	Source        int32
	Status        int32
	MinAmount     int32
	MaxAmount     int32
	PaymentMethod int32
}

// OrderSort 排序
type OrderSort struct {
	Field     string // created_at / total_amount
	Direction string // asc / desc
}

// OrderListResult 订单列表结果
type OrderListResult struct {
	Orders     []*Order
	NextCursor string
	HasMore    bool
	TotalCount int32
}

// OrderRepo 订单仓储接口
type OrderRepo interface {
	CreateOrder(ctx context.Context, o *Order) error
	GetByOrderNo(ctx context.Context, tenantID int64, orderNo string) (*Order, error)
	ListOrders(ctx context.Context, tenantID int64, userID string, filter *OrderFilter, sort *OrderSort, limit int, cursor string) (*OrderListResult, error)
	UpdateStatus(ctx context.Context, tenantID int64, orderNo string, status int32, updates map[string]interface{}) error
}

// OrderUsecase 订单业务逻辑
type OrderUsecase struct {
	repo            OrderRepo
	inventoryUC     *InventoryUsecase
	paymentChannels map[int32]payment.PaymentChannel
	log             *log.Helper
}

// NewOrderUsecase 创建 OrderUsecase
func NewOrderUsecase(repo OrderRepo, inventoryUC *InventoryUsecase, paymentChannels map[int32]payment.PaymentChannel, logger log.Logger) *OrderUsecase {
	return &OrderUsecase{repo: repo, inventoryUC: inventoryUC, paymentChannels: paymentChannels, log: log.NewHelper(logger)}
}

// CreateOrder 创建订单（自动根据 source 决定是否走库存校验）
// source=3(充电宝): 需传 stationID，走 RentPowerBank 事务
// source=4(快递柜): 需传 cabinetID + cellType 或 cellID，走 AllocateCell 事务
// 其他 source: 直接创建订单，不走库存校验
func (uc *OrderUsecase) CreateOrder(ctx context.Context, tenantID int64, userID string, source int32, orderType string, totalAmount int32, currency string, title string, description string, extraJSON string) (*Order, error) {
	if totalAmount <= 0 {
		return nil, errors.BadRequest("INVALID_AMOUNT", "金额必须大于0")
	}
	if source <= 0 {
		return nil, errors.BadRequest("INVALID_SOURCE", "订单来源不能为空")
	}

	order := &Order{
		TenantID:      tenantID,
		UserID:        userID,
		OrderNo:       GenerateOrderNo(source),
		Source:        source,
		OrderType:     orderType,
		Status:        OrderStatusPending,
		TotalAmount:   totalAmount,
		Currency:      currency,
		PaymentMethod: 0,
		Title:         title,
		Description:   description,
		ExtraJSON:     extraJSON,
	}

	if order.Currency == "" {
		order.Currency = "CNY"
	}

	// 根据 source 类型走不同的库存校验路径
	switch source {
	case 3:
		// 充电宝：走库存校验事务（RentPowerBank 内部会创建订单+扣减库存）
		// 注意：此场景下应由 CreateOrderWithInventory 方法调用，此处仅创建订单不校验
		// 因为 stationID 等参数在此方法签名中不存在
		if err := uc.repo.CreateOrder(ctx, order); err != nil {
			return nil, err
		}
	case 4:
		// 快递柜：同上，库存校验由 CreateOrderWithInventory 负责
		if err := uc.repo.CreateOrder(ctx, order); err != nil {
			return nil, err
		}
	default:
		// 其他来源（门票等）：直接创建订单
		if err := uc.repo.CreateOrder(ctx, order); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// CreateOrderWithInventory 创建订单 + 库存校验（充电宝/快递柜专用）
// 此方法在同一个事务内完成：锁库存 → 创建订单 → 扣减库存
func (uc *OrderUsecase) CreateOrderWithInventory(ctx context.Context, order *Order, cabinetID int64, cellType int32, cellID int64, stationID int64) (*InventoryAllocation, error) {
	switch order.Source {
	case 4: // 快递柜
		if cellID > 0 {
			// 指定格口模式（扫码）
			return uc.inventoryUC.AllocateSpecificCell(ctx, order, cellID)
		}
		// 按类型自动分配
		return uc.inventoryUC.AllocateCellForOrder(ctx, order, cabinetID, cellType)

	case 3: // 充电宝
		return uc.inventoryUC.RentPowerBankForOrder(ctx, order, stationID)

	default:
		// 其他来源不需要库存校验，直接创建订单
		if err := uc.repo.CreateOrder(ctx, order); err != nil {
			return nil, err
		}
		return nil, nil
	}
}

// GetOrder 按租户+订单号查询订单
func (uc *OrderUsecase) GetOrder(ctx context.Context, tenantID int64, orderNo string) (*Order, error) {
	if orderNo == "" {
		return nil, errors.BadRequest("INVALID_ORDER_NO", "订单号不能为空")
	}
	return uc.repo.GetByOrderNo(ctx, tenantID, orderNo)
}

// ListOrders 多维筛选 + 游标分页查询订单
func (uc *OrderUsecase) ListOrders(ctx context.Context, tenantID int64, userID string, filter *OrderFilter, sort *OrderSort, limit int, cursor string) (*OrderListResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if sort == nil {
		sort = &OrderSort{Field: "created_at", Direction: "desc"}
	}
	if sort.Field == "" {
		sort.Field = "created_at"
	}
	if sort.Direction == "" {
		sort.Direction = "desc"
	}

	return uc.repo.ListOrders(ctx, tenantID, userID, filter, sort, limit, cursor)
}

// CancelOrder 取消订单（仅待支付可取消，同时释放库存）
func (uc *OrderUsecase) CancelOrder(ctx context.Context, tenantID int64, orderNo string) (*Order, error) {
	order, err := uc.repo.GetByOrderNo(ctx, tenantID, orderNo)
	if err != nil {
		return nil, err
	}
	if order.Status != OrderStatusPending {
		return nil, ErrOrderCannotCancel
	}

	if err := uc.repo.UpdateStatus(ctx, tenantID, orderNo, OrderStatusCancelled, nil); err != nil {
		return nil, err
	}
	order.Status = OrderStatusCancelled

	// 释放库存（充电宝/快递柜订单取消后释放资源）
	if order.Source == 3 || order.Source == 4 {
		// TODO: 从订单关联数据中获取 resourceID，目前先用 orderID 查询
		// 短期方案：由上层调用方主动调用 inventoryUC.ReleaseInventory
		uc.log.Infof("Order cancelled, inventory should be released: orderNo=%s source=%d", orderNo, order.Source)
	}

	return order, nil
}

// PayOrder 发起支付
func (uc *OrderUsecase) PayOrder(ctx context.Context, tenantID int64, orderNo string, paymentMethod int32) (string, error) {
	order, err := uc.repo.GetByOrderNo(ctx, tenantID, orderNo)
	if err != nil {
		return "", err
	}
	if order.Status != OrderStatusPending {
		return "", errors.BadRequest("ORDER_NOT_PENDING", "订单当前状态不允许支付")
	}

	ch, ok := uc.paymentChannels[paymentMethod]
	if !ok {
		return "", ErrInvalidPayChannel
	}

	payParams, err := ch.CreateOrder(order.OrderNo, int64(order.TotalAmount), order.Title)
	if err != nil {
		uc.log.Errorf("PayOrder CreateOrder error: channel=%d err=%v", paymentMethod, err)
		return "", fmt.Errorf("create payment: %w", err)
	}

	// 更新订单支付方式
	_ = uc.repo.UpdateStatus(ctx, tenantID, orderNo, order.Status, map[string]interface{}{
		"payment_method": paymentMethod,
	})

	return payParams, nil
}

// HandlePaymentCallback 处理支付回调
func (uc *OrderUsecase) HandlePaymentCallback(ctx context.Context, channel int32, payload, signature string) error {
	ch, ok := uc.paymentChannels[channel]
	if !ok {
		return ErrInvalidPayChannel
	}

	orderNo, paid, err := ch.VerifyCallback(payload, signature)
	if err != nil {
		uc.log.Errorf("HandlePaymentCallback verify error: channel=%d err=%v", channel, err)
		return fmt.Errorf("verify callback: %w", err)
	}
	if !paid {
		return errors.BadRequest("PAYMENT_NOT_PAID", "支付未完成")
	}

	// 更新订单状态为已支付（tenantID=0 表示跨租户回调，按 orderNo 匹配）
	now := time.Now().Unix()
	if err := uc.repo.UpdateStatus(ctx, 0, orderNo, OrderStatusPaid, map[string]interface{}{
		"paid_at": now,
	}); err != nil {
		uc.log.Errorf("HandlePaymentCallback UpdateStatus error: orderNo=%s err=%v", orderNo, err)
		return err
	}

	uc.log.Infof("HandlePaymentCallback: order %s paid successfully", orderNo)

	// TODO: 通知 storage 开柜（后续迭代）
	// go uc.notifyStorageOpenLocker(ctx, orderNo)

	return nil
}

// GenerateOrderNo 生成订单号（导出，供 service 层使用）
func GenerateOrderNo(source int32) string {
	now := time.Now()
	return fmt.Sprintf("ORD%d%04d%02d%02d%02d%02d%02d%06d",
		source,
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second(),
		now.UnixMilli()%1000000,
	)
}
