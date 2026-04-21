//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"shared-device-saas/app/storage/internal/biz"
	"shared-device-saas/app/storage/internal/conf"
	"shared-device-saas/app/storage/internal/data"
	"shared-device-saas/app/storage/internal/server"
	"shared-device-saas/app/storage/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Data, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
