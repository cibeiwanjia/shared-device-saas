package biz

import (
	"context"
	"math"

	"github.com/go-kratos/kratos/v2/log"
)

type PricingRepo interface {
	FindByTenantAndType(ctx context.Context, tenantID int64, ruleType string, cellType int32) (*PricingRule, error)
}

type PricingEngine struct {
	pricingRepo PricingRepo
	log         *log.Helper
}

func NewPricingEngine(pricingRepo PricingRepo) *PricingEngine {
	return &PricingEngine{pricingRepo: pricingRepo, log: log.NewHelper(log.DefaultLogger)}
}

func (e *PricingEngine) MatchRule(ctx context.Context, tenantID int64, ruleType string, cellType int32) (*PricingRule, error) {
	return e.pricingRepo.FindByTenantAndType(ctx, tenantID, ruleType, cellType)
}

func (e *PricingEngine) CalculateFee(ctx context.Context, rule *PricingRule, overtimeMinutes int) (int32, error) {
	if rule == nil || overtimeMinutes <= 0 {
		return 0, nil
	}

	overtimeHours := int(math.Ceil(float64(overtimeMinutes) / 60.0))
	if overtimeHours <= 0 {
		return 0, nil
	}

	fee := int32(overtimeHours) * rule.PricePerHour

	if rule.MaxFee > 0 && fee > rule.MaxFee {
		fee = rule.MaxFee
	}

	return fee, nil
}
