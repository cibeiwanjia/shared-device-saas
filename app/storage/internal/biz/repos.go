package biz

import (
	"context"
	"time"
)

type CellRepo interface {
	FindByID(ctx context.Context, id int64) (*Cell, error)
	FindByDeviceAndSlot(ctx context.Context, deviceSN string, slotIndex int32) (*Cell, error)
	AllocateForUpdate(ctx context.Context, cabinetID int64, cellType int32) (*Cell, error)
	UpdateStatus(ctx context.Context, id int64, status int32) error
	UpdateStatusCAS(ctx context.Context, id int64, expectedFrom, newStatus int32) (int64, error)
	UpdateOpenedAt(ctx context.Context, id int64, t time.Time) error
	UpdatePendingAction(ctx context.Context, id int64, action *int32, orderID *int64) error
	FindOpenTimeoutCells(ctx context.Context, threshold time.Duration) ([]*Cell, error)
	ListFreeByCabinet(ctx context.Context, cabinetID int64) ([]*Cell, error)
}

type OrderRepo interface {
	Create(ctx context.Context, order *StorageOrder) (*StorageOrder, error)
	GetByID(ctx context.Context, id int64) (*StorageOrder, error)
	GetByOrderNo(ctx context.Context, orderNo string) (*StorageOrder, error)
	UpdateStatusCAS(ctx context.Context, id int64, expectedFrom, newStatus int32) (int64, error)
	UpdateAmount(ctx context.Context, id int64, totalAmount int32) error
	UpdatePickedUp(ctx context.Context, id int64, status int32) error
	FindPossiblyTimeoutOrders(ctx context.Context, threshold time.Duration) ([]*StorageOrder, error)
	ListByUser(ctx context.Context, tenantID, userID int64, orderType string, page, pageSize int32) ([]*StorageOrder, int32, error)
}

type CabinetRepo interface {
	FindByID(ctx context.Context, id int64) (*Cabinet, error)
	ListByTenant(ctx context.Context, tenantID int64, status int32, page, pageSize int32) ([]*Cabinet, int32, error)
	GetFreeCellCount(ctx context.Context, cabinetID int64) (int32, error)
}
