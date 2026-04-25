package biz

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
)

// Wallet 钱包领域实体
type Wallet struct {
	ID           int64
	TenantID     int64
	UserID       int64
	Balance      int64 // 可用余额（分）
	FrozenAmount int64 // 冻结金额（分）
	Version      int32
}

// Transaction 流水记录
type Transaction struct {
	ID           int64
	TenantID     int64
	WalletID     int64
	UserID       int64
	Type         int32 // 1=充值 2=消费 3=退款 4=冻结 5=解冻
	Amount       int64 // 正=入账 负=扣款
	BalanceAfter int64
	OrderNo      string
	Description  string
	CreatedAt    int64
}

// TransactionFilter 流水筛选
type TransactionFilter struct {
	Type          int32
	CreatedAfter  string
	CreatedBefore string
}

// TransactionListResult 流水列表结果
type TransactionListResult struct {
	Transactions []*Transaction
	NextCursor   string
	HasMore      bool
}

// WalletRepo 钱包仓储接口
type WalletRepo interface {
	GetByUserID(ctx context.Context, tenantID, userID int64) (*Wallet, error)
	Consume(ctx context.Context, walletID int64, amount int64, version int32, orderNo, desc string) (*Wallet, *Transaction, error)
	Freeze(ctx context.Context, walletID int64, amount int64, version int32, orderNo, desc string) (*Wallet, *Transaction, error)
	Unfreeze(ctx context.Context, walletID int64, amount int64, version int32, orderNo, desc string) (*Wallet, *Transaction, error)
	Refund(ctx context.Context, walletID int64, amount int64, version int32, orderNo, desc string) (*Wallet, *Transaction, error)
	Recharge(ctx context.Context, walletID int64, amount int64, version int32, orderNo, desc string) (*Wallet, *Transaction, error)
	ListTransactions(ctx context.Context, tenantID, userID int64, filter *TransactionFilter, limit int, cursor string) (*TransactionListResult, error)
}

// WalletUsecase 钱包业务逻辑
type WalletUsecase struct {
	repo WalletRepo
	log  *log.Helper
}

// NewWalletUsecase 创建 WalletUsecase
func NewWalletUsecase(repo WalletRepo, logger log.Logger) *WalletUsecase {
	return &WalletUsecase{repo: repo, log: log.NewHelper(logger)}
}

// GetWallet 查询钱包余额
func (uc *WalletUsecase) GetWallet(ctx context.Context, tenantID, userID int64) (*Wallet, error) {
	return uc.repo.GetByUserID(ctx, tenantID, userID)
}

// Consume 消费扣款
func (uc *WalletUsecase) Consume(ctx context.Context, tenantID, userID, amount int64, orderNo, desc string) (*Transaction, error) {
	wallet, err := uc.repo.GetByUserID(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("get wallet: %w", err)
	}
	if wallet.Balance < amount {
		return nil, fmt.Errorf("insufficient balance: have %d, need %d", wallet.Balance, amount)
	}
	_, tx, err := uc.repo.Consume(ctx, wallet.ID, amount, wallet.Version, orderNo, desc)
	return tx, err
}

// Freeze 冻结金额
func (uc *WalletUsecase) Freeze(ctx context.Context, tenantID, userID, amount int64, orderNo, desc string) (*Transaction, error) {
	wallet, err := uc.repo.GetByUserID(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("get wallet: %w", err)
	}
	if wallet.Balance < amount {
		return nil, fmt.Errorf("insufficient balance to freeze: have %d, need %d", wallet.Balance, amount)
	}
	_, tx, err := uc.repo.Freeze(ctx, wallet.ID, amount, wallet.Version, orderNo, desc)
	return tx, err
}

// Unfreeze 解冻金额
func (uc *WalletUsecase) Unfreeze(ctx context.Context, tenantID, userID, amount int64, orderNo, desc string) (*Transaction, error) {
	wallet, err := uc.repo.GetByUserID(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("get wallet: %w", err)
	}
	_, tx, err := uc.repo.Unfreeze(ctx, wallet.ID, amount, wallet.Version, orderNo, desc)
	return tx, err
}

// Refund 退款入账
func (uc *WalletUsecase) Refund(ctx context.Context, tenantID, userID, amount int64, orderNo, desc string) (*Transaction, error) {
	wallet, err := uc.repo.GetByUserID(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("get wallet: %w", err)
	}
	_, tx, err := uc.repo.Refund(ctx, wallet.ID, amount, wallet.Version, orderNo, desc)
	return tx, err
}

// Recharge 充值入账
func (uc *WalletUsecase) Recharge(ctx context.Context, tenantID, userID, amount int64, orderNo, desc string) (*Transaction, error) {
	wallet, err := uc.repo.GetByUserID(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("get wallet: %w", err)
	}
	_, tx, err := uc.repo.Recharge(ctx, wallet.ID, amount, wallet.Version, orderNo, desc)
	return tx, err
}

// ListTransactions 查询流水
func (uc *WalletUsecase) ListTransactions(ctx context.Context, tenantID, userID int64, filter *TransactionFilter, limit int, cursor string) (*TransactionListResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	return uc.repo.ListTransactions(ctx, tenantID, userID, filter, limit, cursor)
}
