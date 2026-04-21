package auth

import (
	"context"
	"fmt"
	"strings"

	"shared-device-saas/pkg/errx"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

type contextKey string

const (
	contextKeyUserID    contextKey = "user_id"
	contextKeyTenantID  contextKey = "tenant_id"
	contextKeySessionID contextKey = "session_id"
	contextKeyDeviceID  contextKey = "device_id"
	contextKeyRoles     contextKey = "roles"
)

// JWTMiddleware JWT 认证中间件
func JWTMiddleware(jwtMgr *JWTManager, blacklist Blacklist, requiredRoles ...string) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// 从 Header 提取 Token
			tokenStr, err := extractToken(ctx)
			if err != nil {
				return nil, errx.Unauthorized("missing or invalid token")
			}

			// 解析 Token
			claims, err := jwtMgr.ParseAccessToken(tokenStr)
			if err != nil {
				return nil, errx.Unauthorized("invalid token")
			}

			// 检查黑名单
			blacklisted, err := blacklist.IsBlacklisted(ctx, claims.ID)
			if err != nil {
				return nil, errx.Internal("token check failed")
			}
			if blacklisted {
				return nil, errx.Unauthorized("token revoked")
			}

			// RBAC 角色验证
			if len(requiredRoles) > 0 {
				if !hasRole(claims.Roles, requiredRoles) {
					return nil, errx.Forbidden("insufficient permissions")
				}
			}

			// 注入 Context
			ctx = context.WithValue(ctx, contextKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, contextKeyTenantID, claims.TenantID)
			ctx = context.WithValue(ctx, contextKeySessionID, claims.SessionID)
			ctx = context.WithValue(ctx, contextKeyDeviceID, claims.DeviceID)
			ctx = context.WithValue(ctx, contextKeyRoles, claims.Roles)

			return handler(ctx, req)
		}
	}
}

// extractToken 从 HTTP/gRPC Header 提取 Bearer Token
func extractToken(ctx context.Context) (string, error) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		authHeader := tr.RequestHeader().Get("Authorization")
		if authHeader == "" {
			return "", fmt.Errorf("authorization header empty")
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return "", fmt.Errorf("invalid authorization format")
		}
		return parts[1], nil
	}
	return "", fmt.Errorf("transport not found in context")
}

// hasRole 检查用户是否拥有所需角色
func hasRole(userRoles, requiredRoles []string) bool {
	roleSet := make(map[string]struct{}, len(userRoles))
	for _, r := range userRoles {
		roleSet[r] = struct{}{}
	}
	for _, required := range requiredRoles {
		if _, ok := roleSet[required]; ok {
			return true
		}
	}
	return false
}

// --- Context 取值工具函数 ---

// GetUserID 从 Context 获取用户 ID
func GetUserID(ctx context.Context) int64 {
	if v, ok := ctx.Value(contextKeyUserID).(int64); ok {
		return v
	}
	return 0
}

// GetTenantID 从 Context 获取租户 ID
func GetTenantID(ctx context.Context) int64 {
	if v, ok := ctx.Value(contextKeyTenantID).(int64); ok {
		return v
	}
	return 0
}

// GetSessionID 从 Context 获取会话 ID
func GetSessionID(ctx context.Context) string {
	if v, ok := ctx.Value(contextKeySessionID).(string); ok {
		return v
	}
	return ""
}

// GetDeviceID 从 Context 获取设备 ID
func GetDeviceID(ctx context.Context) string {
	if v, ok := ctx.Value(contextKeyDeviceID).(string); ok {
		return v
	}
	return ""
}

// GetRoles 从 Context 获取用户角色
func GetRoles(ctx context.Context) []string {
	if v, ok := ctx.Value(contextKeyRoles).([]string); ok {
		return v
	}
	return nil
}
