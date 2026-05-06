package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var (
	ErrOrderNotFound    = errors.NotFound("ORDER_NOT_FOUND", "订单不存在")
	ErrPickupCodeWrong  = errors.Forbidden("PICKUP_CODE_WRONG", "取件码错误")
	ErrOrderNotPickable = errors.Forbidden("ORDER_NOT_PICKABLE", "订单状态不可取件")
	ErrPaymentRequired  = errors.Forbidden("PAYMENT_REQUIRED", "需先支付超时费")
)

type DeliveryInUsecase struct {
	fsm         *OrderFSM
	allocator   *CellAllocator
	pricing     *PricingEngine
	pickup      *PickupCodeManager
	commander   *DeviceCommander
	publisher   EventPublisher
	timeoutMgr  *TimeoutManager
	orderRepo   OrderRepo
	cellRepo    CellRepo
	cabinetRepo CabinetRepo
	log         *log.Helper
}

func NewDeliveryInUsecase(
	fsm *OrderFSM,
	allocator *CellAllocator,
	pricing *PricingEngine,
	pickup *PickupCodeManager,
	commander *DeviceCommander,
	publisher EventPublisher,
	timeoutMgr *TimeoutManager,
	orderRepo OrderRepo,
	cellRepo CellRepo,
	cabinetRepo CabinetRepo,
	logger log.Logger,
) *DeliveryInUsecase {
	return &DeliveryInUsecase{
		fsm: fsm, allocator: allocator, pricing: pricing,
		pickup: pickup, commander: commander, publisher: publisher,
		timeoutMgr: timeoutMgr, orderRepo: orderRepo, cellRepo: cellRepo,
		cabinetRepo: cabinetRepo, log: log.NewHelper(logger),
	}
}

func (uc *DeliveryInUsecase) InitiateDelivery(ctx context.Context, tenantID, userID, cabinetID int64, cellType int32, refOrderNo string) (*StorageOrder, error) {
	cell, err := uc.allocator.AllocateCell(ctx, cabinetID, cellType)
	if err != nil {
		return nil, fmt.Errorf("allocate cell: %w", err)
	}

	cabinet, _ := uc.cabinetRepo.FindByID(ctx, cabinetID)
	order := &StorageOrder{
		TenantID:  tenantID,
		OrderType: OrderTypeDeliveryIn,
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
		return nil, fmt.Errorf("mark opening: %w", err)
	}

	_, err = uc.commander.OpenCell(ctx, tenantID, cabinet.DeviceSN, cell.SlotIndex, order.OrderNo)
	if err != nil {
		_ = uc.allocator.ReleaseCell(ctx, cell.ID)
		return nil, fmt.Errorf("open cell: %w", err)
	}

	return order, nil
}

func (uc *DeliveryInUsecase) HandleDepositClosed(ctx context.Context, order *StorageOrder, cell *Cell) error {
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

	code, err := uc.pickup.Generate(ctx, order.TenantID, order.CabinetID, order.ID)
	if err != nil {
		uc.log.Errorf("generate pickup code: %v", err)
	} else {
		order.PickupCode = &code
	}

	now := time.Now()
	order.DepositedAt = &now

	rule, _ := uc.pricing.MatchRule(ctx, order.TenantID, "storage_overtime", 0)
	freeHours := int32(24)
	if rule != nil {
		freeHours = rule.FreeHours
	}
	uc.timeoutMgr.RegisterOrderTimeout(order.ID, time.Duration(freeHours)*time.Hour)

	if uc.publisher != nil {
		cabinet, _ := uc.cabinetRepo.FindByID(ctx, order.CabinetID)
		cabinetName := ""
		if cabinet != nil {
			cabinetName = cabinet.Name
		}
		_ = uc.publisher.PublishPickupReady(ctx, order.TenantID, order.OrderNo, order.UserID, code, cabinetName)
	}

	return nil
}

func (uc *DeliveryInUsecase) HandlePickupClosed(ctx context.Context, order *StorageOrder, cell *Cell) error {
	fromStatus := order.Status
	targetStatus := OrderStatusCompleted

	if err := uc.fsm.Transition(order.OrderType, order, targetStatus); err != nil {
		return err
	}
	affected, err := uc.orderRepo.UpdateStatusCAS(ctx, order.ID, fromStatus, targetStatus)
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil
	}

	_ = uc.allocator.ReleaseCell(ctx, cell.ID)
	_ = uc.allocator.ClearPendingAction(ctx, cell.ID)

	uc.timeoutMgr.CancelTimer(order.ID)

	if order.PickupCode != nil {
		_ = uc.pickup.Revoke(ctx, order.TenantID, order.CabinetID, *order.PickupCode)
	}

	return nil
}

func (uc *DeliveryInUsecase) OnOrderTimeout(ctx context.Context, order *StorageOrder) error {
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
