package server

import (
	"encoding/json"
	stdhttp "net/http"

	v1 "shared-device-saas/api/storage/v1"
	"shared-device-saas/app/storage/internal/conf"
	"shared-device-saas/app/storage/internal/service"
	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(
	c *conf.Server,
	express *service.ExpressService,
	courier *service.CourierService,
	station *service.StationService,
	cabinet *service.CabinetService,
	jwtMgr *auth.JWTManager,
	blacklist auth.Blacklist,
	logger log.Logger,
) *http.Server {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			// 选择性应用 JWT 中间件（只对需要认证的接口）
			auth.OperationSelector(
				auth.JWTMiddleware(jwtMgr, blacklist),
				// 所有快递接口都需要认证
				v1.OperationExpressServiceCreateExpress,
				v1.OperationExpressServiceListExpress,
				v1.OperationExpressServiceGetExpress,
				v1.OperationExpressServicePickupExpress,
				v1.OperationExpressServiceCancelExpress,
				// Phase 2 新增快递接口
				v1.OperationExpressServiceVerifyPickup,
				v1.OperationExpressServiceGetCourierPendingOrders,
				v1.OperationExpressServiceCancelTimeoutOrder,
				v1.OperationExpressServiceCheckCoverage,
				v1.OperationExpressServiceManualAssignOrder,
				// Phase 3 新增快递接口
				v1.OperationExpressServiceUpdateOrderStatus,
				v1.OperationExpressServiceDeliverToStation,
				v1.OperationExpressServicePickupFromStation,
				v1.OperationExpressServiceDeliverToCabinet,
				v1.OperationExpressServicePickupFromCabinet,
				v1.OperationExpressServiceGetTrace,
				v1.OperationExpressServiceAppendTrace,
				// 所有快递员接口都需要认证
				v1.OperationCourierServiceApplyCourier,
				v1.OperationCourierServiceGetCourierInfo,
				v1.OperationCourierServiceGetCourierList,
				v1.OperationCourierServiceApproveCourier,
				v1.OperationCourierServiceAssignZone,
				v1.OperationCourierServiceCreateZone,
				v1.OperationCourierServiceGetZoneList,
				// 驿站接口（管理员需认证）
				v1.OperationStationServiceCreateStation,
				v1.OperationStationServiceUpdateStation,
				// 快递柜接口（管理员需认证）
				v1.OperationCabinetServiceCreateCabinet,
				v1.OperationCabinetServiceUpdateCabinet,
				v1.OperationCabinetServiceReleaseGrid,
			),
		),
		// 添加统一返回结构编码器
		http.ResponseEncoder(responseEncoder),
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
	v1.RegisterExpressServiceHTTPServer(srv, express)
	v1.RegisterCourierServiceHTTPServer(srv, courier)
	// Phase 3 新增
	v1.RegisterStationServiceHTTPServer(srv, station)
	v1.RegisterCabinetServiceHTTPServer(srv, cabinet)
	return srv
}

// StandardResponse 统一返回结构
type StandardResponse struct {
	Code    int         `json:"code"`    // 状态码：200成功，其他失败
	Message string      `json:"message"` // 提示信息
	Data    interface{} `json:"data"`    // 原始返回数据
}

// responseEncoder 统一返回结构编码器
func responseEncoder(w stdhttp.ResponseWriter, r *stdhttp.Request, v interface{}) error {
	// 构造统一返回结构
	resp := StandardResponse{
		Code:    200,
		Message: "操作成功",
		Data:    v,
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 序列化返回
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	// 写入响应
	w.Write(data)
	return nil
}