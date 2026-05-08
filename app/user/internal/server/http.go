package server

import (
	"encoding/json"
	stdhttp "net/http"

	v1 "shared-device-saas/api/user/v1"
	"shared-device-saas/app/user/internal/conf"
	"shared-device-saas/app/user/internal/service"
	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new an HTTP server.
// 增加 jwtMgr 和 blacklist 参数用于 JWT 认证
func NewHTTPServer(
	c *conf.Server,
	user *service.UserService,
	jwtMgr *auth.JWTManager,
	blacklist auth.Blacklist,
	logger log.Logger,
) *http.Server {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			// 选择性应用 JWT 中间件（只对需要认证的接口）
			auth.OperationSelector(
				auth.JWTMiddleware(jwtMgr, blacklist),
				// 需要认证的接口列表（普通用户操作自己 + 管理员操作他人）
				v1.OperationUserServiceGetUserMe,      // GET /v1/user/me
				v1.OperationUserServiceUpdateUserMe,   // PATCH /v1/user/me
				v1.OperationUserServiceGetUserById,    // GET /v1/user/{id}（管理员）
				v1.OperationUserServiceUpdateUserById, // PATCH /v1/user/{id}（管理员）
				v1.OperationUserServiceUpdateProfile,  // PATCH /v1/user/profile
				v1.OperationUserServiceLogout,
			),
			// 权限检查中间件（管理员操作他人需要 admin 权限）
			auth.PermissionMiddleware(),
		),
		// 添加统一返回结构编码器
		http.ResponseEncoder(responseEncoder),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	v1.RegisterUserServiceHTTPServer(srv, user)
	return srv
}

// StandardResponse 统一返回结构
type StandardResponse struct {
	Code    int         `json:"code"`    // 状态码：200成功，其他失败
	Message string      `json:"message"` // 提示信息
	Data    interface{} `json:"data"`    // 原始返回数据
}

// responseEncoder 统一返回结构编码器
// 把所有成功返回包装成 {code: 200, message: "操作成功", data: 原数据}
func responseEncoder(w stdhttp.ResponseWriter, r *stdhttp.Request, v interface{}) error {
	// 构造统一返回结构
	resp := StandardResponse{
		Code:    200,
		Message: "操作成功",
		Data:    v,
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 序列化返回
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	// 写入响应
	w.Write(data)
	return nil
}