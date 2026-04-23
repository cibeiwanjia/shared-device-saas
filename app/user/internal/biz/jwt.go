package biz

import (
	"context"

	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
)

// JwtUsecase JWT 业务逻辑
type JwtUsecase struct {
	jwtCfg *auth.JWTConfig
	log    *log.Helper
}

// NewJwtUsecase 创建 JwtUsecase
func NewJwtUsecase(jwtCfg *auth.JWTConfig, logger log.Logger) *JwtUsecase {
	return &JwtUsecase{jwtCfg: jwtCfg, log: log.NewHelper(logger)}
}

// GenerateToken 签发 Access Token
func (uc *JwtUsecase) GenerateToken(ctx context.Context, userID, tenantID int64, deviceID string) (string, error) {
	return auth.GenerateToken(uc.jwtCfg, userID, tenantID, deviceID)
}

// ParseToken 解析 Token
func (uc *JwtUsecase) ParseToken(ctx context.Context, tokenStr string) (*auth.Claims, error) {
	return auth.ParseToken(uc.jwtCfg, tokenStr)
}
