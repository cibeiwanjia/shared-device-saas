package server

import (
	v1 "shared-device-saas/api/storage/v1"
	"shared-device-saas/app/storage/internal/conf"
	"shared-device-saas/app/storage/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c *conf.Server, express *service.ExpressService, courier *service.CourierService, station *service.StationService, cabinet *service.CabinetService, logger log.Logger) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
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
	v1.RegisterExpressServiceServer(srv, express)
	v1.RegisterCourierServiceServer(srv, courier)
	// Phase 3 新增
	v1.RegisterStationServiceServer(srv, station)
	v1.RegisterCabinetServiceServer(srv, cabinet)
	return srv
}