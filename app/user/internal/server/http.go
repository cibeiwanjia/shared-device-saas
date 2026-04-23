package server

import (
	"shared-device-saas/app/user/internal/conf"
	"shared-device-saas/app/user/internal/service"
	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, jwtCfg *auth.JWTConfig, svc *service.UserService, logger log.Logger) *http.Server {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			auth.JWTMiddleware(jwtCfg),
		),
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

	// 手动注册 HTTP 路由（proto 无 google.api.http 注解，待后续补充）
	// TODO: 补充 proto HTTP 注解后替换为 pb.RegisterUserServiceHTTPServer(srv, svc)
	srv.Route("/api/v1/user").POST("/login", svc.LoginHTTP)
	srv.Route("/api/v1/user").POST("/refresh-token", svc.RefreshTokenHTTP)
	srv.Route("/api/v1/user").POST("/logout", svc.LogoutHTTP)
	srv.Route("/api/v1/user").GET("/profile", svc.GetUserHTTP)
	srv.Route("/api/v1/user").PUT("/profile", svc.UpdateUserHTTP)
	srv.Route("/api/v1/user").GET("/orders", svc.ListOrdersHTTP)
	srv.Route("/api/v1/user").POST("/upload", svc.UploadImageHTTP)
	srv.Route("/api/v1/user").POST("/upload/batch", svc.BatchUploadImagesHTTP)
	srv.Route("/api/v1/user").POST("/upload/signed-url", svc.GetSignedURLHTTP)
	srv.Route("/api/v1/user").GET("/wallet", svc.GetWalletHTTP)
	srv.Route("/api/v1/user").GET("/wallet/transactions", svc.ListTransactionsHTTP)
	srv.Route("/api/v1/user").POST("/recharge", svc.CreateRechargeHTTP)
	srv.Route("/api/v1/user").POST("/recharge/callback", svc.RechargeCallbackHTTP)
	srv.Route("/api/v1/user").GET("/recharges", svc.ListRechargesHTTP)

	return srv
}
