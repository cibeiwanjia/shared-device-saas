package service

import "github.com/google/wire"

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	NewExpressService,
	NewCourierService,
	// Phase 3 新增
	NewStationService,
	NewCabinetService,
)
