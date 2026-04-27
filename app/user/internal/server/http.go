package server

import (
	_ "embed"
	"encoding/json"
	stdhttp "net/http"

	v1 "shared-device-saas/api/user/v1"
	"shared-device-saas/app/user/internal/conf"
	"shared-device-saas/app/user/internal/service"
	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
	swaggerUI "github.com/tx7do/kratos-swagger-ui"
)

//go:embed openapi.yaml
var openapiDoc []byte

// NewHTTPServer 创建 HTTP Server
// 使用 OperationSelector 选择性应用 JWT 中间件（只对需要认证的接口）
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
				// 需要认证的接口列表
				v1.OperationUserServiceGetMe,
				v1.OperationUserServiceGetUser,
				v1.OperationUserServiceUpdateUser,
				v1.OperationUserServiceLogout,
				// 订单接口（支付回调不加 JWT）
				v1.OperationUserServiceCreateOrder,
				v1.OperationUserServiceGetOrder,
				v1.OperationUserServiceListOrders,
				v1.OperationUserServiceCancelOrder,
				v1.OperationUserServicePayOrder,
				// 站点接口
				v1.OperationUserServiceSearchNearbyStations,
				v1.OperationUserServiceGetStation,
			),
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

	// 使用 proto 自动注册（HEAD 方式，更规范）
	v1.RegisterUserServiceHTTPServer(srv, user)

	// 提供 openapi.yaml 静态文件服务（必须在 Swagger UI 之前注册，否则 HandlePrefix 会吞掉子路径）
	srv.Route("/docs").GET("/openapi.yaml", func(ctx http.Context) error {
		ctx.Response().Header().Set("Content-Type", "application/yaml")
		ctx.Response().Write(openapiDoc)
		return nil
	})

	// Swagger UI（内嵌静态资源，访问 /docs/ 即可）
	swaggerUI.RegisterSwaggerUIServer(srv, "Shared Device SaaS API", "/docs/openapi.yaml", "/docs/")

	// 手动注册额外的 HTTP 路由（proto 中未定义 google.api.http 注解的业务接口）
	srv.Route("/api/v1/user").POST("/upload", user.UploadImageHTTP)
	srv.Route("/api/v1/user").POST("/upload/batch", user.BatchUploadImagesHTTP)
	srv.Route("/api/v1/user").POST("/upload/signed-url", user.GetSignedURLHTTP)
	srv.Route("/api/v1/user").GET("/wallet", user.GetWalletHTTP)
	srv.Route("/api/v1/user").GET("/wallet/transactions", user.ListTransactionsHTTP)
	srv.Route("/api/v1/user").POST("/recharge", user.CreateRechargeHTTP)
	srv.Route("/api/v1/user").POST("/recharge/callback", user.RechargeCallbackHTTP)
	srv.Route("/api/v1/user").GET("/recharges", user.ListRechargesHTTP)

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
	resp := StandardResponse{
		Code:    200,
		Message: "操作成功",
		Data:    v,
	}

	w.Header().Set("Content-Type", "application/json")

	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	w.Write(data)
	return nil
}
