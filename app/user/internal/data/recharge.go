package data

import (
	"context"

	"shared-device-saas/app/user/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type rechargeRepo struct {
	data *Data
	log  *log.Helper
}

// NewRechargeRepo 创建 RechargeRepo
func NewRechargeRepo(data *Data, logger log.Logger) biz.RechargeRepo {
	return &rechargeRepo{data: data, log: log.NewHelper(logger)}
}

func (r *rechargeRepo) Create(ctx context.Context, order *biz.RechargeOrder) (*biz.RechargeOrder, error) {
	// TODO: 接入 MySQL INSERT
	return order, nil
}

func (r *rechargeRepo) GetByOrderNo(ctx context.Context, orderNo string) (*biz.RechargeOrder, error) {
	// TODO: 接入 MySQL SELECT
	return &biz.RechargeOrder{}, nil
}

func (r *rechargeRepo) UpdateStatus(ctx context.Context, id int64, status int32, channelOrderNo string) error {
	// TODO: 接入 MySQL UPDATE
	return nil
}

func (r *rechargeRepo) List(ctx context.Context, tenantID, userID int64, status int32, createdAfter, createdBefore string, limit int, cursor string) ([]*biz.RechargeOrder, string, bool, error) {
	// TODO: 接入 MySQL 查询
	return []*biz.RechargeOrder{}, "", false, nil
}
