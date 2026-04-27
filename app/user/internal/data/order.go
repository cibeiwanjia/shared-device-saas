package data

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"shared-device-saas/app/user/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// orderRow MySQL 订单表行结构
type orderRow struct {
	ID            int64   `db:"id"`
	TenantID      int64   `db:"tenant_id"`
	UserID        string  `db:"user_id"`
	OrderNo       string  `db:"order_no"`
	Source        int32   `db:"source"`
	OrderType     string  `db:"order_type"`
	Status        int32   `db:"status"`
	TotalAmount   int32   `db:"total_amount"`
	Currency      string  `db:"currency"`
	PaymentMethod int32   `db:"payment_method"`
	Title         string  `db:"title"`
	Description   string  `db:"description"`
	ExtraJSON     string  `db:"extra_json"`
	PaidAt        sql.NullInt64 `db:"paid_at"`
	CreatedAt     int64   `db:"created_at"`
	UpdatedAt     int64   `db:"updated_at"`
}

type orderRepo struct {
	data *Data
	log  *log.Helper
}

// NewOrderRepo 创建 OrderRepo（MySQL 实现）
func NewOrderRepo(data *Data, logger log.Logger) biz.OrderRepo {
	return &orderRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// CreateOrder 创建订单
func (r *orderRepo) CreateOrder(ctx context.Context, o *biz.Order) error {
	now := time.Now().Unix()
	o.CreatedAt = now
	o.UpdatedAt = now

	db := r.data.GetSqlDB()
	if db == nil {
		return fmt.Errorf("mysql not connected")
	}

	result, err := db.ExecContext(ctx,
		`INSERT INTO orders (tenant_id, user_id, order_no, source, order_type, status, total_amount, currency, payment_method, title, description, extra_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.TenantID, o.UserID, o.OrderNo, o.Source, o.OrderType, o.Status,
		o.TotalAmount, o.Currency, o.PaymentMethod, o.Title, o.Description, o.ExtraJSON,
		o.CreatedAt, o.UpdatedAt,
	)
	if err != nil {
		r.log.Errorf("CreateOrder Insert error: %v", err)
		return fmt.Errorf("create order: %w", err)
	}

	id, _ := result.LastInsertId()
	o.ID = strconv.FormatInt(id, 10)
	return nil
}

// GetByOrderNo 按租户+订单号查询订单
func (r *orderRepo) GetByOrderNo(ctx context.Context, tenantID int64, orderNo string) (*biz.Order, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, fmt.Errorf("mysql not connected")
	}

	var row orderRow
	query := `SELECT id, tenant_id, user_id, order_no, source, order_type, status, total_amount, currency, payment_method, title, description, extra_json, paid_at, created_at, updated_at
	          FROM orders WHERE order_no = ?`
	args := []interface{}{orderNo}

	// tenantID > 0 时增加租户过滤
	if tenantID > 0 {
		query += ` AND tenant_id = ?`
		args = append(args, tenantID)
	}

	err := db.QueryRowContext(ctx, query, args...).Scan(
		&row.ID, &row.TenantID, &row.UserID, &row.OrderNo, &row.Source, &row.OrderType,
		&row.Status, &row.TotalAmount, &row.Currency, &row.PaymentMethod,
		&row.Title, &row.Description, &row.ExtraJSON, &row.PaidAt,
		&row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, biz.ErrOrderNotFound
		}
		r.log.Errorf("GetByOrderNo QueryRow error: %v", err)
		return nil, fmt.Errorf("get order: %w", err)
	}

	return rowToOrder(&row), nil
}

// ListOrders 多维筛选 + 游标分页查询订单
func (r *orderRepo) ListOrders(ctx context.Context, tenantID int64, userID string, filter *biz.OrderFilter, sort *biz.OrderSort, limit int, cursor string) (*biz.OrderListResult, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, fmt.Errorf("mysql not connected")
	}

	// 构建 WHERE 条件
	var conditions []string
	var args []interface{}

	conditions = append(conditions, "tenant_id = ?")
	args = append(args, tenantID)
	conditions = append(conditions, "user_id = ?")
	args = append(args, userID)

	if filter != nil {
		if filter.CreatedAfter != "" {
			v, err := parseInt64(filter.CreatedAfter)
			if err == nil {
				conditions = append(conditions, "created_at >= ?")
				args = append(args, v)
			}
		}
		if filter.CreatedBefore != "" {
			v, err := parseInt64(filter.CreatedBefore)
			if err == nil {
				conditions = append(conditions, "created_at <= ?")
				args = append(args, v)
			}
		}
		if filter.Source > 0 {
			conditions = append(conditions, "source = ?")
			args = append(args, filter.Source)
		}
		if filter.Status > 0 {
			conditions = append(conditions, "status = ?")
			args = append(args, filter.Status)
		}
		if filter.MinAmount > 0 {
			conditions = append(conditions, "total_amount >= ?")
			args = append(args, filter.MinAmount)
		}
		if filter.MaxAmount > 0 {
			conditions = append(conditions, "total_amount <= ?")
			args = append(args, filter.MaxAmount)
		}
		if filter.PaymentMethod > 0 {
			conditions = append(conditions, "payment_method = ?")
			args = append(args, filter.PaymentMethod)
		}
	}

	// 游标分页（基于 id）
	if cursor != "" {
		cursorID, err := strconv.ParseInt(cursor, 10, 64)
		if err == nil {
			conditions = append(conditions, "id < ?")
			args = append(args, cursorID)
		}
	}

	whereClause := strings.Join(conditions, " AND ")

	// 排序
	sortField := "created_at"
	sortDir := "DESC"
	if sort != nil {
		if sort.Field == "total_amount" {
			sortField = "total_amount"
		}
		if sort.Direction == "asc" {
			sortDir = "ASC"
		}
	}

	// 多取一条判断 has_more
	fetchLimit := limit + 1

	query := fmt.Sprintf(
		`SELECT id, tenant_id, user_id, order_no, source, order_type, status, total_amount, currency, payment_method, title, description, extra_json, paid_at, created_at, updated_at
		 FROM orders WHERE %s ORDER BY %s %s, id DESC LIMIT ?`,
		whereClause, sortField, sortDir,
	)
	args = append(args, fetchLimit)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		r.log.Errorf("ListOrders Query error: %v", err)
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var orderRows []orderRow
	for rows.Next() {
		var row orderRow
		if err := rows.Scan(
			&row.ID, &row.TenantID, &row.UserID, &row.OrderNo, &row.Source, &row.OrderType,
			&row.Status, &row.TotalAmount, &row.Currency, &row.PaymentMethod,
			&row.Title, &row.Description, &row.ExtraJSON, &row.PaidAt,
			&row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			r.log.Errorf("ListOrders Scan error: %v", err)
			return nil, fmt.Errorf("list orders scan: %w", err)
		}
		orderRows = append(orderRows, row)
	}

	hasMore := len(orderRows) > limit
	if hasMore {
		orderRows = orderRows[:limit]
	}

	orders := make([]*biz.Order, len(orderRows))
	for i, row := range orderRows {
		orders[i] = rowToOrder(&row)
	}

	var nextCursor string
	if hasMore && len(orders) > 0 {
		nextCursor = strconv.FormatInt(orderRows[len(orderRows)-1].ID, 10)
	}

	// total_count 仅当结果不足一页时精确返回
	totalCount := int32(len(orders))
	if !hasMore {
		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM orders WHERE %s`, whereClause)
		// 复用 args 但去掉最后的 limit 参数
		countArgs := args[:len(args)-1]
		var total int64
		if err := db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
			r.log.Warnf("ListOrders Count error: %v", err)
		} else {
			totalCount = int32(total)
		}
	}

	return &biz.OrderListResult{
		Orders:     orders,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		TotalCount: totalCount,
	}, nil
}

