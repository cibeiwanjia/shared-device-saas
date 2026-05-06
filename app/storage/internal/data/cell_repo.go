package data

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"shared-device-saas/app/storage/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type cellRepo struct {
	data *Data
	log  *log.Helper
}

func NewCellRepo(data *Data, logger log.Logger) biz.CellRepo {
	return &cellRepo{data: data, log: log.NewHelper(logger)}
}

func (r *cellRepo) FindByID(ctx context.Context, id int64) (*biz.Cell, error) {
	return r.scanOne(ctx, "WHERE c.id = ?", id)
}

func (r *cellRepo) FindByDeviceAndSlot(ctx context.Context, deviceSN string, slotIndex int32) (*biz.Cell, error) {
	return r.scanOne(ctx, "JOIN cabinets cab ON c.cabinet_id = cab.id WHERE cab.device_sn = ? AND c.slot_index = ?", deviceSN, slotIndex)
}

func (r *cellRepo) AllocateForUpdate(ctx context.Context, cabinetID int64, cellType int32) (*biz.Cell, error) {
	c := &biz.Cell{}
	var currentOrderID sql.NullInt64
	var pendingAction sql.NullInt32
	var openedAt sql.NullTime

	err := r.data.db.QueryRowContext(ctx,
		`SELECT c.id, c.tenant_id, c.cabinet_id, c.slot_index, c.cell_type, c.status, c.current_order_id, c.pending_action, c.opened_at, c.created_at, c.updated_at
		 FROM cells c WHERE c.cabinet_id = ? AND c.status = ? AND (c.cell_type = ? OR 0 = ?)
		 ORDER BY c.slot_index LIMIT 1 FOR UPDATE`,
		cabinetID, biz.CellStatusFree, cellType, cellType,
	).Scan(&c.ID, &c.TenantID, &c.CabinetID, &c.SlotIndex, &c.CellType, &c.Status, &currentOrderID, &pendingAction, &openedAt, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("allocate cell for update: %w", err)
	}

	if currentOrderID.Valid {
		c.CurrentOrderID = ptrInt64(currentOrderID.Int64)
	}
	if pendingAction.Valid {
		c.PendingAction = ptrInt32(pendingAction.Int32)
	}
	if openedAt.Valid {
		c.OpenedAt = &openedAt.Time
	}

	_, err = r.data.db.ExecContext(ctx,
		"UPDATE cells SET status = ?, current_order_id = NULL WHERE id = ?",
		biz.CellStatusOpening, c.ID)
	if err != nil {
		return nil, fmt.Errorf("mark cell opening: %w", err)
	}
	c.Status = biz.CellStatusOpening
	return c, nil
}

func (r *cellRepo) UpdateStatus(ctx context.Context, id int64, status int32) error {
	_, err := r.data.db.ExecContext(ctx, "UPDATE cells SET status = ? WHERE id = ?", status, id)
	return err
}

func (r *cellRepo) UpdateStatusCAS(ctx context.Context, id int64, expectedFrom, newStatus int32) (int64, error) {
	result, err := r.data.db.ExecContext(ctx,
		"UPDATE cells SET status = ? WHERE id = ? AND status = ?", newStatus, id, expectedFrom)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func (r *cellRepo) UpdateOpenedAt(ctx context.Context, id int64, t time.Time) error {
	_, err := r.data.db.ExecContext(ctx, "UPDATE cells SET opened_at = ? WHERE id = ?", t, id)
	return err
}

func (r *cellRepo) UpdatePendingAction(ctx context.Context, id int64, action *int32, orderID *int64) error {
	_, err := r.data.db.ExecContext(ctx,
		"UPDATE cells SET pending_action = ?, current_order_id = ? WHERE id = ?",
		action, orderID, id)
	return err
}

func (r *cellRepo) FindOpenTimeoutCells(ctx context.Context, threshold time.Duration) ([]*biz.Cell, error) {
	cutoff := time.Now().Add(-threshold)
	rows, err := r.data.db.QueryContext(ctx,
		`SELECT id, tenant_id, cabinet_id, slot_index, cell_type, status, current_order_id, pending_action, opened_at, created_at, updated_at
		 FROM cells WHERE status = ? AND opened_at IS NOT NULL AND opened_at < ?`,
		biz.CellStatusOpening, cutoff)
	if err != nil {
		return nil, fmt.Errorf("find open timeout cells: %w", err)
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *cellRepo) ListFreeByCabinet(ctx context.Context, cabinetID int64) ([]*biz.Cell, error) {
	rows, err := r.data.db.QueryContext(ctx,
		`SELECT id, tenant_id, cabinet_id, slot_index, cell_type, status, current_order_id, pending_action, opened_at, created_at, updated_at
		 FROM cells WHERE cabinet_id = ? AND status = ? ORDER BY slot_index`, cabinetID, biz.CellStatusFree)
	if err != nil {
		return nil, fmt.Errorf("list free cells: %w", err)
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *cellRepo) scanOne(ctx context.Context, whereClause string, args ...interface{}) (*biz.Cell, error) {
	c := &biz.Cell{}
	var currentOrderID sql.NullInt64
	var pendingAction sql.NullInt32
	var openedAt sql.NullTime

	query := fmt.Sprintf(
		`SELECT c.id, c.tenant_id, c.cabinet_id, c.slot_index, c.cell_type, c.status, c.current_order_id, c.pending_action, c.opened_at, c.created_at, c.updated_at
		 FROM cells c %s`, whereClause)

	err := r.data.db.QueryRowContext(ctx, query, args...).Scan(
		&c.ID, &c.TenantID, &c.CabinetID, &c.SlotIndex, &c.CellType, &c.Status,
		&currentOrderID, &pendingAction, &openedAt, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan cell: %w", err)
	}
	if currentOrderID.Valid {
		c.CurrentOrderID = ptrInt64(currentOrderID.Int64)
	}
	if pendingAction.Valid {
		c.PendingAction = ptrInt32(pendingAction.Int32)
	}
	if openedAt.Valid {
		c.OpenedAt = &openedAt.Time
	}
	return c, nil
}

func (r *cellRepo) scanRows(rows *sql.Rows) ([]*biz.Cell, error) {
	var cells []*biz.Cell
	for rows.Next() {
		c := &biz.Cell{}
		var currentOrderID sql.NullInt64
		var pendingAction sql.NullInt32
		var openedAt sql.NullTime
		rows.Scan(&c.ID, &c.TenantID, &c.CabinetID, &c.SlotIndex, &c.CellType, &c.Status,
			&currentOrderID, &pendingAction, &openedAt, &c.CreatedAt, &c.UpdatedAt)
		if currentOrderID.Valid {
			c.CurrentOrderID = ptrInt64(currentOrderID.Int64)
		}
		if pendingAction.Valid {
			c.PendingAction = ptrInt32(pendingAction.Int32)
		}
		if openedAt.Valid {
			c.OpenedAt = &openedAt.Time
		}
		cells = append(cells, c)
	}
	return cells, nil
}

func ptrInt64(v int64) *int64    { return &v }
func ptrInt32(v int32) *int32    { return &v }
