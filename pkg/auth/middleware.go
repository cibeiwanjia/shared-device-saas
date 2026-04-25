package auth

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// JWTMiddleware 创建 Kratos JWT 中间件
// 从 HTTP Header Authorization: Bearer <token> 或 gRPC metadata 解析 token
func JWTMiddleware(cfg *JWTConfig) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// 从 transport 层获取 token
			tr, ok := transport.FromServerContext(ctx)
			if ok {
				var tokenStr string
				// 从 Header 获取 Authorization
				authHeader := tr.RequestHeader().Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
				}

				if tokenStr != "" {
					claims, err := ParseToken(cfg, tokenStr)
					if err == nil {
						ctx = SetUserID(ctx, claims.UserID)
						ctx = SetTenantID(ctx, claims.TenantID)
						ctx = SetDeviceID(ctx, claims.DeviceID)
					}
					// Token 无效时不拦截，仅不注入上下文（支持匿名访问）
				}
			}
			return handler(ctx, req)
		}
	}
}
