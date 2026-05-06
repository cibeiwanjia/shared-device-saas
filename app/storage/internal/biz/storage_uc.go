package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type StorageUsecase struct {
	fsm         *OrderFSM
	allocator   *CellAllocator
	pricing     *PricingEngine
	commander   *DeviceCommander
	publisher   EventPublisher
	timeoutMgr  *TimeoutManager
	orderRepo   OrderRepo
	cellRepo    CellRepo
	cabinetRepo CabinetRepo
	log         *log.Helper
}

func NewStorageUsecase(
	fsm *OrderFSM, allocator *CellAllocator, pricing *PricingEngine,
	commander *DeviceCommander, publisher EventPublisher, timeoutMgr *TimeoutManager,
	orderRepo OrderRepo, cellRepo CellRepo, cabinetRepo CabinetRepo, logger log.Logger,
) *StorageUsecase {
	return &StorageUsecase{
		fsm: fsm, allocator: allocator, pricing: pricing,
		commander: commander, publisher: publisher, timeoutMgr: timeoutMgr,
		orderRepo: orderRepo, cellRepo: cellRepo, cabinetRepo: cabinetRepo,
		log: log.NewHelper(logger),
	}
}

func (uc *StorageUsecase) InitiateStorage(ctx context.Context, tenantID, userID, cabinetID int64, cellType int32) (*StorageOrder, error) {
	cell, err := uc.allocator.AllocateCell(ctx, cabinetID, cellType)
	if err != nil {
		return nil, fmt.Errorf("allocate cell: %w", err)
	}

	cabinet, _ := uc.cabinetRepo.FindByID(ctx, cabinetID)
	order := &StorageOrder{
		TenantID:  tenantID,
		OrderType: OrderTypeStorage,
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

	if err := uc.allocator.MarkOpening(ctx, cell.ID, PendingActionStore, order.ID); err != nil {
		return nil, err
	}

	_, err = uc.commander.OpenCell(ctx, tenantID, cabinet.DeviceSN, cell.SlotIndex, order.OrderNo)
	if err != nil {
		_ = uc.allocator.ReleaseCell(ctx, cell.ID)
		return nil, fmt.Errorf("open cell: %w", err)
	}

	return order, nil
}

func (uc *StorageUsecase) HandleDepositClosed(ctx context.Context, order *StorageOrder, cell *Cell) error {
	fromStatus := order.Status
	if err := uc.fsm.Transition(order.OrderType, order, OrderStatusStoring); err != nil {
		return err
	}
	affected, err := uc.orderRepo.UpdateStatusCAS(ctx, order.ID, fromStatus, OrderStatusStoring)
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

	rule, _ := uc.pricing.MatchRule(ctx, order.TenantID, "storage_overtime", 0)
	freeHours := int32(24)
	if rule != nil {
		freeHours = rule.FreeHours
	}
	uc.timeoutMgr.RegisterOrderTimeout(order.ID, time.Duration(freeHours)*time.Hour)

	return nil
}

func (uc *StorageUsecase) HandleTempOpenClosed(_ context.Context, _ *StorageOrder, cell *Cell) error {
	_ = uc.allocator.ClearPendingAction(nil, cell.ID)
	return nil
}

func (uc *StorageUsecase) HandleRetrieveClosed(ctx context.Context, order *StorageOrder, cell *Cell) error {
	fromStatus := order.Status
	if err := uc.fsm.Transition(order.OrderType, order, OrderStatusCompleted); err != nil {
		return err
	}
	affected, err := uc.orderRepo.UpdateStatusCAS(ctx, order.ID, fromStatus, OrderStatusCompleted)
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil
	}
	_ = uc.allocator.ReleaseCell(ctx, cell.ID)
	_ = uc.allocator.ClearPendingAction(ctx, cell.ID)
	uc.timeoutMgr.CancelTimer(order.ID)
	return nil
}

func (uc *StorageUsecase) OnOrderTimeout(ctx context.Context, order *StorageOrder) error {
	fromStatus := order.Status
	if err := uc.fsm.Transition(order.OrderType, order, OrderStatusTimeout); err != nil {
		return err
	}
	affected, err := uc.orderRepo.UpdateStatusCAS(ctx, order.ID, fromStatus, OrderStatusTimeout)
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil
	}

	rule, _ := uc.pricing.MatchRule(ctx, order.TenantID, "storage_overtime", 0)
	if rule != nil && order.DepositedAt != nil {
		overtimeMinutes := int(time.Since(*order.DepositedAt).Minutes()) - int(rule.FreeHours)*60
		if overtimeMinutes > 0 {
			fee, _ := uc.pricing.CalculateFee(ctx, rule, overtimeMinutes)
			_ = uc.orderRepo.UpdateAmount(ctx, order.ID, fee)
		}
	}

	if uc.publisher != nil {
		_ = uc.publisher.PublishOrderTimeout(ctx, order.TenantID, order.OrderNo, order.UserID, order.TotalAmount)
	}
	return nil
}
