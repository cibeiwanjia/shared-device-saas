package data

import (
	"context"

	"shared-device-saas/app/user/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type walletRepo struct {
	data *Data
	log  *log.Helper
}

// NewWalletRepo 创建 WalletRepo
func NewWalletRepo(data *Data, logger log.Logger) biz.WalletRepo {
	return &walletRepo{data: data, log: log.NewHelper(logger)}
}

func (r *walletRepo) GetByUserID(ctx context.Context, tenantID, userID int64) (*biz.Wallet, error) {
	// TODO: 接入 MySQL 查询
	return &biz.Wallet{ID: 1, TenantID: tenantID, UserID: userID, Balance: 0, FrozenAmount: 0, Version: 0}, nil
}

func (r *walletRepo) Consume(ctx context.Context, walletID int64, amount int64, version int32, orderNo, desc string) (*biz.Wallet, *biz.Transaction, error) {
	// TODO: 乐观锁扣款 UPDATE ... SET balance=balance-?, version=version+1 WHERE id=? AND version=? AND balance>=?
	return &biz.Wallet{}, &biz.Transaction{}, nil
}

func (r *walletRepo) Freeze(ctx context.Context, walletID int64, amount int64, version int32, orderNo, desc string) (*biz.Wallet, *biz.Transaction, error) {
	// TODO: UPDATE ... SET balance=balance-?, frozen_amount=frozen_amount+?, version=version+1 WHERE ...
	return &biz.Wallet{}, &biz.Transaction{}, nil
}

func (r *walletRepo) Unfreeze(ctx context.Context, walletID int64, amount int64, version int32, orderNo, desc string) (*biz.Wallet, *biz.Transaction, error) {
	// TODO: UPDATE ... SET balance=balance+?, frozen_amount=frozen_amount-?, version=version+1 WHERE ...
	return &biz.Wallet{}, &biz.Transaction{}, nil
}

func (r *walletRepo) Refund(ctx context.Context, walletID int64, amount int64, version int32, orderNo, desc string) (*biz.Wallet, *biz.Transaction, error) {
	// TODO: UPDATE ... SET balance=balance+?, version=version+1 WHERE ...
	return &biz.Wallet{}, &biz.Transaction{}, nil
}

func (r *walletRepo) Recharge(ctx context.Context, walletID int64, amount int64, version int32, orderNo, desc string) (*biz.Wallet, *biz.Transaction, error) {
	// TODO: UPDATE ... SET balance=balance+?, version=version+1 WHERE ...
	return &biz.Wallet{}, &biz.Transaction{}, nil
}

func (r *walletRepo) ListTransactions(ctx context.Context, tenantID, userID int64, filter *biz.TransactionFilter, limit int, cursor string) (*biz.TransactionListResult, error) {
	// TODO: 接入 MySQL 查询
	return &biz.TransactionListResult{Transactions: []*biz.Transaction{}, HasMore: false}, nil
}
