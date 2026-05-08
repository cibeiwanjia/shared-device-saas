package biz

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// ============================================
// 超时处理定时任务（简化版：仅告警，不重新派单）
// ============================================

// TimeoutHandler 超时处理处理器
type TimeoutHandler struct {
	orderRepo   ExpressRepo // 订单仓库
	courierRepo CourierRepo // 快递员仓库
	log         *log.Helper // 日志助手
}

// NewTimeoutHandler 创建超时处理器
func NewTimeoutHandler(orderRepo ExpressRepo, courierRepo CourierRepo, logger log.Logger) *TimeoutHandler {
	return &TimeoutHandler{
		orderRepo:   orderRepo,
		courierRepo: courierRepo,
		log:         log.NewHelper(logger),
	}
}

// ScanTimeoutOrders 扫描超时订单
// 每分钟执行一次，扫描条件：status=101 AND pickupTimeEnd < now() AND timeoutNotified=false
func (h *TimeoutHandler) ScanTimeoutOrders(ctx context.Context) ([]*ExpressOrder, error) {
	now := time.Now().Format(time.RFC3339)                 // 获取当前时间
	orders, err := h.orderRepo.FindTimeoutOrders(ctx, now) // 查询超时订单
	if err != nil {
		h.log.Errorf("FindTimeoutOrders failed: %v", err)
		return nil, err
	}
	// 无超时订单，直接返回
	if len(orders) == 0 {
		h.log.Debug("No timeout orders found")
		return nil, nil
	}

	h.log.Infof("Found %d timeout orders", len(orders))

	// 处理每个超时订单
	for _, order := range orders {
		h.handleTimeoutOrder(ctx, order)
	}

	return orders, nil
}

// handleTimeoutOrder 处理单个超时订单
// 简化逻辑：标记超时 + 发送告警给原快递员（不释放负载，不重新派单）
func (h *TimeoutHandler) handleTimeoutOrder(ctx context.Context, order *ExpressOrder) {
	// 1. 标记订单状态为超时未取件
	order.Status = StatusTimeoutPickup
	order.TimeoutNotified = true
	// 注意：ShortCode 不清空，原快递员仍可核验完成揽收
	order.UpdateTime = time.Now().Format(time.RFC3339)

	_, err := h.orderRepo.Update(ctx, order)
	if err != nil {
		h.log.Errorf("Update timeout order failed: orderId=%s, err=%v", order.ID, err)
		return
	}

	// 2. 发送告警给原快递员（要求优先处理）
	if order.CourierID != "" {
		h.sendAlertToCourier(ctx, order.CourierID, order.ID)
	}

	// 3. TODO: 发送通知给用户（短信/推送）
	// 通知内容："您的寄件订单超时未取件，快递员正在优先处理"
	h.log.Infof("Timeout order notified: orderId=%s, userId=%s, courierId=%s", order.ID, order.UserID, order.CourierID)
}

// sendAlertToCourier 发送告警给快递员
// 当前实现：日志记录 + TODO标记（未来可接入WebSocket/短信）
func (h *TimeoutHandler) sendAlertToCourier(ctx context.Context, courierID, orderID string) {
	// TODO: 接入 WebSocket 推送或短信通知
	// 消息内容："您有订单超时未取件，请优先处理。订单号：xxx"

	h.log.Warnf("ALERT: Courier timeout order - courierId=%s, orderId=%s, message=您有订单超时未取件，请优先处理", courierID, orderID)

	// 可选：查询快递员信息，用于后续通知
	courier, err := h.courierRepo.FindByID(ctx, courierID)
	if err == nil && courier != nil {
		h.log.Infof("Courier info for alert: name=%s, phone=%s", courier.RealName, courier.Phone)
	}
}

// Run 定时任务运行循环
func (h *TimeoutHandler) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	h.log.Infof("Timeout handler started, interval=%v", interval)

	for {
		select {
		case <-ctx.Done():
			h.log.Info("Timeout handler stopped")
			return
		case <-ticker.C:
			h.ScanTimeoutOrders(ctx)
		}
	}
}
