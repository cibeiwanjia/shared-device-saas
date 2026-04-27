package biz

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewDeviceUsecase,
	NewInventoryUsecase,
	NewMonitorUsecase,
	NewDeviceMQTTAuthUsecase,
)
