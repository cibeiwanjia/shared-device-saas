package data

import (
	"context"
	"database/sql"
	"fmt"

	"shared-device-saas/app/storage/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type pricingRepo struct {
	data *Data
	log  *log.Helper
}

func NewPricingRepo(data *Data, logger log.Logger) biz.PricingRepo {
	return &pricingRepo{data: data, log: log.NewHelper(logger)}
}

func (r *pricingRepo) FindByTenantAndType(ctx context.Context, tenantID int64, ruleType string, cellType int32) (*biz.PricingRule, error) {
	p := &biz.PricingRule{}
	var cellTypeVal sql.NullInt32
	var effectiveFrom, effectiveUntil sql.NullTime

	err := r.data.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, rule_type, free_hours, price_per_hour, price_per_day, max_fee, cell_type, priority, effective_from, effective_until, enabled, created_at, updated_at
		 FROM pricing_rules
		 WHERE tenant_id = ? AND rule_type = ? AND enabled = 1
		   AND (cell_type IS NULL OR cell_type = ?)
		   AND (effective_from IS NULL OR effective_from <= NOW())
		   AND (effective_until IS NULL OR effective_until >= NOW())
		 ORDER BY priority DESC LIMIT 1`,
		tenantID, ruleType, cellType,
	).Scan(&p.ID, &p.TenantID, &p.RuleType, &p.FreeHours, &p.PricePerHour, &p.PricePerDay, &p.MaxFee,
		&cellTypeVal, &p.Priority, &effectiveFrom, &effectiveUntil, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find pricing rule: %w", err)
	}
	if cellTypeVal.Valid {
		p.CellType = &cellTypeVal.Int32
	}
	if effectiveFrom.Valid {
		p.EffectiveFrom = &effectiveFrom.Time
	}
	if effectiveUntil.Valid {
		p.EffectiveUntil = &effectiveUntil.Time
	}
	return p, nil
}
