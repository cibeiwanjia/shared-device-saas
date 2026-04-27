package biz

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
)

var (
	ErrDeviceNotFound    = errors.NotFound("DEVICE_NOT_FOUND", "设备不存在")
	ErrDeviceAlreadyExists = errors.Conflict("DEVICE_ALREADY_EXISTS", "设备已存在")
	ErrDeviceBusy        = errors.Conflict("DEVICE_BUSY", "设备正在被使用")
	ErrDeviceOffline     = errors.ServiceUnavailable("DEVICE_OFFLINE", "设备离线")
)

type Device struct {
	ID           int64
	TenantID     int64
	DeviceType   string
	DeviceSN     string
	Name         string
	Status       int32
	LocationLat  float64
	LocationLng  float64
	LocationName string
	StationID    int64
	BatteryLevel uint8
	Metadata     map[string]interface{}
	LastOnlineAt  time.Time
	LastOfflineAt time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type DeviceRepo interface {
	Create(ctx context.Context, d *Device) (*Device, error)
	FindByID(ctx context.Context, id int64) (*Device, error)
	FindBySN(ctx context.Context, tenantID int64, deviceSN string) (*Device, error)
	ListByType(ctx context.Context, tenantID int64, deviceType string, status int32, page, pageSize int32) ([]*Device, int32, error)
	UpdateStatus(ctx context.Context, id int64, status int32) error
	Update(ctx context.Context, d *Device) (*Device, error)
}

type DeviceUsecase struct {
	repo DeviceRepo
}

func NewDeviceUsecase(repo DeviceRepo) *DeviceUsecase {
	return &DeviceUsecase{repo: repo}
}

func (uc *DeviceUsecase) RegisterDevice(ctx context.Context, d *Device) (*Device, error) {
	existing, err := uc.repo.FindBySN(ctx, d.TenantID, d.DeviceSN)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrDeviceAlreadyExists
	}
	return uc.repo.Create(ctx, d)
}

func (uc *DeviceUsecase) GetDevice(ctx context.Context, id int64) (*Device, error) {
	d, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, ErrDeviceNotFound
	}
	return d, nil
}

func (uc *DeviceUsecase) ListDevices(ctx context.Context, tenantID int64, deviceType string, status int32, page, pageSize int32) ([]*Device, int32, error) {
	return uc.repo.ListByType(ctx, tenantID, deviceType, status, page, pageSize)
}

func (uc *DeviceUsecase) UpdateDeviceStatus(ctx context.Context, id int64, status int32) error {
	return uc.repo.UpdateStatus(ctx, id, status)
}

func (uc *DeviceUsecase) UpdateDevice(ctx context.Context, d *Device) (*Device, error) {
	return uc.repo.Update(ctx, d)
}
