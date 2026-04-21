package biz

import (
	"context"
	"fmt"
	"time"

	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
)

// JwtUsecase JWT 业务逻辑
type JwtUsecase struct {
	jwtMgr    *auth.JWTManager
	rotator   *auth.Rotator
	blacklist auth.Blacklist
	sessions  *auth.SessionManager
	audit     auth.AuditLogger
	log       *log.Helper
}

// NewJwtUsecase 创建 JwtUsecase
func NewJwtUsecase(
	jwtMgr *auth.JWTManager,
	rotator *auth.Rotator,
	blacklist auth.Blacklist,
	sessions *auth.SessionManager,
	audit auth.AuditLogger,
	logger log.Logger,
) *JwtUsecase {
	return &JwtUsecase{
		jwtMgr:    jwtMgr,
		rotator:   rotator,
		blacklist: blacklist,
		sessions:  sessions,
		audit:     audit,
		log:       log.NewHelper(logger),
	}
}

// Login 登录签发 Token
func (uc *JwtUsecase) Login(ctx context.Context, userID, tenantID int64, deviceID, deviceName, ip, userAgent string) (*auth.TokenPair, error) {
	sessionID := auth.NewJTI()

	// 签发 Token
	pair, err := uc.jwtMgr.GenerateTokenPair(userID, tenantID, sessionID, deviceID, []string{"user"})
	if err != nil {
		return nil, fmt.Errorf("generate token pair: %w", err)
	}

	// 存储 Refresh Token 哈希
	if err := uc.rotator.Store(ctx, sessionID, pair.RefreshToken); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	// 保存会话信息
	session := &auth.SessionInfo{
		SessionID:  sessionID,
		DeviceID:   deviceID,
		DeviceName: deviceName,
		IP:         ip,
		UserAgent:  userAgent,
		LoginAt:    time.Now(),
	}
	if err := uc.sessions.CreateSession(ctx, userID, session); err != nil {
		uc.log.Warnf("save session failed: %v", err)
	}

	// 审计日志
	_ = uc.audit.Log(ctx, &auth.AuditEvent{
		EventType: auth.AuditEventLoginSuccess,
		UserID:    userID,
		TenantID:  tenantID,
		SessionID: sessionID,
		DeviceID:  deviceID,
		IP:        ip,
		Timestamp: time.Now(),
	})

	return pair, nil
}

// RefreshToken 刷新 Token
func (uc *JwtUsecase) RefreshToken(ctx context.Context, oldRefreshToken, deviceID string) (*auth.TokenPair, error) {
	// 解析旧 Refresh Token
	claims, err := uc.jwtMgr.ParseRefreshToken(oldRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("parse refresh token: %w", err)
	}

	// 轮换验证
	valid, reuseDetected, err := uc.rotator.Rotate(ctx, claims.SessionID, oldRefreshToken, "")
	if err != nil {
		return nil, fmt.Errorf("rotate refresh token: %w", err)
	}

	if reuseDetected {
		// 重用检测 → 撤销用户所有 session
		_ = uc.rotator.RevokeAll(ctx, claims.UserID)
		_ = uc.sessions.RevokeAllSessions(ctx, claims.UserID)
		_ = uc.audit.Log(ctx, &auth.AuditEvent{
			EventType: auth.AuditEventTokenReuse,
			UserID:    claims.UserID,
			TenantID:  claims.TenantID,
			SessionID: claims.SessionID,
			DeviceID:  deviceID,
			Timestamp: time.Now(),
			Detail:    "refresh token reuse detected, all sessions revoked",
		})
		return nil, fmt.Errorf("token reuse detected, all sessions revoked")
	}

	if !valid {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// 签发新 Token 对
	newPair, err := uc.jwtMgr.GenerateTokenPair(claims.UserID, claims.TenantID, claims.SessionID, claims.DeviceID, claims.Roles)
	if err != nil {
		return nil, fmt.Errorf("generate new token pair: %w", err)
	}

	// 存储新 Refresh Token
	if err := uc.rotator.Store(ctx, claims.SessionID, newPair.RefreshToken); err != nil {
		return nil, fmt.Errorf("store new refresh token: %w", err)
	}

	// 审计日志
	_ = uc.audit.Log(ctx, &auth.AuditEvent{
		EventType: auth.AuditEventTokenRefreshed,
		UserID:    claims.UserID,
		TenantID:  claims.TenantID,
		SessionID: claims.SessionID,
		JTI:       claims.ID,
		DeviceID:  deviceID,
		Timestamp: time.Now(),
	})

	return newPair, nil
}

// Logout 登出
func (uc *JwtUsecase) Logout(ctx context.Context, accessToken, sessionID string) error {
	// 解析 Access Token 获取 jti
	claims, err := uc.jwtMgr.ParseAccessToken(accessToken)
	if err != nil {
		return fmt.Errorf("parse access token: %w", err)
	}

	// 加入黑名单（TTL = Token 剩余有效期）
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl > 0 {
		_ = uc.blacklist.Add(ctx, claims.ID, ttl)
	}

	// 撤销 Refresh Token
	_ = uc.rotator.Revoke(ctx, sessionID)

	// 撤销会话
	_ = uc.sessions.RevokeSession(ctx, sessionID)

	// 审计日志
	_ = uc.audit.Log(ctx, &auth.AuditEvent{
		EventType: auth.AuditEventTokenRevoked,
		UserID:    claims.UserID,
		TenantID:  claims.TenantID,
		SessionID: sessionID,
		JTI:       claims.ID,
		Timestamp: time.Now(),
	})

	return nil
}

// ListSessions 获取用户活跃会话
func (uc *JwtUsecase) ListSessions(ctx context.Context, userID int64) ([]*auth.SessionInfo, error) {
	return uc.sessions.ListSessions(ctx, userID)
}

// RevokeSession 踢掉指定设备
func (uc *JwtUsecase) RevokeSession(ctx context.Context, sessionID string) error {
	_ = uc.rotator.Revoke(ctx, sessionID)
	return uc.sessions.RevokeSession(ctx, sessionID)
}
