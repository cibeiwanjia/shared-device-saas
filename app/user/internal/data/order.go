package data

import (
	"context"
	"fmt"
	"strings"

	"shared-device-saas/app/user/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type orderRepo struct {
	data *Data
	log  *log.Helper
}

// NewOrderRepo 创建 OrderRepo
func NewOrderRepo(data *Data, logger log.Logger) biz.OrderRepo {
	return &orderRepo{data: data, log: log.NewHelper(logger)}
}

func (r *orderRepo) ListOrders(ctx context.Context, tenantID, userID int64, filter *biz.OrderFilter, sort *biz.OrderSort, limit int, cursor string) (*biz.OrderListResult, error) {
	// TODO: 待 Data 层接入 MySQL 后实现真实查询
	// 当前返回空结果桩实现

	// 构建查询条件（预留，接入 DB 后启用）
	_ = r.buildWhereClause(tenantID, userID, filter)
	_ = r.buildOrderBy(sort)

	return &biz.OrderListResult{
		Orders:     []*biz.Order{},
		NextCursor: "",
		HasMore:    false,
		TotalCount: 0,
	}, nil
}

// buildWhereClause 构建动态 WHERE 条件
func (r *orderRepo) buildWhereClause(tenantID, userID int64, filter *biz.OrderFilter) string {
	var conditions []string
	conditions = append(conditions, fmt.Sprintf("tenant_id = %d", tenantID))
	conditions = append(conditions, fmt.Sprintf("user_id = %d", userID))

	if filter != nil {
		if filter.CreatedAfter != "" {
			conditions = append(conditions, fmt.Sprintf("created_at >= '%s'", filter.CreatedAfter))
		}
		if filter.CreatedBefore != "" {
			conditions = append(conditions, fmt.Sprintf("created_at <= '%s'", filter.CreatedBefore))
		}
		if filter.Source > 0 {
			conditions = append(conditions, fmt.Sprintf("source = %d", filter.Source))
		}
		if filter.Status > 0 {
			conditions = append(conditions, fmt.Sprintf("status = %d", filter.Status))
		}
		if filter.MinAmount > 0 {
			conditions = append(conditions, fmt.Sprintf("total_amount >= %d", filter.MinAmount))
		}
		if filter.MaxAmount > 0 {
			conditions = append(conditions, fmt.Sprintf("total_amount <= %d", filter.MaxAmount))
		}
		if filter.PaymentMethod > 0 {
			conditions = append(conditions, fmt.Sprintf("payment_method = %d", filter.PaymentMethod))
		}
	}

	return "WHERE " + strings.Join(conditions, " AND ")
}

// buildOrderBy 构建排序
func (r *orderRepo) buildOrderBy(sort *biz.OrderSort) string {
	if sort == nil {
		return "ORDER BY created_at DESC, id DESC"
	}
	dir := "DESC"
	if strings.EqualFold(sort.Direction, "asc") {
		dir = "ASC"
	}
	field := "created_at"
	if sort.Field == "total_amount" {
		field = "total_amount"
	}
	return fmt.Sprintf("ORDER BY %s %s, id %s", field, dir, dir)
}
