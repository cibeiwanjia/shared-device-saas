package auth

import (
	"context"
	"strconv"
)

type contextKey string

const (
	tenantIDKey  contextKey = "tenant_id"
	userIDKey    contextKey = "user_id"
	sessionIDKey contextKey = "session_id"
	deviceIDKey  contextKey = "device_id"
	rolesKey     contextKey = "roles"
)

// GetTenantID 从 Context 获取租户 ID
func GetTenantID(ctx context.Context) int64 {
	if v, ok := ctx.Value(tenantIDKey).(int64); ok {
		return v
	}
	return 0
}

// SetTenantID 向 Context 设置租户 ID
func SetTenantID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, tenantIDKey, id)
}

// GetUserID 从 Context 获取用户 ID（MongoDB ObjectID.Hex() 字符串）
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	return ""
}

// GetUserIDInt64 从 Context 获取用户 ID 并转为 int64（用于钱包/订单等模块）
func GetUserIDInt64(ctx context.Context) int64 {
	s := GetUserID(ctx)
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// SetUserID 向 Context 设置用户 ID
func SetUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// GetSessionID 从 Context 获取会话 ID
func GetSessionID(ctx context.Context) string {
	if v, ok := ctx.Value(sessionIDKey).(string); ok {
		return v
	}
	return ""
}

// SetSessionID 向 Context 设置会话 ID
func SetSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey, id)
}

// GetDeviceID 从 Context 获取设备 ID
func GetDeviceID(ctx context.Context) string {
	if v, ok := ctx.Value(deviceIDKey).(string); ok {
		return v
	}
	return ""
}

// SetDeviceID 向 Context 设置设备 ID
func SetDeviceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, deviceIDKey, id)
}

// GetRoles 从 Context 获取用户角色
func GetRoles(ctx context.Context) []string {
	if v, ok := ctx.Value(rolesKey).([]string); ok {
		return v
	}
	return nil
}

// SetRoles 向 Context 设置用户角色
func SetRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, rolesKey, roles)
}