// UpdateStatus 更新订单状态
func (r *orderRepo) UpdateStatus(ctx context.Context, tenantID int64, orderNo string, status int32, updates map[string]interface{}) error {
	db := r.data.GetSqlDB()
	if db == nil {
		return fmt.Errorf("mysql not connected")
	}

	var setClauses []string
	var args []interface{}

	setClauses = append(setClauses, "status = ?")
	args = append(args, status)
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now().Unix())

	// 动态字段更新（如 paid_at, payment_method 等）
	for k, v := range updates {
		// 白名单：只允许更新指定字段，防 SQL 注入
		switch k {
		case "paid_at", "payment_method":
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", k))
			args = append(args, v)
		}
	}

	whereClause := "order_no = ?"
	args = append(args, orderNo)
	if tenantID > 0 {
		whereClause += " AND tenant_id = ?"
		args = append(args, tenantID)
	}

	query := fmt.Sprintf("UPDATE orders SET %s WHERE %s", strings.Join(setClauses, ", "), whereClause)
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		r.log.Errorf("UpdateStatus Exec error: %v", err)
		return fmt.Errorf("update order status: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return biz.ErrOrderNotFound
	}
	return nil
}

// rowToOrder 数据库行转领域实体
func rowToOrder(row *orderRow) *biz.Order {
	o := &biz.Order{
		ID:            strconv.FormatInt(row.ID, 10),
		TenantID:      row.TenantID,
		UserID:        row.UserID,
		OrderNo:       row.OrderNo,
		Source:        row.Source,
		OrderType:     row.OrderType,
		Status:        row.Status,
		TotalAmount:   row.TotalAmount,
		Currency:      row.Currency,
		PaymentMethod: row.PaymentMethod,
		Title:         row.Title,
		Description:   row.Description,
		ExtraJSON:     row.ExtraJSON,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}

	if row.PaidAt.Valid {
		o.PaidAt = row.PaidAt.Int64
	}
	return o
}

// parseInt64 解析字符串为 int64
func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
