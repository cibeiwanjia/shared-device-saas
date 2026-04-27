package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"shared-device-saas/pkg/redis"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	DeviceStatusKeyPrefix = "device:status:"
	DeviceLockKeyPrefix   = "device:lock:"
	DeviceSlotsKeyPrefix  = "device:slots:"
	DeviceStatusTTL       = 5 * time.Minute
	DeviceLockTTL         = 10 * time.Second
)

type DeviceStatus struct {
	DeviceID     string `json:"device_id"`
	DeviceType   string `json:"device_type"`
	Status       int32  `json:"status"`
	BatteryLevel uint8  `json:"battery_level"`
	TenantID     string `json:"tenant_id"`
	UpdatedAt    int64  `json:"updated_at"`
}

type InventoryUsecase struct {
	repo   DeviceRepo
	redis  *redis.Client
	log    *log.Helper
}

func NewInventoryUsecase(repo DeviceRepo, redisClient *redis.Client, logger log.Logger) *InventoryUsecase {
	return &InventoryUsecase{repo: repo, redis: redisClient, log: log.NewHelper(logger)}
}

func (uc *InventoryUsecase) CacheDeviceStatus(ctx context.Context, tenantID string, deviceID string, status *DeviceStatus) error {
	key := fmt.Sprintf("%s%s:%s", DeviceStatusKeyPrefix, tenantID, deviceID)
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal device status: %w", err)
	}
	return uc.redis.Set(ctx, key, string(data), DeviceStatusTTL)
}

func (uc *InventoryUsecase) GetCachedStatus(ctx context.Context, tenantID string, deviceID string) (*DeviceStatus, error) {
	key := fmt.Sprintf("%s%s:%s", DeviceStatusKeyPrefix, tenantID, deviceID)
	data, err := uc.redis.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if data == "" {
		return nil, nil
	}
	var status DeviceStatus
	if err := json.Unmarshal([]byte(data), &status); err != nil {
		return nil, fmt.Errorf("unmarshal device status: %w", err)
	}
	return &status, nil
}

func (uc *InventoryUsecase) LockDevice(ctx context.Context, deviceID string, userID string) (bool, error) {
	key := fmt.Sprintf("%s%s", DeviceLockKeyPrefix, deviceID)
	return uc.redis.SetNX(ctx, key, userID, DeviceLockTTL)
}

func (uc *InventoryUsecase) UnlockDevice(ctx context.Context, deviceID string) error {
	key := fmt.Sprintf("%s%s", DeviceLockKeyPrefix, deviceID)
	return uc.redis.Del(ctx, key)
}

func (uc *InventoryUsecase) UpdateSlotStatus(ctx context.Context, tenantID string, deviceID string, slots map[string]string) error {
	key := fmt.Sprintf("%s%s:%s", DeviceSlotsKeyPrefix, tenantID, deviceID)
	for slotIndex, slotStatus := range slots {
		field := fmt.Sprintf("slot_%s", slotIndex)
		uc.redis.GetClient().HSet(ctx, key, field, slotStatus)
	}
	uc.redis.GetClient().Expire(ctx, key, DeviceStatusTTL)
	return nil
}

func (uc *InventoryUsecase) GetSlotStatus(ctx context.Context, tenantID string, deviceID string) (map[string]string, error) {
	key := fmt.Sprintf("%s%s:%s", DeviceSlotsKeyPrefix, tenantID, deviceID)
	result, err := uc.redis.GetClient().HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (uc *InventoryUsecase) GetAvailableSlotCount(ctx context.Context, tenantID string, deviceID string) (int, error) {
	slots, err := uc.GetSlotStatus(ctx, tenantID, deviceID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, status := range slots {
		if status == "free" {
			count++
		}
	}
	return count, nil
}
