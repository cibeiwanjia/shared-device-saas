package auth

import (
	"context"
	"fmt"
	"strings"

	"shared-device-saas/pkg/errx"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// JWTMiddleware JWT 认证中间件（严格模式）
// 从 HTTP Header Authorization: Bearer <token> 解析并验证 token
// 支持黑名单检查和 RBAC 角色验证
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
			ctx = SetUserID(ctx, claims.UserID)
			ctx = SetTenantID(ctx, claims.TenantID)
			ctx = SetSessionID(ctx, claims.SessionID)
			ctx = SetDeviceID(ctx, claims.DeviceID)
			ctx = SetRoles(ctx, claims.Roles)

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
