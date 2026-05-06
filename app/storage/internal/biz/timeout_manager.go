package biz

import (
	"context"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type TimeoutCallback func(ctx context.Context, id int64) error

type TimeoutManager struct {
	cellRepo        CellRepo
	deviceCommander *DeviceCommander
	cellAllocator   *CellAllocator
	eventPublisher  EventPublisher
	orderRepo       OrderRepo
	orderTimeoutCb  TimeoutCallback
	log             *log.Helper

	mu     sync.Mutex
	timers map[int64]*time.Timer
}

func NewTimeoutManager(
	cellRepo CellRepo,
	deviceCommander *DeviceCommander,
	cellAllocator *CellAllocator,
	eventPublisher EventPublisher,
	orderRepo OrderRepo,
	logger log.Logger,
) *TimeoutManager {
	return &TimeoutManager{
		cellRepo:        cellRepo,
		deviceCommander: deviceCommander,
		cellAllocator:   cellAllocator,
		eventPublisher:  eventPublisher,
		orderRepo:       orderRepo,
		log:             log.NewHelper(logger),
		timers:          make(map[int64]*time.Timer),
	}
}

func (m *TimeoutManager) SetOrderTimeoutCallback(cb TimeoutCallback) {
	m.orderTimeoutCb = cb
}

func (m *TimeoutManager) RegisterOrderTimeout(orderID int64, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := orderID
	m.timers[key] = time.AfterFunc(duration, func() {
		ctx := context.Background()
		if m.orderTimeoutCb != nil {
			if err := m.orderTimeoutCb(ctx, orderID); err != nil {
				m.log.Errorf("order timeout callback failed: orderID=%d err=%v", orderID, err)
			}
		}
		delete(m.timers, key)
	})
}

func (m *TimeoutManager) RegisterOpenTimeout(cellID int64, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := -cellID
	m.timers[key] = time.AfterFunc(duration, func() {
		ctx := context.Background()
		if err := m.HandleOpenTimeout(ctx, cellID); err != nil {
			m.log.Errorf("open timeout handler failed: cellID=%d err=%v", cellID, err)
		}
		delete(m.timers, key)
	})
}

func (m *TimeoutManager) HandleOpenTimeout(ctx context.Context, cellID int64) error {
	cell, err := m.cellRepo.FindByID(ctx, cellID)
	if err != nil || cell == nil {
		return err
	}

	_ = m.deviceCommander.ForceReleaseCell(ctx, cell.TenantID, "", cell.SlotIndex, "system", "开门超时自动回退")

	if err := m.cellAllocator.ConfirmOccupied(ctx, cellID); err != nil {
		return err
	}

	if m.eventPublisher != nil {
		_ = m.eventPublisher.PublishOpenTimeout(ctx, cell.TenantID, "", cellID, "")
	}
	return nil
}

func (m *TimeoutManager) StartDBPolling(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			orders, err := m.orderRepo.FindPossiblyTimeoutOrders(ctx, 1*time.Hour)
			if err != nil {
				m.log.Errorf("timeout scan orders: %v", err)
			} else {
				for _, o := range orders {
					if m.orderTimeoutCb != nil {
						_ = m.orderTimeoutCb(ctx, o.ID)
					}
				}
			}

			cells, err := m.cellRepo.FindOpenTimeoutCells(ctx, 5*time.Minute)
			if err != nil {
				m.log.Errorf("timeout scan cells: %v", err)
			} else {
				for _, c := range cells {
					_ = m.HandleOpenTimeout(ctx, c.ID)
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

func (m *TimeoutManager) CancelTimer(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.timers[id]; ok {
		t.Stop()
		delete(m.timers, id)
	}
}
