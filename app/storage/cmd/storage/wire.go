//go:build wireinject
// +build wireinject

package main

import (
	"shared-device-saas/app/storage/internal/biz"
	"shared-device-saas/app/storage/internal/conf"
	"shared-device-saas/app/storage/internal/data"
	"shared-device-saas/app/storage/internal/server"
	"shared-device-saas/app/storage/internal/service"

	pb "shared-device-saas/api/device/v1"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func wireApp(*conf.Server, *conf.Data, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(
		server.ProviderSet,
		data.ProviderSet,
		biz.ProviderSet,
		service.ProviderSet,
		newApp,
		NewDeviceCommandClient,
	))
}

func NewDeviceCommandClient(logger log.Logger) (pb.DeviceCommandServiceClient, func(), error) {
	conn, err := grpc.NewClient("localhost:9001",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { conn.Close() }
	return pb.NewDeviceCommandServiceClient(conn), cleanup, nil
}
