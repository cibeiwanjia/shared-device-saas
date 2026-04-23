package server

import (
	pb "shared-device-saas/api/user/v1"
	"shared-device-saas/app/user/internal/conf"
	"shared-device-saas/app/user/internal/service"
	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c *conf.Server, jwtCfg *auth.JWTConfig, svc *service.UserService, logger log.Logger) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
			auth.JWTMiddleware(jwtCfg),
		),
	}
	if c.Grpc.Network != "" {
		opts = append(opts, grpc.Network(c.Grpc.Network))
	}
	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}
	srv := grpc.NewServer(opts...)
	pb.RegisterUserServiceServer(srv, svc)
	return srv
}
