package server

import (
	"context"
	"encoding/json"
	"fmt"

	"shared-device-saas/app/device/internal/biz"
	paho "github.com/eclipse/paho.golang/paho"
	"github.com/go-kratos/kratos/v2/log"
)

type MQTTHandler struct {
	monitorUC   *biz.MonitorUsecase
	inventoryUC *biz.InventoryUsecase
	log         *log.Helper
}

func NewMQTTHandler(monitorUC *biz.MonitorUsecase, inventoryUC *biz.InventoryUsecase, logger log.Logger) *MQTTHandler {
	return &MQTTHandler{monitorUC: monitorUC, inventoryUC: inventoryUC, log: log.NewHelper(logger)}
}

func (h *MQTTHandler) HandleDeviceStatus(pub *paho.Publish) {
	var status struct {
		DeviceID     string `json:"device_id"`
		DeviceType   string `json:"device_type"`
		TenantID     string `json:"tenant_id"`
		Status       int32  `json:"status"`
		BatteryLevel uint8  `json:"battery_level"`
	}
	if err := json.Unmarshal(pub.Payload, &status); err != nil {
		h.log.Errorf("parse device status: %v", err)
		return
	}

	ds := &biz.DeviceStatus{
		DeviceID:     status.DeviceID,
		DeviceType:   status.DeviceType,
		TenantID:     status.TenantID,
		Status:       status.Status,
		BatteryLevel: status.BatteryLevel,
	}
	ctx := context.Background()
	if err := h.inventoryUC.CacheDeviceStatus(ctx, status.TenantID, status.DeviceID, ds); err != nil {
		h.log.Errorf("cache device status: %v", err)
	}
}

func (h *MQTTHandler) HandleClientConnected(pub *paho.Publish) {
	var event struct {
		ClientID string `json:"clientid"`
		IP       string `json:"ip_address"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(pub.Payload, &event); err != nil {
		h.log.Errorf("parse connected event: %v", err)
		return
	}
	h.log.Infof("device connected: clientID=%s, ip=%s", event.ClientID, event.IP)
}

func (h *MQTTHandler) HandleClientDisconnected(pub *paho.Publish) {
	var event struct {
		ClientID   string `json:"clientid"`
		Reason     string `json:"reason"`
		ReasonCode int    `json:"reason_code"`
	}
	if err := json.Unmarshal(pub.Payload, &event); err != nil {
		h.log.Errorf("parse disconnected event: %v", err)
		return
	}
	h.log.Infof("device disconnected: clientID=%s, reason=%s", event.ClientID, event.Reason)
}

func (h *MQTTHandler) RegisterRoutes(mqttClient interface{ RegisterHandler(topic string, handler func(p *paho.Publish)) }) {
	mqttClient.RegisterHandler("+/device/+/+/status", h.HandleDeviceStatus)
	mqttClient.RegisterHandler("$events/client_connected", h.HandleClientConnected)
	mqttClient.RegisterHandler("$events/client_disconnected", h.HandleClientDisconnected)
	h.log.Info(fmt.Sprintf("MQTT routes registered"))
}
