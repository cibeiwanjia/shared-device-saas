package biz

import (
	"time"

	"shared-device-saas/app/user/internal/conf"
	"shared-device-saas/pkg/auth"

	"github.com/google/wire"
	"github.com/go-kratos/kratos/v2/log"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewUserUsecase, NewJWTManager)

// NewJWTManager creates JWT manager from config
func NewJWTManager(c *conf.Data, logger log.Logger) *auth.JWTManager {
	log.NewHelper(logger).Info("Creating JWT manager")
	
	jwtCfg := c.GetJwt()
	if jwtCfg == nil {
		// Default JWT config
		return auth.NewJWTManager(
			"shared-device-saas-access-secret-default",
			"shared-device-saas-refresh-secret-default",
			2*time.Hour,
			7*24*time.Hour,
		)
	}

	accessExpiry := 2 * time.Hour
	if jwtCfg.AccessExpiry != nil {
		accessExpiry = jwtCfg.AccessExpiry.AsDuration()
	}

	refreshExpiry := 7 * 24 * time.Hour
	if jwtCfg.RefreshExpiry != nil {
		refreshExpiry = jwtCfg.RefreshExpiry.AsDuration()
	}

	return auth.NewJWTManager(
		jwtCfg.AccessSecret,
		jwtCfg.RefreshSecret,
		accessExpiry,
		refreshExpiry,
	)
}