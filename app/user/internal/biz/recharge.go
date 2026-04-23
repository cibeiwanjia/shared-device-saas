package biz

import (
	"context"
	"fmt"

	"shared-device-saas/pkg/payment"

	"github.com/go-kratos/kratos/v2/log"
)

// RechargeOrder 充值订单
type RechargeOrder struct {
	ID             int64
	TenantID       int64
	UserID         int64
	OrderNo        string
	Amount         int64 // 金额（分）
	PaymentMethod  int32 // 1=微信 2=支付宝
	Status         int32 // 1=待支付 2=已支付 3=失败
	ChannelOrderNo string
	PaidAt         int64
	CreatedAt      int64
}

// RechargeRepo 充值仓储接口
type RechargeRepo interface {
	Create(ctx context.Context, order *RechargeOrder) (*RechargeOrder, error)
	GetByOrderNo(ctx context.Context, orderNo string) (*RechargeOrder, error)
	UpdateStatus(ctx context.Context, id int64, status int32, channelOrderNo string) error
	List(ctx context.Context, tenantID, userID int64, status int32, createdAfter, createdBefore string, limit int, cursor string) ([]*RechargeOrder, string, bool, error)
}

// RechargeUsecase 充值业务逻辑
type RechargeUsecase struct {
	repo     RechargeRepo
	wallet   WalletUsecase
	channels map[int32]payment.PaymentChannel
	log      *log.Helper
}

// NewRechargeUsecase 创建 RechargeUsecase
func NewRechargeUsecase(repo RechargeRepo, wallet WalletUsecase, channels map[int32]payment.PaymentChannel, logger log.Logger) *RechargeUsecase {
	return &RechargeUsecase{repo: repo, wallet: wallet, channels: channels, log: log.NewHelper(logger)}
}

// CreateRecharge 创建充值订单
func (uc *RechargeUsecase) CreateRecharge(ctx context.Context, tenantID, userID int64, amount int64, paymentMethod int32) (*RechargeOrder, string, error) {
	// 金额校验
	if amount < 100 { // 最低 1 元
		return nil, "", fmt.Errorf("minimum recharge amount is 1.00 yuan")
	}
	if amount > 500000 { // 最高 5000 元
		return nil, "", fmt.Errorf("maximum recharge amount is 5000.00 yuan")
	}

	// 生成订单号
	orderNo := fmt.Sprintf("RC%d%d", tenantID, userID) // TODO: 用 snowflake

	order := &RechargeOrder{
		TenantID:      tenantID,
		UserID:        userID,
		OrderNo:       orderNo,
		Amount:        amount,
		PaymentMethod: paymentMethod,
		Status:        1, // pending
	}

	saved, err := uc.repo.Create(ctx, order)
	if err != nil {
		return nil, "", fmt.Errorf("create recharge order: %w", err)
	}

	// 调用支付渠道
	ch, ok := uc.channels[paymentMethod]
	if !ok {
		return nil, "", fmt.Errorf("unsupported payment method: %d", paymentMethod)
	}

	payParams, err := ch.CreateOrder(orderNo, amount, "钱包充值")
	if err != nil {
		return nil, "", fmt.Errorf("create payment order: %w", err)
	}

	return saved, payParams, nil
}

// HandleCallback 处理支付回调
func (uc *RechargeUsecase) HandleCallback(ctx context.Context, paymentMethod int32, payload, signature string) error {
	ch, ok := uc.channels[paymentMethod]
	if !ok {
		return fmt.Errorf("unsupported payment channel: %d", paymentMethod)
	}

	orderNo, paid, err := ch.VerifyCallback(payload, signature)
	if err != nil {
		return fmt.Errorf("verify callback: %w", err)
	}

	if !paid {
		return nil
	}

	// 查询充值订单
	order, err := uc.repo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return fmt.Errorf("get recharge order: %w", err)
	}

	// 幂等检查
	if order.Status == 2 { // already paid
		return nil
	}

	// 更新状态
	if err := uc.repo.UpdateStatus(ctx, order.ID, 2, orderNo); err != nil {
		return fmt.Errorf("update recharge status: %w", err)
	}

	// 入账钱包
	_, err = uc.wallet.Recharge(ctx, order.TenantID, order.UserID, order.Amount, orderNo, "钱包充值")
	if err != nil {
		return fmt.Errorf("recharge to wallet: %w", err)
	}

	return nil
}

// ListRecharges 查询充值记录
func (uc *RechargeUsecase) ListRecharges(ctx context.Context, tenantID, userID int64, status int32, createdAfter, createdBefore string, limit int, cursor string) ([]*RechargeOrder, string, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	return uc.repo.List(ctx, tenantID, userID, status, createdAfter, createdBefore, limit, cursor)
}
