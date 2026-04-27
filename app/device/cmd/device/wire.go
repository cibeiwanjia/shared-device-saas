//go:build wireinject
// +build wireinject

package main

import (
	"shared-device-saas/app/device/internal/biz"
	"shared-device-saas/app/device/internal/conf"
	"shared-device-saas/app/device/internal/data"
	"shared-device-saas/app/device/internal/server"
	"shared-device-saas/app/device/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

func wireApp(*conf.Server, *conf.Data, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
