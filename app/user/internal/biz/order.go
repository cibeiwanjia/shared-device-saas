package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

// Order 订单领域实体
type Order struct {
	ID            int64
	TenantID      int64
	UserID        int64
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
	ListOrders(ctx context.Context, tenantID, userID int64, filter *OrderFilter, sort *OrderSort, limit int, cursor string) (*OrderListResult, error)
}

// OrderUsecase 订单业务逻辑
type OrderUsecase struct {
	repo OrderRepo
	log  *log.Helper
}

// NewOrderUsecase 创建 OrderUsecase
func NewOrderUsecase(repo OrderRepo, logger log.Logger) *OrderUsecase {
	return &OrderUsecase{repo: repo, log: log.NewHelper(logger)}
}

// ListOrders 多维筛选 + 游标分页查询订单
func (uc *OrderUsecase) ListOrders(ctx context.Context, tenantID, userID int64, filter *OrderFilter, sort *OrderSort, limit int, cursor string) (*OrderListResult, error) {
	// 参数校验
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
