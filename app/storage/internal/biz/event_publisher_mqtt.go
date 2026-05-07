package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mqttpkg "shared-device-saas/pkg/mqtt"

	"github.com/go-kratos/kratos/v2/log"
)

type MQTTEventPublisher struct {
	client *mqttpkg.Client
	log    *log.Helper
}

func NewMQTTEventPublisher(client *mqttpkg.Client, logger log.Logger) EventPublisher {
	return &MQTTEventPublisher{client: client, log: log.NewHelper(logger)}
}

func (p *MQTTEventPublisher) PublishPickupReady(ctx context.Context, tenantID int64, orderNo string, userID int64, pickupCode string, cabinetName string) error {
	evt := StorageEvent{
		EventID:   fmt.Sprintf("pickup-%d-%d", tenantID, time.Now().UnixNano()),
		EventType: "pickup_ready",
		TenantID:  tenantID,
		OrderNo:   orderNo,
		UserID:    userID,
		Payload:   mustJSON(map[string]string{"pickup_code": pickupCode, "cabinet_name": cabinetName}),
		Timestamp: time.Now().Unix(),
	}
	return p.publish(ctx, evt)
}

func (p *MQTTEventPublisher) PublishOrderTimeout(ctx context.Context, tenantID int64, orderNo string, userID int64, fee int32) error {
	evt := StorageEvent{
		EventID:   fmt.Sprintf("timeout-%d-%d", tenantID, time.Now().UnixNano()),
		EventType: "order_timeout",
		TenantID:  tenantID,
		OrderNo:   orderNo,
		UserID:    userID,
		Payload:   fmt.Sprintf(`{"fee":%d}`, fee),
		Timestamp: time.Now().Unix(),
	}
	return p.publish(ctx, evt)
}

func (p *MQTTEventPublisher) PublishOpenTimeout(ctx context.Context, tenantID int64, deviceSN string, cellID int64, orderNo string) error {
	evt := StorageEvent{
		EventID:   fmt.Sprintf("open_timeout-%d-%d", tenantID, time.Now().UnixNano()),
		EventType: "open_timeout",
		TenantID:  tenantID,
		OrderNo:   orderNo,
		CellID:    cellID,
		Payload:   fmt.Sprintf(`{"device_sn":"%s"}`, deviceSN),
		Timestamp: time.Now().Unix(),
	}
	return p.publish(ctx, evt)
}

func (p *MQTTEventPublisher) publish(ctx context.Context, evt StorageEvent) error {
	topic := fmt.Sprintf("%d/storage/event/%s", evt.TenantID, evt.EventType)
	data, _ := json.Marshal(evt)

	if p.client != nil && p.client.IsConnected() {
		if err := p.client.Publish(ctx, topic, 1, false, data); err != nil {
			p.log.Errorf("publish event failed: topic=%s err=%v", topic, err)
			return err
		}
		p.log.Infof("event published: topic=%s event_id=%s", topic, evt.EventID)
	} else {
		p.log.Warnf("MQTT not connected, event logged: topic=%s payload=%s", topic, string(data))
	}
	return nil
}

func mustJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
