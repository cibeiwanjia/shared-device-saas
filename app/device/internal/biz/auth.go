package biz

import (
	"context"
	"fmt"

	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
)

type DeviceMQTTAuthUsecase struct {
	jwtManager *auth.JWTManager
	log        *log.Helper
}

func NewDeviceMQTTAuthUsecase(jwtManager *auth.JWTManager, logger log.Logger) *DeviceMQTTAuthUsecase {
	return &DeviceMQTTAuthUsecase{jwtManager: jwtManager, log: log.NewHelper(logger)}
}

func (uc *DeviceMQTTAuthUsecase) GenerateDeviceToken(ctx context.Context, deviceID string, tenantID int64, deviceType string) (string, error) {
	if uc.jwtManager == nil {
		return "", fmt.Errorf("JWT manager not configured")
	}

	claims := auth.Claims{
		UserID:   deviceID,
		TenantID: tenantID,
		DeviceID: deviceID,
		Roles:    []string{"device"},
	}

	token, err := uc.jwtManager.GenerateTokenPair(deviceID, tenantID, deviceID, deviceType, []string{"device"})
	if err != nil {
		return "", fmt.Errorf("generate device token: %w", err)
	}
	_ = claims
	return token.AccessToken, nil
}
