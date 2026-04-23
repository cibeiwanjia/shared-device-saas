package auth

import (
	"context"

	v1 "shared-device-saas/api/user/v1"
	"shared-device-saas/pkg/errx"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// PermissionChecker 权限检查器
// 用于 GetUser 和 UpdateUser 接口的权限检查
type PermissionChecker struct{}

// NewPermissionChecker 创建权限检查器
func NewPermissionChecker() *PermissionChecker {
	return &PermissionChecker{}
}

// CheckUserPermission 检查用户是否有权限操作指定用户
// 规则：admin 可以操作任何用户，普通用户只能操作自己
func (p *PermissionChecker) CheckUserPermission(ctx context.Context, targetUserID string) error {
	// 从 Context 获取当前用户信息
	ctxUserID := GetUserID(ctx) // 现在是 string 类型（MongoDB ObjectID.Hex()）
	ctxRoles := GetRoles(ctx)

	// 权限判断
	// 1. admin 角色 → 允许操作任何用户
	if hasRole(ctxRoles, []string{"admin"}) {
		return nil
	}

	// 2. 普通用户 → 只能操作自己
	if ctxUserID != targetUserID {
		return errors.New(403, v1.ErrorReason_PERMISSION_DENIED.String(), "权限不足")
	}

	return nil
}

// PermissionMiddleware 权限检查中间件（仅针对 GetUser 和 UpdateUser）
// 使用 OperationSelector 确保只在特定接口上运行
// 注意：此中间件必须在 JWTMiddleware 之后使用
func PermissionMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// 检查是否是需要权限验证的 Operation
			if tr, ok := transport.FromServerContext(ctx); ok {
				operation := tr.Operation()
				// 只对 GetUser 和 UpdateUser 进行权限检查
				if operation != v1.OperationUserServiceGetUser && operation != v1.OperationUserServiceUpdateUser {
					return handler(ctx, req)
				}
			}

			// 从请求中提取 targetUserID
			var targetUserID string
			switch r := req.(type) {
			case *v1.GetUserRequest:
				targetUserID = r.Id
			case *v1.UpdateUserRequest:
				targetUserID = r.Id
			default:
				return handler(ctx, req)
			}

			// 执行权限检查
			checker := NewPermissionChecker()
			if err := checker.CheckUserPermission(ctx, targetUserID); err != nil {
				return nil, errx.Forbidden("权限不足，无法操作该用户")
			}

			return handler(ctx, req)
		}
	}
}

// convertUserIDToString 将 int64 userID 转换为 string
// 由于 JWT 中 userID 是 ObjectID.Timestamp().Unix()，这里需要反向查询
// 实际上，这个转换在当前设计下是不对的，需要在 Service 层处理
func convertUserIDToString(userID int64) string {
	// 当前设计：JWT userID = ObjectID.Timestamp().Unix()
	// MongoDB userID = ObjectID.Hex()
	// 这两个不是同一个值，无法直接转换
	// 解决方案：在 JWT Claims 中直接存储 MongoDB 的 string ID
	// 或者：在 Service 层通过 Context 中的其他信息（如 SessionID）查询用户 ID
	return "" // 暂时返回空，由 Service 层处理
}