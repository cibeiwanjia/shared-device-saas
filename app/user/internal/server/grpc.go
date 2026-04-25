package server

import (
<<<<<<< HEAD
	v1 "shared-device-saas/api/user/v1"
=======
	pb "shared-device-saas/api/user/v1"
>>>>>>> dev/wangqinghua
	"shared-device-saas/app/user/internal/conf"
	"shared-device-saas/app/user/internal/service"
	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

// NewGRPCServer new a gRPC server.
<<<<<<< HEAD
func NewGRPCServer(c *conf.Server, user *service.UserService, logger log.Logger) *grpc.Server {
=======
func NewGRPCServer(c *conf.Server, jwtCfg *auth.JWTConfig, svc *service.UserService, logger log.Logger) *grpc.Server {
>>>>>>> dev/wangqinghua
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
<<<<<<< HEAD
	v1.RegisterUserServiceServer(srv, user)
=======
	pb.RegisterUserServiceServer(srv, svc)
>>>>>>> dev/wangqinghua
	return srv
}