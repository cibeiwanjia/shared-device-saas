package auth

import (
	"context"
	"fmt"
	"time"
)

// RefreshTokenStore Refresh Token 存储接口
type RefreshTokenStore interface {
	// Store 保存 Refresh Token 哈希，key 为 session_id
	Store(ctx context.Context, sessionID, tokenHash string, ttl time.Duration) error
	// Verify 验证 Refresh Token 哈希是否匹配，返回是否有效
	Verify(ctx context.Context, sessionID, tokenHash string) (bool, error)
	// Delete 删除指定 session 的 Refresh Token
	Delete(ctx context.Context, sessionID string) error
	// DeleteByUser 删除用户所有 Refresh Token（强制重新登录）
	DeleteByUser(ctx context.Context, userID int64) error
}

// Rotator Refresh Token 轮换器
type Rotator struct {
	store      RefreshTokenStore
	refreshTTL time.Duration
}

// NewRotator 创建轮换器
func NewRotator(store RefreshTokenStore, refreshTTL time.Duration) *Rotator {
	return &Rotator{store: store, refreshTTL: refreshTTL}
}

// Store 存储 Refresh Token 哈希
func (r *Rotator) Store(ctx context.Context, sessionID, refreshToken string) error {
	tokenHash := HashToken(refreshToken)
	return r.store.Store(ctx, sessionID, tokenHash, r.refreshTTL)
}

// Rotate 轮换验证：校验旧 Token，存储新 Token
// 返回 (valid, reuseDetected, error)
func (r *Rotator) Rotate(ctx context.Context, sessionID, oldRefreshToken, newRefreshToken string) (bool, bool, error) {
	oldHash := HashToken(oldRefreshToken)

	// 验证旧 Token 是否匹配存储的哈希
	valid, err := r.store.Verify(ctx, sessionID, oldHash)
	if err != nil {
		return false, false, fmt.Errorf("verify refresh token: %w", err)
	}

	if !valid {
		// 不匹配说明已经被使用过 → 重用检测，撤销用户所有 session
		return false, true, nil
	}

	// 删除旧 Token
	if err := r.store.Delete(ctx, sessionID); err != nil {
		return false, false, fmt.Errorf("delete old refresh token: %w", err)
	}

	// 存储新 Token
	if err := r.Store(ctx, sessionID, newRefreshToken); err != nil {
		return false, false, fmt.Errorf("store new refresh token: %w", err)
	}

	return true, false, nil
}

// Revoke 撤销指定 session 的 Refresh Token
func (r *Rotator) Revoke(ctx context.Context, sessionID string) error {
	return r.store.Delete(ctx, sessionID)
}

// RevokeAll 撤销用户所有 Refresh Token（强制重新登录）
func (r *Rotator) RevokeAll(ctx context.Context, userID int64) error {
	return r.store.DeleteByUser(ctx, userID)
}
