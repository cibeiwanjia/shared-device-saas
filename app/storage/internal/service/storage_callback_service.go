package service

import (
	"context"
	"time"

	pb "shared-device-saas/api/storage/v1"
	"shared-device-saas/app/storage/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type StorageCallbackService struct {
	pb.UnimplementedStorageCallbackServiceServer

	deliveryIn  *biz.DeliveryInUsecase
	deliveryOut *biz.DeliveryOutUsecase
	storageUc   *biz.StorageUsecase
	cellRepo    biz.CellRepo
	orderRepo   biz.OrderRepo
	timeoutMgr  *biz.TimeoutManager
	log         *log.Helper
}

func NewStorageCallbackService(
	deliveryIn *biz.DeliveryInUsecase,
	deliveryOut *biz.DeliveryOutUsecase,
	storageUc *biz.StorageUsecase,
	cellRepo biz.CellRepo,
	orderRepo biz.OrderRepo,
	timeoutMgr *biz.TimeoutManager,
	logger log.Logger,
) *StorageCallbackService {
	return &StorageCallbackService{
		deliveryIn: deliveryIn, deliveryOut: deliveryOut, storageUc: storageUc,
		cellRepo: cellRepo, orderRepo: orderRepo, timeoutMgr: timeoutMgr,
		log: log.NewHelper(logger),
	}
}

func (s *StorageCallbackService) ReportDoorEvent(ctx context.Context, req *pb.ReportDoorEventRequest) (*pb.ReportDoorEventReply, error) {
	if !req.DoorClosed {
		cell, err := s.cellRepo.FindByDeviceAndSlot(ctx, req.DeviceSn, req.SlotIndex)
		if err != nil || cell == nil {
			s.log.Warnf("door_opened: cell not found device=%s slot=%d", req.DeviceSn, req.SlotIndex)
			return &pb.ReportDoorEventReply{Ok: true}, nil
		}
		s.cellRepo.UpdateOpenedAt(ctx, cell.ID, time.Now())
		s.timeoutMgr.RegisterOpenTimeout(cell.ID, 5*time.Minute)
		return &pb.ReportDoorEventReply{Ok: true}, nil
	}

	cell, err := s.cellRepo.FindByDeviceAndSlot(ctx, req.DeviceSn, req.SlotIndex)
	if err != nil || cell == nil {
		s.log.Warnf("door_closed: cell not found device=%s slot=%d", req.DeviceSn, req.SlotIndex)
		return &pb.ReportDoorEventReply{Ok: true}, nil
	}

	if cell.PendingAction == nil {
		s.log.Warnf("unexpected door_closed: device=%s slot=%d", req.DeviceSn, req.SlotIndex)
		return &pb.ReportDoorEventReply{Ok: true}, nil
	}

	if cell.CurrentOrderID == nil {
		s.log.Warnf("door_closed: no current order cellID=%d", cell.ID)
		return &pb.ReportDoorEventReply{Ok: true}, nil
	}

	order, err := s.orderRepo.GetByID(ctx, *cell.CurrentOrderID)
	if err != nil || order == nil {
		s.log.Errorf("door_closed: order not found orderID=%d", *cell.CurrentOrderID)
		return &pb.ReportDoorEventReply{Ok: true}, nil
	}

	switch *cell.PendingAction {
	case biz.PendingActionDeposit:
		if order.OrderType == biz.OrderTypeDeliveryIn {
			return &pb.ReportDoorEventReply{Ok: true}, s.deliveryIn.HandleDepositClosed(ctx, order, cell)
		}
		if order.OrderType == biz.OrderTypeDeliveryOut {
			return &pb.ReportDoorEventReply{Ok: true}, s.deliveryOut.HandleDepositClosed(ctx, order, cell)
		}
		return &pb.ReportDoorEventReply{Ok: true}, s.storageUc.HandleDepositClosed(ctx, order, cell)
	case biz.PendingActionPickup:
		return &pb.ReportDoorEventReply{Ok: true}, s.deliveryIn.HandlePickupClosed(ctx, order, cell)
	case biz.PendingActionTempOpen:
		return &pb.ReportDoorEventReply{Ok: true}, s.storageUc.HandleTempOpenClosed(ctx, order, cell)
	case biz.PendingActionStore:
		return &pb.ReportDoorEventReply{Ok: true}, s.storageUc.HandleDepositClosed(ctx, order, cell)
	}

	return &pb.ReportDoorEventReply{Ok: true}, nil
}
