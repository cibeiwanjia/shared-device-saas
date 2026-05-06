package biz

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewOrderFSM,
	NewPricingEngine,
	NewPickupCodeManager,
	NewCellAllocator,
	NewDeviceCommander,
	NewTimeoutManager,
	NewTimeoutHandler,
	NewDeliveryInUsecase,
	NewDeliveryOutUsecase,
	NewStorageUsecase,
)
