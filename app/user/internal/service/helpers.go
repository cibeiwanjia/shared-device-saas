package service

import (
	"context"

	"shared-device-saas/pkg/auth"
)

// getTenantID 从 Context 获取租户 ID
func getTenantID(ctx context.Context) int64 {
	return auth.GetTenantID(ctx)
}

// getUserID 从 Context 获取用户 ID
func getUserID(ctx context.Context) int64 {
	return auth.GetUserID(ctx)
}

// getDeviceID 从 Context 获取设备 ID
func getDeviceID(ctx context.Context) string {
	return auth.GetDeviceID(ctx)
}
