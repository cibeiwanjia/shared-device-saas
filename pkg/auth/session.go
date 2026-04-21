package auth

import (
	"context"
	"time"
)

// SessionInfo 会话信息
type SessionInfo struct {
	SessionID  string
	DeviceID   string
	DeviceName string
	IP         string
	UserAgent  string
	LoginAt    time.Time
}

// SessionStore 会话存储接口
type SessionStore interface {
	// Save 保存会话信息
	Save(ctx context.Context, userID int64, session *SessionInfo, ttl time.Duration) error
	// List 获取用户所有活跃会话
	List(ctx context.Context, userID int64) ([]*SessionInfo, error)
	// Get 获取指定会话
	Get(ctx context.Context, sessionID string) (*SessionInfo, error)
	// Revoke 撤销指定会话
	Revoke(ctx context.Context, sessionID string) error
	// RevokeAll 撤销用户所有会话
	RevokeAll(ctx context.Context, userID int64) error
}

// SessionManager 会话管理器
type SessionManager struct {
	store      SessionStore
	sessionTTL time.Duration
}

// NewSessionManager 创建会话管理器
func NewSessionManager(store SessionStore, sessionTTL time.Duration) *SessionManager {
	return &SessionManager{store: store, sessionTTL: sessionTTL}
}

// CreateSession 创建新会话
func (sm *SessionManager) CreateSession(ctx context.Context, userID int64, info *SessionInfo) error {
	return sm.store.Save(ctx, userID, info, sm.sessionTTL)
}

// ListSessions 获取用户所有活跃会话
func (sm *SessionManager) ListSessions(ctx context.Context, userID int64) ([]*SessionInfo, error) {
	return sm.store.List(ctx, userID)
}

// RevokeSession 撤销指定会话（踢设备）
func (sm *SessionManager) RevokeSession(ctx context.Context, sessionID string) error {
	return sm.store.Revoke(ctx, sessionID)
}

// RevokeAllSessions 撤销用户所有会话（强制全部设备重新登录）
func (sm *SessionManager) RevokeAllSessions(ctx context.Context, userID int64) error {
	return sm.store.RevokeAll(ctx, userID)
}
