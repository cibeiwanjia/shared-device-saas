package server

import (
	"time"

	"shared-device-saas/app/storage/internal/conf"
	"shared-device-saas/pkg/auth"
	"shared-device-saas/pkg/userclient"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// ProviderSet is server providers.
var ProviderSet = wire.NewSet(
	NewGRPCServer,
	NewHTTPServer,
	NewJWTManager,
	NewUserClient,
)

// NewJWTManager 创建 JWT Manager
func NewJWTManager(c *conf.Data, logger log.Logger) *auth.JWTManager {
	helper := log.NewHelper(logger)
	jwtCfg := c.GetJwt()
	if jwtCfg == nil {
		helper.Warn("JWT config not found")
		return nil
	}

	accessExpiry := 2 * time.Hour // 2小时
	if jwtCfg.AccessExpiry != nil {
		accessExpiry = jwtCfg.AccessExpiry.AsDuration()
	}

	refreshExpiry := 7 * 24 * time.Hour // 7天
	if jwtCfg.RefreshExpiry != nil {
		refreshExpiry = jwtCfg.RefreshExpiry.AsDuration()
	}

	helper.Info("JWT Manager initialized")
	return auth.NewJWTManager(
		jwtCfg.AccessSecret,
		jwtCfg.RefreshSecret,
		accessExpiry,
		refreshExpiry,
	)
}

// NewUserClient 创建 User 服务 gRPC 客户端
func NewUserClient(c *conf.Data, logger log.Logger) (*userclient.UserClient, func(), error) {
	helper := log.NewHelper(logger)
	userCfg := c.GetUserClient()
	if userCfg == nil {
		helper.Warn("UserClient config not found")
		return nil, func() {}, nil
	}

	helper.Infof("User client initialized: endpoint=%s", userCfg.Endpoint)
	client, err := userclient.NewUserClient(userCfg.Endpoint, logger)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		helper.Info("Closing User client...")
		if client != nil {
			client.Close()
		}
	}

	return client, cleanup, nil
}