package biz

import (
	"context"
	"fmt"
	"time"

	pb "shared-device-saas/api/device/v1"

	"github.com/go-kratos/kratos/v2/log"
)

type SlotOverview struct {
	Online     bool
	TotalSlots int32
	FreeSlots  int32
	SlotStatus map[int32]string
}

type DeviceCommander struct {
	deviceClient pb.DeviceCommandServiceClient
	log          *log.Helper
}

func NewDeviceCommander(deviceClient pb.DeviceCommandServiceClient, logger log.Logger) *DeviceCommander {
	return &DeviceCommander{deviceClient: deviceClient, log: log.NewHelper(logger)}
}

func (dc *DeviceCommander) OpenCell(ctx context.Context, tenantID int64, deviceSN string, slotIndex int32, refOrderNo string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reply, err := dc.deviceClient.OpenCell(ctx, &pb.OpenCellRequest{
		TenantId:   tenantID,
		DeviceSn:   deviceSN,
		SlotIndex:  slotIndex,
		TimeoutSec: 30,
		Operator:   "system",
		RefOrderNo: refOrderNo,
	})
	if err != nil {
		return "", fmt.Errorf("gRPC open cell: %w", err)
	}
	if !reply.Ok {
		return "", fmt.Errorf("device rejected: %s", reply.Error)
	}
	return reply.MsgId, nil
}

func (dc *DeviceCommander) GetSlotStatus(ctx context.Context, tenantID int64, deviceSN string) (*SlotOverview, error) {
	reply, err := dc.deviceClient.GetDeviceSlotStatus(ctx, &pb.GetDeviceSlotStatusRequest{
		TenantId: tenantID,
		DeviceSn: deviceSN,
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC get slot status: %w", err)
	}
	return &SlotOverview{
		Online:     reply.Online,
		TotalSlots: reply.TotalSlots,
		FreeSlots:  reply.FreeSlots,
		SlotStatus: reply.SlotStatus,
	}, nil
}

func (dc *DeviceCommander) ForceReleaseCell(ctx context.Context, tenantID int64, deviceSN string, slotIndex int32, operator, reason string) error {
	reply, err := dc.deviceClient.ForceReleaseCell(ctx, &pb.ForceReleaseCellRequest{
		TenantId:  tenantID,
		DeviceSn:  deviceSN,
		SlotIndex: slotIndex,
		Operator:  operator,
		Reason:    reason,
	})
	if err != nil {
		return fmt.Errorf("gRPC force release: %w", err)
	}
	if !reply.Ok {
		return fmt.Errorf("device rejected: %s", reply.Error)
	}
	return nil
}
