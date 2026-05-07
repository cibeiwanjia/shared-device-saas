package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "shared-device-saas/api/device/v1"
	"shared-device-saas/app/device/internal/biz"
	mqtt "shared-device-saas/pkg/mqtt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type DeviceCommandService struct {
	pb.UnimplementedDeviceCommandServiceServer

	inventoryUC *biz.InventoryUsecase
	deviceUC    *biz.DeviceUsecase
	mqttClient  *mqtt.Client
	log         *log.Helper
}

func NewDeviceCommandService(
	inventoryUC *biz.InventoryUsecase,
	deviceUC *biz.DeviceUsecase,
	mqttClient *mqtt.Client,
	logger log.Logger,
) *DeviceCommandService {
	return &DeviceCommandService{
		inventoryUC: inventoryUC,
		deviceUC:    deviceUC,
		mqttClient:  mqttClient,
		log:         log.NewHelper(logger),
	}
}

func (s *DeviceCommandService) OpenCell(ctx context.Context, req *pb.OpenCellRequest) (*pb.OpenCellReply, error) {
	msgID := uuid.New().String()
	topic := mqtt.BuildCommandTopic(fmt.Sprintf("%d", req.TenantId), mqtt.DeviceTypeLocker, req.DeviceSn)

	payload := map[string]interface{}{
		"v":      1,
		"ts":     time.Now().Unix(),
		"msg_id": msgID,
		"type":   "open_door",
		"data": map[string]interface{}{
			"slot_index":  req.SlotIndex,
			"timeout_sec": req.TimeoutSec,
			"operator":    req.Operator,
		},
	}

	data, _ := json.Marshal(payload)

	if s.mqttClient != nil && s.mqttClient.IsConnected() {
		if err := s.mqttClient.Publish(ctx, topic, 1, false, data); err != nil {
			s.log.Errorf("OpenCell publish failed: topic=%s err=%v", topic, err)
			return nil, fmt.Errorf("publish command failed: %w", err)
		}
		s.log.Infof("OpenCell published: topic=%s msg_id=%s", topic, msgID)
	} else {
		s.log.Warnf("MQTT not connected, OpenCell logged only: topic=%s payload=%s", topic, string(data))
	}

	return &pb.OpenCellReply{Ok: true, MsgId: msgID}, nil
}

func (s *DeviceCommandService) GetDeviceSlotStatus(ctx context.Context, req *pb.GetDeviceSlotStatusRequest) (*pb.GetDeviceSlotStatusReply, error) {
	slots, err := s.inventoryUC.GetSlotStatus(ctx, fmt.Sprintf("%d", req.TenantId), req.DeviceSn)
	if err != nil {
		return nil, fmt.Errorf("get slot status: %w", err)
	}
	if slots == nil {
		return &pb.GetDeviceSlotStatusReply{Online: false}, nil
	}

	slotStatus := make(map[int32]string)
	totalSlots := int32(len(slots))
	freeSlots := int32(0)
	for k, v := range slots {
		var idx int32
		fmt.Sscanf(k, "%d", &idx)
		slotStatus[idx] = v
		if v == "free" {
			freeSlots++
		}
	}

	return &pb.GetDeviceSlotStatusReply{
		Online:     true,
		TotalSlots: totalSlots,
		FreeSlots:  freeSlots,
		SlotStatus: slotStatus,
	}, nil
}

func (s *DeviceCommandService) ForceReleaseCell(ctx context.Context, req *pb.ForceReleaseCellRequest) (*pb.ForceReleaseCellReply, error) {
	msgID := uuid.New().String()
	topic := mqtt.BuildCommandTopic(fmt.Sprintf("%d", req.TenantId), mqtt.DeviceTypeLocker, req.DeviceSn)

	payload := map[string]interface{}{
		"v":      1,
		"ts":     time.Now().Unix(),
		"msg_id": msgID,
		"type":   "force_release",
		"data": map[string]interface{}{
			"slot_index": req.SlotIndex,
			"operator":   req.Operator,
			"reason":     req.Reason,
		},
	}

	data, _ := json.Marshal(payload)

	if s.mqttClient != nil && s.mqttClient.IsConnected() {
		if err := s.mqttClient.Publish(ctx, topic, 1, false, data); err != nil {
			s.log.Errorf("ForceRelease publish failed: topic=%s err=%v", topic, err)
			return nil, fmt.Errorf("publish command failed: %w", err)
		}
		s.log.Infof("ForceRelease published: topic=%s msg_id=%s", topic, msgID)
	} else {
		s.log.Warnf("MQTT not connected, ForceRelease logged only: topic=%s payload=%s", topic, string(data))
	}

	return &pb.ForceReleaseCellReply{Ok: true}, nil
}
