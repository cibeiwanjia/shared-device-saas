package data

import (
	"context"
	"database/sql"
	"fmt"

	"shared-device-saas/app/storage/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type cabinetRepo struct {
	data *Data
	log  *log.Helper
}

func NewCabinetRepo(data *Data, logger log.Logger) biz.CabinetRepo {
	return &cabinetRepo{data: data, log: log.NewHelper(logger)}
}

func (r *cabinetRepo) FindByID(ctx context.Context, id int64) (*biz.Cabinet, error) {
	c := &biz.Cabinet{}
	err := r.data.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, device_id, device_sn, name, location_name, total_cells, status, created_at, updated_at
		 FROM cabinets WHERE id = ?`, id,
	).Scan(&c.ID, &c.TenantID, &c.DeviceID, &c.DeviceSN, &c.Name, &c.LocationName, &c.TotalCells, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find cabinet by id: %w", err)
	}
	return c, nil
}

func (r *cabinetRepo) ListByTenant(ctx context.Context, tenantID int64, status int32, page, pageSize int32) ([]*biz.Cabinet, int32, error) {
	where := "WHERE tenant_id = ?"
	args := []interface{}{tenantID}
	if status > 0 {
		where += " AND status = ?"
		args = append(args, status)
	}

	var total int32
	r.data.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cabinets "+where, args...).Scan(&total)

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(`SELECT id, tenant_id, device_id, device_sn, name, location_name, total_cells, status, created_at, updated_at
		FROM cabinets %s ORDER BY id DESC LIMIT ? OFFSET ?`, where)
	args = append(args, pageSize, offset)

	rows, err := r.data.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list cabinets: %w", err)
	}
	defer rows.Close()

	var cabinets []*biz.Cabinet
	for rows.Next() {
		c := &biz.Cabinet{}
		rows.Scan(&c.ID, &c.TenantID, &c.DeviceID, &c.DeviceSN, &c.Name, &c.LocationName, &c.TotalCells, &c.Status, &c.CreatedAt, &c.UpdatedAt)
		cabinets = append(cabinets, c)
	}
	return cabinets, total, nil
}

func (r *cabinetRepo) GetFreeCellCount(ctx context.Context, cabinetID int64) (int32, error) {
	var count int32
	err := r.data.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cells WHERE cabinet_id = ? AND status = ?", cabinetID, biz.CellStatusFree,
	).Scan(&count)
	return count, err
}
