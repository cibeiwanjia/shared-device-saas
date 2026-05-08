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
// 用于 GetUserById 和 UpdateUserById 接口的权限检查（管理员操作他人）
type PermissionChecker struct{}

// NewPermissionChecker 创建权限检查器
func NewPermissionChecker() *PermissionChecker {
	return &PermissionChecker{}
}

// CheckUserPermission 检查用户是否有权限操作指定用户
// 规则：只有 admin 可以操作他人（使用 GetUserById/UpdateUserById）
// 普通用户应该使用 GetUserMe/UpdateUserMe 操作自己
func (p *PermissionChecker) CheckUserPermission(ctx context.Context, targetUserID string) error {
	// 从 Context 获取当前用户信息
	ctxRoles := GetRoles(ctx)

	// 只有 admin 角色 → 允许操作他人
	if hasRole(ctxRoles, []string{"admin"}) {
		return nil
	}

	// 普通用户 → 不允许操作他人（应该用 GetUserMe/UpdateUserMe）
	return errors.New(403, v1.ErrorReason_PERMISSION_DENIED.String(), "权限不足，请使用 /v1/user/me 接口")
}

// PermissionMiddleware 权限检查中间件（仅针对 GetUserById 和 UpdateUserById）
// 使用 OperationSelector 确保只在特定接口上运行
// 注意：此中间件必须在 JWTMiddleware 之后使用
func PermissionMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// 检查是否是需要权限验证的 Operation
			if tr, ok := transport.FromServerContext(ctx); ok {
				operation := tr.Operation()
				// 只对 GetUserById 和 UpdateUserById 进行权限检查（管理员操作他人）
				if operation != v1.OperationUserServiceGetUserById && operation != v1.OperationUserServiceUpdateUserById {
					return handler(ctx, req)
				}
			}

			// 从请求中提取 targetUserID
			var targetUserID string
			switch r := req.(type) {
			case *v1.GetUserByIdRequest:
				targetUserID = r.Id
			case *v1.UpdateUserByIdRequest:
				targetUserID = r.Id
			default:
				return handler(ctx, req)
			}

			// 执行权限检查（只有 admin 可以操作他人）
			checker := NewPermissionChecker()
			if err := checker.CheckUserPermission(ctx, targetUserID); err != nil {
				return nil, errx.Forbidden("权限不足，请使用 /v1/user/me 接口")
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