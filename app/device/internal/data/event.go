package data

import (
	"context"
	"fmt"
	"time"

	"shared-device-saas/app/device/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type connectionEventRepo struct {
	data *Data
	log  *log.Helper
}

func NewConnectionEventRepo(data *Data, logger log.Logger) biz.ConnectionEventRepo {
	return &connectionEventRepo{data: data, log: log.NewHelper(logger)}
}

func (r *connectionEventRepo) Create(ctx context.Context, e *biz.ConnectionEvent) error {
	_, err := r.data.DB().ExecContext(ctx,
		`INSERT INTO device_connection_events (tenant_id, device_id, event_type, reason_code, ip_address, client_id, occurred_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.TenantID, e.DeviceID, e.EventType, e.ReasonCode, e.IPAddress, e.ClientID, e.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("insert connection event: %w", err)
	}
	return nil
}

func (r *connectionEventRepo) ListByDevice(ctx context.Context, deviceID int64, eventType string, start, end time.Time, page, pageSize int32) ([]*biz.ConnectionEvent, int32, error) {
	where := "WHERE device_id = ?"
	args := []interface{}{deviceID}

	if eventType != "" {
		where += " AND event_type = ?"
		args = append(args, eventType)
	}
	if !start.IsZero() {
		where += " AND occurred_at >= ?"
		args = append(args, start)
	}
	if !end.IsZero() {
		where += " AND occurred_at <= ?"
		args = append(args, end)
	}

	var total int32
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	r.data.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM device_connection_events "+where, countArgs...).Scan(&total)

	offset := (page - 1) * pageSize
	query := fmt.Sprintf("SELECT id, tenant_id, device_id, event_type, reason_code, ip_address, client_id, occurred_at FROM device_connection_events %s ORDER BY occurred_at DESC LIMIT ? OFFSET ?", where)
	args = append(args, pageSize, offset)

	rows, err := r.data.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list connection events: %w", err)
	}
	defer rows.Close()

	var events []*biz.ConnectionEvent
	for rows.Next() {
		e := &biz.ConnectionEvent{}
		rows.Scan(&e.ID, &e.TenantID, &e.DeviceID, &e.EventType, &e.ReasonCode, &e.IPAddress, &e.ClientID, &e.OccurredAt)
		events = append(events, e)
	}
	return events, total, nil
}

func (r *connectionEventRepo) CleanBefore(ctx context.Context, before time.Time) error {
	_, err := r.data.DB().ExecContext(ctx, "DELETE FROM device_connection_events WHERE occurred_at < ?", before)
	return err
}
