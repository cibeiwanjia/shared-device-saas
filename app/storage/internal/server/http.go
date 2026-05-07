package server

import (
	v1 "shared-device-saas/api/storage/v1"
	"shared-device-saas/app/storage/internal/conf"
	"shared-device-saas/app/storage/internal/service"
	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

func NewHTTPServer(c *conf.Server, storageSvc *service.StorageService, jwtMgr *auth.JWTManager, blacklist auth.Blacklist, logger log.Logger) *http.Server {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			auth.OperationSelector(
				auth.JWTMiddleware(jwtMgr, blacklist),
				v1.OperationStorageServiceInitiateDelivery,
				v1.OperationStorageServicePickup,
				v1.OperationStorageServiceConfirmPickup,
				v1.OperationStorageServiceInitiateShipment,
				v1.OperationStorageServiceInitiateStorage,
				v1.OperationStorageServiceRetrieveStorage,
				v1.OperationStorageServiceConfirmRetrieve,
				v1.OperationStorageServiceTempOpenCell,
				v1.OperationStorageServiceGetOrder,
				v1.OperationStorageServiceListMyOrders,
				v1.OperationStorageServiceForceOpenCell,
			),
		),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	v1.RegisterStorageServiceHTTPServer(srv, storageSvc)
	return srv
}
