package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"shared-device-saas/app/device/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type deviceRepo struct {
	data *Data
	log  *log.Helper
}

func NewDeviceRepo(data *Data, logger log.Logger) biz.DeviceRepo {
	return &deviceRepo{data: data, log: log.NewHelper(logger)}
}

func (r *deviceRepo) Create(ctx context.Context, d *biz.Device) (*biz.Device, error) {
	metadataJSON, _ := json.Marshal(d.Metadata)
	result, err := r.data.DB().ExecContext(ctx,
		`INSERT INTO devices (tenant_id, device_type, device_sn, name, status, location_lat, location_lng, location_name, station_id, battery_level, metadata_json, last_online_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.TenantID, d.DeviceType, d.DeviceSN, d.Name, d.Status,
		d.LocationLat, d.LocationLng, d.LocationName, d.StationID,
		d.BatteryLevel, string(metadataJSON), d.LastOnlineAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert device: %w", err)
	}
	id, _ := result.LastInsertId()
	d.ID = id
	return d, nil
}

func (r *deviceRepo) FindByID(ctx context.Context, id int64) (*biz.Device, error) {
	d := &biz.Device{}
	var metadataStr sql.NullString
	var lat, lng sql.NullFloat64
	var stationID sql.NullInt64
	var battery sql.NullInt32
	var lastOnline, lastOffline sql.NullTime

	err := r.data.DB().QueryRowContext(ctx,
		`SELECT id, tenant_id, device_type, device_sn, name, status, location_lat, location_lng, location_name, station_id, battery_level, metadata_json, last_online_at, last_offline_at, created_at, updated_at
		 FROM devices WHERE id = ?`, id,
	).Scan(&d.ID, &d.TenantID, &d.DeviceType, &d.DeviceSN, &d.Name, &d.Status,
		&lat, &lng, &d.LocationName, &stationID, &battery, &metadataStr,
		&lastOnline, &lastOffline, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find device by id: %w", err)
	}

	if lat.Valid {
		d.LocationLat = lat.Float64
	}
	if lng.Valid {
		d.LocationLng = lng.Float64
	}
	if stationID.Valid {
		d.StationID = stationID.Int64
	}
	if battery.Valid {
		d.BatteryLevel = uint8(battery.Int32)
	}
	if metadataStr.Valid {
		json.Unmarshal([]byte(metadataStr.String), &d.Metadata)
	}
	if lastOnline.Valid {
		d.LastOnlineAt = lastOnline.Time
	}
	if lastOffline.Valid {
		d.LastOfflineAt = lastOffline.Time
	}
	return d, nil
}

func (r *deviceRepo) FindBySN(ctx context.Context, tenantID int64, deviceSN string) (*biz.Device, error) {
	d := &biz.Device{}
	var metadataStr sql.NullString
	var lat, lng sql.NullFloat64
	var stationID sql.NullInt64
	var battery sql.NullInt32
	var lastOnline, lastOffline sql.NullTime

	err := r.data.DB().QueryRowContext(ctx,
		`SELECT id, tenant_id, device_type, device_sn, name, status, location_lat, location_lng, location_name, station_id, battery_level, metadata_json, last_online_at, last_offline_at, created_at, updated_at
		 FROM devices WHERE tenant_id = ? AND device_sn = ?`, tenantID, deviceSN,
	).Scan(&d.ID, &d.TenantID, &d.DeviceType, &d.DeviceSN, &d.Name, &d.Status,
		&lat, &lng, &d.LocationName, &stationID, &battery, &metadataStr,
		&lastOnline, &lastOffline, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find device by sn: %w", err)
	}

	if lat.Valid {
		d.LocationLat = lat.Float64
	}
	if lng.Valid {
		d.LocationLng = lng.Float64
	}
	if stationID.Valid {
		d.StationID = stationID.Int64
	}
	if battery.Valid {
		d.BatteryLevel = uint8(battery.Int32)
	}
	if metadataStr.Valid {
		json.Unmarshal([]byte(metadataStr.String), &d.Metadata)
	}
	if lastOnline.Valid {
		d.LastOnlineAt = lastOnline.Time
	}
	if lastOffline.Valid {
		d.LastOfflineAt = lastOffline.Time
	}
	return d, nil
}

func (r *deviceRepo) ListByType(ctx context.Context, tenantID int64, deviceType string, status int32, page, pageSize int32) ([]*biz.Device, int32, error) {
	where := "WHERE tenant_id = ?"
	args := []interface{}{tenantID}
	if deviceType != "" {
		where += " AND device_type = ?"
		args = append(args, deviceType)
	}
	if status >= 0 {
		where += " AND status = ?"
		args = append(args, status)
	}

	var total int32
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	r.data.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM devices "+where, countArgs...).Scan(&total)

	offset := (page - 1) * pageSize
	query := fmt.Sprintf("SELECT id, tenant_id, device_type, device_sn, name, status, battery_level, last_online_at, created_at, updated_at FROM devices %s ORDER BY id DESC LIMIT ? OFFSET ?", where)
	args = append(args, pageSize, offset)

	rows, err := r.data.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	var devices []*biz.Device
	for rows.Next() {
		d := &biz.Device{}
		var lastOnline sql.NullTime
		var battery sql.NullInt32
		rows.Scan(&d.ID, &d.TenantID, &d.DeviceType, &d.DeviceSN, &d.Name, &d.Status, &battery, &lastOnline, &d.CreatedAt, &d.UpdatedAt)
		if battery.Valid {
			d.BatteryLevel = uint8(battery.Int32)
		}
		if lastOnline.Valid {
			d.LastOnlineAt = lastOnline.Time
		}
		devices = append(devices, d)
	}
	return devices, total, nil
}

func (r *deviceRepo) UpdateStatus(ctx context.Context, id int64, status int32) error {
	now := time.Now()
	var err error
	if status == 1 {
		_, err = r.data.DB().ExecContext(ctx, "UPDATE devices SET status = ?, last_online_at = ? WHERE id = ?", status, now, id)
	} else {
		_, err = r.data.DB().ExecContext(ctx, "UPDATE devices SET status = ?, last_offline_at = ? WHERE id = ?", status, now, id)
	}
	return err
}

func (r *deviceRepo) Update(ctx context.Context, d *biz.Device) (*biz.Device, error) {
	metadataJSON, _ := json.Marshal(d.Metadata)
	_, err := r.data.DB().ExecContext(ctx,
		`UPDATE devices SET name=?, status=?, location_lat=?, location_lng=?, location_name=?, station_id=?, battery_level=?, metadata_json=?, updated_at=? WHERE id=?`,
		d.Name, d.Status, d.LocationLat, d.LocationLng, d.LocationName, d.StationID, d.BatteryLevel, string(metadataJSON), time.Now(), d.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update device: %w", err)
	}
	return d, nil
}
