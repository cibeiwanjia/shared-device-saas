package biz

import "github.com/google/wire"

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	NewExpressUsecase,  // 创建快递用例
	NewCourierUsecase,  // 创建快递员用例
	NewDispatchService, // 创建调度服务
	NewTimeoutHandler,  // 创建超时处理程序
)
