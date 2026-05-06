package service

import (
	"context"
	"encoding/json"
	"fmt"

	pb "shared-device-saas/api/device/v1"
	"shared-device-saas/app/device/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type DeviceCommandService struct {
	pb.UnimplementedDeviceCommandServiceServer

	inventoryUC *biz.InventoryUsecase
	deviceUC    *biz.DeviceUsecase
	log         *log.Helper
}

func NewDeviceCommandService(
	inventoryUC *biz.InventoryUsecase,
	deviceUC *biz.DeviceUsecase,
	logger log.Logger,
) *DeviceCommandService {
	return &DeviceCommandService{
		inventoryUC: inventoryUC,
		deviceUC:    deviceUC,
		log:         log.NewHelper(logger),
	}
}

func (s *DeviceCommandService) OpenCell(ctx context.Context, req *pb.OpenCellRequest) (*pb.OpenCellReply, error) {
	msgID := uuid.New().String()
	topic := fmt.Sprintf("%d/device/locker/%s/command", req.TenantId, req.DeviceSn)

	payload := map[string]interface{}{
		"v":      1,
		"ts":     0,
		"msg_id": msgID,
		"type":   "open_door",
		"data": map[string]interface{}{
			"slot_index":  req.SlotIndex,
			"timeout_sec": req.TimeoutSec,
			"operator":    req.Operator,
		},
	}

	data, _ := json.Marshal(payload)
	s.log.Infof("OpenCell publish topic=%s payload=%s", topic, string(data))

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
	topic := fmt.Sprintf("%d/device/locker/%s/command", req.TenantId, req.DeviceSn)

	payload := map[string]interface{}{
		"v":      1,
		"ts":     0,
		"msg_id": msgID,
		"type":   "force_release",
		"data": map[string]interface{}{
			"slot_index": req.SlotIndex,
			"operator":   req.Operator,
			"reason":     req.Reason,
		},
	}

	data, _ := json.Marshal(payload)
	s.log.Infof("ForceReleaseCell publish topic=%s payload=%s", topic, string(data))

	return &pb.ForceReleaseCellReply{Ok: true}, nil
}
