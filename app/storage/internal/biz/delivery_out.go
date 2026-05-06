package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type DeliveryOutUsecase struct {
	fsm         *OrderFSM
	allocator   *CellAllocator
	commander   *DeviceCommander
	orderRepo   OrderRepo
	cellRepo    CellRepo
	cabinetRepo CabinetRepo
	log         *log.Helper
}

func NewDeliveryOutUsecase(
	fsm *OrderFSM, allocator *CellAllocator, commander *DeviceCommander,
	orderRepo OrderRepo, cellRepo CellRepo, cabinetRepo CabinetRepo, logger log.Logger,
) *DeliveryOutUsecase {
	return &DeliveryOutUsecase{
		fsm: fsm, allocator: allocator, commander: commander,
		orderRepo: orderRepo, cellRepo: cellRepo, cabinetRepo: cabinetRepo,
		log: log.NewHelper(logger),
	}
}

func (uc *DeliveryOutUsecase) InitiateShipment(ctx context.Context, tenantID, userID, cabinetID int64, cellType int32) (*StorageOrder, error) {
	cell, err := uc.allocator.AllocateCell(ctx, cabinetID, cellType)
	if err != nil {
		return nil, fmt.Errorf("allocate cell: %w", err)
	}

	cabinet, _ := uc.cabinetRepo.FindByID(ctx, cabinetID)
	order := &StorageOrder{
		TenantID:  tenantID,
		OrderType: OrderTypeDeliveryOut,
		Status:    OrderStatusPending,
		UserID:    userID,
		CabinetID: cabinetID,
		CellID:    cell.ID,
		DeviceSN:  cabinet.DeviceSN,
		SlotIndex: cell.SlotIndex,
	}
	order, err = uc.orderRepo.Create(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	if err := uc.allocator.MarkOpening(ctx, cell.ID, PendingActionDeposit, order.ID); err != nil {
		return nil, err
	}

	_, err = uc.commander.OpenCell(ctx, tenantID, cabinet.DeviceSN, cell.SlotIndex, order.OrderNo)
	if err != nil {
		_ = uc.allocator.ReleaseCell(ctx, cell.ID)
		return nil, fmt.Errorf("open cell: %w", err)
	}

	return order, nil
}

func (uc *DeliveryOutUsecase) HandleDepositClosed(ctx context.Context, order *StorageOrder, cell *Cell) error {
	fromStatus := order.Status
	if err := uc.fsm.Transition(order.OrderType, order, OrderStatusDeposited); err != nil {
		return err
	}
	affected, err := uc.orderRepo.UpdateStatusCAS(ctx, order.ID, fromStatus, OrderStatusDeposited)
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil
	}
	_ = uc.allocator.ConfirmOccupied(ctx, cell.ID)
	_ = uc.allocator.ClearPendingAction(ctx, cell.ID)
	now := time.Now()
	order.DepositedAt = &now
	return nil
}

func (uc *DeliveryOutUsecase) OnOrderTimeout(_ context.Context, _ *StorageOrder) error {
	return nil
}
