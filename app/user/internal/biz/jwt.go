package biz

import (
	"context"

	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
)

// JwtUsecase JWT 业务逻辑（委托给 auth.JWTManager）
type JwtUsecase struct {
	jwtMgr *auth.JWTManager
	log    *log.Helper
}

// NewJwtUsecase 创建 JwtUsecase
func NewJwtUsecase(jwtMgr *auth.JWTManager, logger log.Logger) *JwtUsecase {
	return &JwtUsecase{jwtMgr: jwtMgr, log: log.NewHelper(logger)}
}

// GenerateTokenPair 签发 Token 对
func (uc *JwtUsecase) GenerateTokenPair(ctx context.Context, userID string, tenantID int64, sessionID, deviceID string, roles []string) (*auth.TokenPair, error) {
	return uc.jwtMgr.GenerateTokenPair(userID, tenantID, sessionID, deviceID, roles)
}

// ParseAccessToken 解析 Access Token
func (uc *JwtUsecase) ParseAccessToken(ctx context.Context, tokenStr string) (*auth.Claims, error) {
	return uc.jwtMgr.ParseAccessToken(tokenStr)
}

// ParseRefreshToken 解析 Refresh Token
func (uc *JwtUsecase) ParseRefreshToken(ctx context.Context, tokenStr string) (*auth.Claims, error) {
	return uc.jwtMgr.ParseRefreshToken(tokenStr)
}
