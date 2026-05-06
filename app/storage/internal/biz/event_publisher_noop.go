package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

type NoOpEventPublisher struct {
	log *log.Helper
}

func NewNoOpEventPublisher(logger log.Logger) EventPublisher {
	return &NoOpEventPublisher{log: log.NewHelper(logger)}
}

func (p *NoOpEventPublisher) PublishPickupReady(_ context.Context, tenantID int64, orderNo string, userID int64, pickupCode string, cabinetName string) error {
	p.log.Infof("PublishPickupReady: tenant=%d order=%s user=%d code=%s cabinet=%s", tenantID, orderNo, userID, pickupCode, cabinetName)
	return nil
}

func (p *NoOpEventPublisher) PublishOrderTimeout(_ context.Context, tenantID int64, orderNo string, userID int64, fee int32) error {
	p.log.Infof("PublishOrderTimeout: tenant=%d order=%s user=%d fee=%d", tenantID, orderNo, userID, fee)
	return nil
}

func (p *NoOpEventPublisher) PublishOpenTimeout(_ context.Context, tenantID int64, deviceSN string, cellID int64, orderNo string) error {
	p.log.Infof("PublishOpenTimeout: tenant=%d device=%s cell=%d order=%s", tenantID, deviceSN, cellID, orderNo)
	return nil
}
