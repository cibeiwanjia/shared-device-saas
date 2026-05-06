package biz

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"shared-device-saas/pkg/redis"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var (
	ErrPickupCodeNotFound = errors.NotFound("PICKUP_CODE_NOT_FOUND", "取件码无效或已过期")
	ErrPickupCodeGenerate = errors.InternalServer("PICKUP_CODE_GENERATE_FAILED", "取件码生成失败，请重试")
)

const (
	PickupCodeKeyPrefix = "storage:pickup:"
	PickupCodeTTL       = 72 * time.Hour
	PickupCodeOrderKey  = "storage:pickup:order:"
)

type PickupCodeManager struct {
	redis *redis.Client
	log   *log.Helper
}

func NewPickupCodeManager(redisClient *redis.Client, logger log.Logger) *PickupCodeManager {
	return &PickupCodeManager{redis: redisClient, log: log.NewHelper(logger)}
}

func (m *PickupCodeManager) Generate(ctx context.Context, tenantID, cabinetID, orderID int64) (string, error) {
	key := fmt.Sprintf("%s%d:%d", PickupCodeKeyPrefix, tenantID, cabinetID)

	for i := 0; i < 10; i++ {
		code := fmt.Sprintf("%06d", rand.Intn(900000)+100000)

		exists, err := m.redis.GetClient().SIsMember(ctx, key, code).Result()
		if err != nil {
			return "", fmt.Errorf("check pickup code: %w", err)
		}
		if !exists {
			m.redis.GetClient().SAdd(ctx, key, code)
			m.redis.GetClient().Expire(ctx, key, PickupCodeTTL)
			codeMapKey := fmt.Sprintf("%s%d:%s", PickupCodeKeyPrefix, tenantID, code)
			m.redis.Set(ctx, codeMapKey, fmt.Sprintf("%d", orderID), PickupCodeTTL)
			return code, nil
		}
	}
	return "", ErrPickupCodeGenerate
}

func (m *PickupCodeManager) Verify(ctx context.Context, tenantID int64, code string) (int64, error) {
	codeMapKey := fmt.Sprintf("%s%d:%s", PickupCodeKeyPrefix, tenantID, code)
	data, err := m.redis.Get(ctx, codeMapKey)
	if err != nil {
		return 0, fmt.Errorf("verify pickup code: %w", err)
	}
	if data == "" {
		return 0, ErrPickupCodeNotFound
	}
	var orderID int64
	fmt.Sscanf(data, "%d", &orderID)
	return orderID, nil
}

func (m *PickupCodeManager) Revoke(ctx context.Context, tenantID int64, cabinetID int64, code string) error {
	key := fmt.Sprintf("%s%d:%d", PickupCodeKeyPrefix, tenantID, cabinetID)
	m.redis.GetClient().SRem(ctx, key, code)
	codeMapKey := fmt.Sprintf("%s%d:%s", PickupCodeKeyPrefix, tenantID, code)
	return m.redis.Del(ctx, codeMapKey)
}
