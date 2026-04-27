package server

import (
	"encoding/json"
	stdhttp "net/http"

	pb "shared-device-saas/api/device/v1"
	"shared-device-saas/app/device/internal/conf"
	"shared-device-saas/app/device/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

func NewHTTPServer(c *conf.Server, deviceSvc *service.DeviceService, logger log.Logger) *http.Server {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
		),
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
	pb.RegisterDeviceServiceHTTPServer(srv, deviceSvc)
	return srv
}

type standardResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func responseEncoder(w stdhttp.ResponseWriter, r *stdhttp.Request, v interface{}) error {
	resp := standardResponse{Code: 200, Message: "操作成功", Data: v}
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	w.Write(data)
	return nil
}
