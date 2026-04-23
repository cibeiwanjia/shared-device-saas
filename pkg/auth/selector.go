package auth

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// OperationSelector 选择性应用中间件（根据 Operation 名称）
func OperationSelector(m middleware.Middleware, operations ...string) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// 从 transport context 获取 operation
			if tr, ok := transport.FromServerContext(ctx); ok {
				operation := tr.Operation()
				// 检查是否在允许的 operations 列表中
				for _, op := range operations {
					if operation == op {
						// 应用中间件
						return m(handler)(ctx, req)
					}
				}
			}
			// 不在列表中，直接跳过中间件
			return handler(ctx, req)
		}
	}
}