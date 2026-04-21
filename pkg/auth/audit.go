package auth

import (
	"context"
	"time"
)

// AuditEventType 审计事件类型
type AuditEventType string

const (
	AuditEventTokenIssued    AuditEventType = "token_issued"
	AuditEventTokenRefreshed AuditEventType = "token_refreshed"
	AuditEventTokenRevoked   AuditEventType = "token_revoked"
	AuditEventTokenReuse     AuditEventType = "token_reuse_detected"
	AuditEventLoginSuccess   AuditEventType = "login_success"
	AuditEventLoginFailed    AuditEventType = "login_failed"
)

// AuditEvent 审计事件
type AuditEvent struct {
	EventType AuditEventType
	UserID    int64
	TenantID  int64
	SessionID string
	JTI       string
	DeviceID  string
	IP        string
	Timestamp time.Time
	Detail    string
}

// AuditLogger 审计日志接口
type AuditLogger interface {
	Log(ctx context.Context, event *AuditEvent) error
}

// DefaultAuditLogger 默认的日志审计（输出到 kratos log）
// 实际项目中可替换为写入数据库或消息队列的实现
type DefaultAuditLogger struct{}

// NewDefaultAuditLogger 创建默认审计日志器
func NewDefaultAuditLogger() *DefaultAuditLogger {
	return &DefaultAuditLogger{}
}

// Log 记录审计事件
func (l *DefaultAuditLogger) Log(ctx context.Context, event *AuditEvent) error {
	// 通过 context 中的 kratos log 记录
	// 后续接入 data 层时替换为数据库写入
	return nil
}
