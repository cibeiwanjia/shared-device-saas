package biz

import "context"

type StorageEvent struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	TenantID  int64  `json:"tenant_id"`
	OrderNo   string `json:"order_no"`
	UserID    int64  `json:"user_id"`
	CabinetID int64  `json:"cabinet_id,omitempty"`
	CellID    int64  `json:"cell_id,omitempty"`
	Payload   string `json:"payload"`
	Timestamp int64  `json:"timestamp"`
}

type EventPublisher interface {
	PublishPickupReady(ctx context.Context, tenantID int64, orderNo string, userID int64, pickupCode string, cabinetName string) error
	PublishOrderTimeout(ctx context.Context, tenantID int64, orderNo string, userID int64, fee int32) error
	PublishOpenTimeout(ctx context.Context, tenantID int64, deviceSN string, cellID int64, orderNo string) error
}
