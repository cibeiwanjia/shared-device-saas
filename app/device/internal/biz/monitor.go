package biz

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type ConnectionEvent struct {
	ID         int64
	TenantID   int64
	DeviceID   int64
	EventType  string
	ReasonCode int
	IPAddress  string
	ClientID   string
	OccurredAt time.Time
}

type ConnectionEventRepo interface {
	Create(ctx context.Context, e *ConnectionEvent) error
	ListByDevice(ctx context.Context, deviceID int64, eventType string, start, end time.Time, page, pageSize int32) ([]*ConnectionEvent, int32, error)
	CleanBefore(ctx context.Context, before time.Time) error
}

type MonitorUsecase struct {
	eventRepo  ConnectionEventRepo
	deviceRepo DeviceRepo
	log        *log.Helper
}

func NewMonitorUsecase(eventRepo ConnectionEventRepo, deviceRepo DeviceRepo, logger log.Logger) *MonitorUsecase {
	return &MonitorUsecase{eventRepo: eventRepo, deviceRepo: deviceRepo, log: log.NewHelper(logger)}
}

func (uc *MonitorUsecase) HandleConnected(ctx context.Context, tenantID int64, deviceID int64, ip, clientID string) error {
	err := uc.eventRepo.Create(ctx, &ConnectionEvent{
		TenantID:   tenantID,
		DeviceID:   deviceID,
		EventType:  "connected",
		IPAddress:  ip,
		ClientID:   clientID,
		OccurredAt: time.Now(),
	})
	if err != nil {
		uc.log.Errorf("save connected event: %v", err)
	}

	if dbErr := uc.deviceRepo.UpdateStatus(ctx, deviceID, 1); dbErr != nil {
		uc.log.Errorf("update device online status: %v", dbErr)
	}
	return err
}

func (uc *MonitorUsecase) HandleDisconnected(ctx context.Context, tenantID int64, deviceID int64, reasonCode int, clientID string) error {
	err := uc.eventRepo.Create(ctx, &ConnectionEvent{
		TenantID:   tenantID,
		DeviceID:   deviceID,
		EventType:  "disconnected",
		ReasonCode: reasonCode,
		ClientID:   clientID,
		OccurredAt: time.Now(),
	})
	if err != nil {
		uc.log.Errorf("save disconnected event: %v", err)
	}

	if dbErr := uc.deviceRepo.UpdateStatus(ctx, deviceID, 0); dbErr != nil {
		uc.log.Errorf("update device offline status: %v", dbErr)
	}
	return err
}

func (uc *MonitorUsecase) ListEvents(ctx context.Context, deviceID int64, eventType string, start, end time.Time, page, pageSize int32) ([]*ConnectionEvent, int32, error) {
	return uc.eventRepo.ListByDevice(ctx, deviceID, eventType, start, end, page, pageSize)
}

func (uc *MonitorUsecase) CleanExpiredEvents(ctx context.Context) error {
	cutoff := time.Now().AddDate(0, 0, -90)
	return uc.eventRepo.CleanBefore(ctx, cutoff)
}
