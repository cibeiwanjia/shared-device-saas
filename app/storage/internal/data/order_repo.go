package data

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"shared-device-saas/app/storage/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type orderRepo struct {
	data *Data
	log  *log.Helper
}

func NewOrderRepo(data *Data, logger log.Logger) biz.OrderRepo {
	return &orderRepo{data: data, log: log.NewHelper(logger)}
}

func (r *orderRepo) Create(ctx context.Context, order *biz.StorageOrder) (*biz.StorageOrder, error) {
	orderNo := generateOrderNo()
	result, err := r.data.db.ExecContext(ctx,
		`INSERT INTO storage_orders (tenant_id, order_no, order_type, status, user_id, operator_id, cabinet_id, cell_id, device_sn, slot_index)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		order.TenantID, orderNo, order.OrderType, order.Status, order.UserID, order.OperatorID,
		order.CabinetID, order.CellID, order.DeviceSN, order.SlotIndex,
	)
	if err != nil {
		return nil, fmt.Errorf("insert order: %w", err)
	}
	id, _ := result.LastInsertId()
	order.ID = id
	order.OrderNo = orderNo
	return order, nil
}

func (r *orderRepo) GetByID(ctx context.Context, id int64) (*biz.StorageOrder, error) {
	return r.scanOne(ctx, "WHERE id = ?", id)
}

func (r *orderRepo) GetByOrderNo(ctx context.Context, orderNo string) (*biz.StorageOrder, error) {
	return r.scanOne(ctx, "WHERE order_no = ?", orderNo)
}

func (r *orderRepo) UpdateStatusCAS(ctx context.Context, id int64, expectedFrom, newStatus int32) (int64, error) {
	result, err := r.data.db.ExecContext(ctx,
		"UPDATE storage_orders SET status = ? WHERE id = ? AND status = ?", newStatus, id, expectedFrom)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func (r *orderRepo) UpdateAmount(ctx context.Context, id int64, totalAmount int32) error {
	_, err := r.data.db.ExecContext(ctx,
		"UPDATE storage_orders SET total_amount = ? WHERE id = ?", totalAmount, id)
	return err
}

func (r *orderRepo) UpdatePickedUp(ctx context.Context, id int64, status int32) error {
	_, err := r.data.db.ExecContext(ctx,
		"UPDATE storage_orders SET status = ?, picked_up_at = ? WHERE id = ?", status, time.Now(), id)
	return err
}

func (r *orderRepo) FindPossiblyTimeoutOrders(ctx context.Context, threshold time.Duration) ([]*biz.StorageOrder, error) {
	cutoff := time.Now().Add(-threshold)
	rows, err := r.data.db.QueryContext(ctx,
		`SELECT id, tenant_id, order_no, order_type, status, user_id, operator_id, cabinet_id, cell_id, device_sn, slot_index,
		        pickup_code, deposited_at, picked_up_at, total_amount, paid_amount, overtime_minutes, remark, created_at, updated_at
		 FROM storage_orders WHERE status IN (?, ?) AND deposited_at IS NOT NULL AND deposited_at < ?`,
		biz.OrderStatusDeposited, biz.OrderStatusStoring, cutoff)
	if err != nil {
		return nil, fmt.Errorf("find timeout orders: %w", err)
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *orderRepo) ListByUser(ctx context.Context, tenantID, userID int64, orderType string, page, pageSize int32) ([]*biz.StorageOrder, int32, error) {
	where := "WHERE tenant_id = ? AND user_id = ?"
	args := []interface{}{tenantID, userID}
	if orderType != "" {
		where += " AND order_type = ?"
		args = append(args, orderType)
	}
	where += " AND status NOT IN (?, ?)"
	args = append(args, biz.OrderStatusCompleted, biz.OrderStatusCleared)

	var total int32
	r.data.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM storage_orders "+where, args...).Scan(&total)

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(`SELECT id, tenant_id, order_no, order_type, status, user_id, operator_id, cabinet_id, cell_id, device_sn, slot_index,
		pickup_code, deposited_at, picked_up_at, total_amount, paid_amount, overtime_minutes, remark, created_at, updated_at
		FROM storage_orders %s ORDER BY id DESC LIMIT ? OFFSET ?`, where)
	args = append(args, pageSize, offset)

	rows, err := r.data.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()
	orders, err := r.scanRows(rows)
	return orders, total, err
}

func (r *orderRepo) scanOne(ctx context.Context, whereClause string, args ...interface{}) (*biz.StorageOrder, error) {
	o := &biz.StorageOrder{}
	var operatorID, cellID sql.NullInt64
	var pickupCode, remark sql.NullString
	var depositedAt, pickedUpAt sql.NullTime

	err := r.data.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id, tenant_id, order_no, order_type, status, user_id, operator_id, cabinet_id, cell_id, device_sn, slot_index,
			pickup_code, deposited_at, picked_up_at, total_amount, paid_amount, overtime_minutes, remark, created_at, updated_at
			FROM storage_orders %s`, whereClause), args...,
	).Scan(&o.ID, &o.TenantID, &o.OrderNo, &o.OrderType, &o.Status, &o.UserID, &operatorID,
		&o.CabinetID, &cellID, &o.DeviceSN, &o.SlotIndex,
		&pickupCode, &depositedAt, &pickedUpAt, &o.TotalAmount, &o.PaidAmount, &o.OvertimeMinutes, &remark, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan order: %w", err)
	}
	if operatorID.Valid {
		o.OperatorID = &operatorID.Int64
	}
	if cellID.Valid {
		o.CellID = cellID.Int64
	}
	if pickupCode.Valid {
		o.PickupCode = &pickupCode.String
	}
	if depositedAt.Valid {
		o.DepositedAt = &depositedAt.Time
	}
	if pickedUpAt.Valid {
		o.PickedUpAt = &pickedUpAt.Time
	}
	if remark.Valid {
		o.Remark = &remark.String
	}
	return o, nil
}

func (r *orderRepo) scanRows(rows *sql.Rows) ([]*biz.StorageOrder, error) {
	var orders []*biz.StorageOrder
	for rows.Next() {
		o := &biz.StorageOrder{}
		var operatorID, cellID sql.NullInt64
		var pickupCode, remark sql.NullString
		var depositedAt, pickedUpAt sql.NullTime
		rows.Scan(&o.ID, &o.TenantID, &o.OrderNo, &o.OrderType, &o.Status, &o.UserID, &operatorID,
			&o.CabinetID, &cellID, &o.DeviceSN, &o.SlotIndex,
			&pickupCode, &depositedAt, &pickedUpAt, &o.TotalAmount, &o.PaidAmount, &o.OvertimeMinutes, &remark, &o.CreatedAt, &o.UpdatedAt)
		if operatorID.Valid {
			o.OperatorID = &operatorID.Int64
		}
		if cellID.Valid {
			o.CellID = cellID.Int64
		}
		if pickupCode.Valid {
			o.PickupCode = &pickupCode.String
		}
		if depositedAt.Valid {
			o.DepositedAt = &depositedAt.Time
		}
		if pickedUpAt.Valid {
			o.PickedUpAt = &pickedUpAt.Time
		}
		if remark.Valid {
			o.Remark = &remark.String
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func generateOrderNo() string {
	return fmt.Sprintf("ST%d", time.Now().UnixNano())
}
