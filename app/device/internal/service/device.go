package service

import (
	"context"
	"fmt"
	"time"

	pb "shared-device-saas/api/device/v1"
	"shared-device-saas/app/device/internal/biz"
)

type DeviceService struct {
	pb.UnimplementedDeviceServiceServer

	deviceUC *biz.DeviceUsecase
	inventoryUC *biz.InventoryUsecase
	monitorUC *biz.MonitorUsecase
	authUC *biz.DeviceMQTTAuthUsecase
}

func NewDeviceService(
	deviceUC *biz.DeviceUsecase,
	inventoryUC *biz.InventoryUsecase,
	monitorUC *biz.MonitorUsecase,
	authUC *biz.DeviceMQTTAuthUsecase,
) *DeviceService {
	return &DeviceService{
		deviceUC:    deviceUC,
		inventoryUC: inventoryUC,
		monitorUC:   monitorUC,
		authUC:      authUC,
	}
}

func (s *DeviceService) RegisterDevice(ctx context.Context, req *pb.RegisterDeviceRequest) (*pb.RegisterDeviceReply, error) {
	d, err := s.deviceUC.RegisterDevice(ctx, &biz.Device{
		TenantID:     req.TenantId,
		DeviceType:   req.DeviceType,
		DeviceSN:     req.DeviceSn,
		Name:         req.Name,
		LocationLat:  req.LocationLat,
		LocationLng:  req.LocationLng,
		LocationName: req.LocationName,
		StationID:    req.StationId,
	})
	if err != nil {
		return nil, err
	}
	return &pb.RegisterDeviceReply{Id: d.ID, DeviceSn: d.DeviceSN}, nil
}

func (s *DeviceService) GetDevice(ctx context.Context, req *pb.GetDeviceRequest) (*pb.GetDeviceReply, error) {
	d, err := s.deviceUC.GetDevice(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return deviceToReply(d), nil
}

func (s *DeviceService) ListDevices(ctx context.Context, req *pb.ListDevicesRequest) (*pb.ListDevicesReply, error) {
	devices, total, err := s.deviceUC.ListDevices(ctx, req.TenantId, req.DeviceType, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*pb.GetDeviceReply, 0, len(devices))
	for _, d := range devices {
		items = append(items, deviceToReply(d))
	}
	return &pb.ListDevicesReply{Items: items, Total: total}, nil
}

func (s *DeviceService) GetDeviceStatus(ctx context.Context, req *pb.GetDeviceStatusRequest) (*pb.GetDeviceStatusReply, error) {
	d, err := s.deviceUC.GetDevice(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	online := d.Status >= 1
	lastOnline := formatTime(d.LastOnlineAt)
	lastOffline := formatTime(d.LastOfflineAt)
	return &pb.GetDeviceStatusReply{
		Id:            d.ID,
		Status:        d.Status,
		BatteryLevel:  uint32(d.BatteryLevel),
		Online:        online,
		LastOnlineAt:  lastOnline,
		LastOfflineAt: lastOffline,
	}, nil
}

func (s *DeviceService) ListConnectionEvents(ctx context.Context, req *pb.ListConnectionEventsRequest) (*pb.ListConnectionEventsReply, error) {
	var start, end time.Time
	if req.StartTime != "" {
		start, _ = time.Parse(time.RFC3339, req.StartTime)
	}
	if req.EndTime != "" {
		end, _ = time.Parse(time.RFC3339, req.EndTime)
	}
	events, total, err := s.monitorUC.ListEvents(ctx, req.DeviceId, req.EventType, start, end, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*pb.ConnectionEventItem, 0, len(events))
	for _, e := range events {
		items = append(items, &pb.ConnectionEventItem{
			Id:         e.ID,
			EventType:  e.EventType,
			ReasonCode: int32(e.ReasonCode),
			IpAddress:  e.IPAddress,
			ClientId:   e.ClientID,
			OccurredAt: formatTime(e.OccurredAt),
		})
	}
	return &pb.ListConnectionEventsReply{Items: items, Total: total}, nil
}

func (s *DeviceService) GenerateDeviceToken(ctx context.Context, req *pb.GenerateDeviceTokenRequest) (*pb.GenerateDeviceTokenReply, error) {
	token, err := s.authUC.GenerateDeviceToken(ctx, req.DeviceId, req.TenantId, req.DeviceType)
	if err != nil {
		return nil, err
	}
	return &pb.GenerateDeviceTokenReply{Token: token, ExpiresIn: 86400}, nil
}

func deviceToReply(d *biz.Device) *pb.GetDeviceReply {
	return &pb.GetDeviceReply{
		Id:            d.ID,
		TenantId:      d.TenantID,
		DeviceType:    d.DeviceType,
		DeviceSn:      d.DeviceSN,
		Name:          d.Name,
		Status:        d.Status,
		LocationLat:   d.LocationLat,
		LocationLng:   d.LocationLng,
		LocationName:  d.LocationName,
		StationId:     d.StationID,
		BatteryLevel:  uint32(d.BatteryLevel),
		LastOnlineAt:  formatTime(d.LastOnlineAt),
		LastOfflineAt: formatTime(d.LastOfflineAt),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return fmt.Sprintf("%d", t.Unix())
}
