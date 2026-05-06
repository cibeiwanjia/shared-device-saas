package server

import (
	pb "shared-device-saas/api/device/v1"
	"shared-device-saas/app/device/internal/conf"
	"shared-device-saas/app/device/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

func NewGRPCServer(c *conf.Server, deviceSvc *service.DeviceService, commandSvc *service.DeviceCommandService, logger log.Logger) *grpc.Server {
	var opts = []grpc.ServerOption{}
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
	pb.RegisterDeviceServiceServer(srv, deviceSvc)
	pb.RegisterDeviceCommandServiceServer(srv, commandSvc)
	return srv
}
