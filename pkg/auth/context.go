package auth

import "context"

type contextKey string

const (
	tenantIDKey contextKey = "tenant_id"
	userIDKey   contextKey = "user_id"
	deviceIDKey contextKey = "device_id"
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

// GetUserID 从 Context 获取用户 ID
func GetUserID(ctx context.Context) int64 {
	if v, ok := ctx.Value(userIDKey).(int64); ok {
		return v
	}
	return 0
}

// SetUserID 向 Context 设置用户 ID
func SetUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, userIDKey, id)
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
