package biz

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var (
	ErrNoFreeCell  = errors.NotFound("NO_FREE_CELL", "无空闲格口")
	ErrCellNotFree = errors.Conflict("CELL_NOT_FREE", "格口非空闲状态")
)

type CellAllocator struct {
	cellRepo CellRepo
	log      *log.Helper
}

func NewCellAllocator(cellRepo CellRepo, logger log.Logger) *CellAllocator {
	return &CellAllocator{cellRepo: cellRepo, log: log.NewHelper(logger)}
}

func (a *CellAllocator) AllocateCell(ctx context.Context, cabinetID int64, cellType int32) (*Cell, error) {
	cell, err := a.cellRepo.AllocateForUpdate(ctx, cabinetID, cellType)
	if err != nil {
		return nil, fmt.Errorf("allocate cell: %w", err)
	}
	if cell == nil {
		return nil, ErrNoFreeCell
	}
	return cell, nil
}

func (a *CellAllocator) ReleaseCell(ctx context.Context, cellID int64) error {
	affected, err := a.cellRepo.UpdateStatusCAS(ctx, cellID, CellStatusOccupied, CellStatusFree)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrCellNotFree
	}
	return nil
}

func (a *CellAllocator) MarkOpening(ctx context.Context, cellID int64, pendingAction int32, orderID int64) error {
	action := int32(pendingAction)
	order := int64(orderID)
	if err := a.cellRepo.UpdatePendingAction(ctx, cellID, &action, &order); err != nil {
		return err
	}
	return a.cellRepo.UpdateStatus(ctx, cellID, CellStatusOpening)
}

func (a *CellAllocator) ConfirmOccupied(ctx context.Context, cellID int64) error {
	return a.cellRepo.UpdateStatus(ctx, cellID, CellStatusOccupied)
}

func (a *CellAllocator) ClearPendingAction(ctx context.Context, cellID int64) error {
	return a.cellRepo.UpdatePendingAction(ctx, cellID, nil, nil)
}
