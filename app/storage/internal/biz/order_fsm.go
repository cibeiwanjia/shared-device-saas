package biz

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"
)

var (
	ErrInvalidTransition = errors.Forbidden("INVALID_TRANSITION", "非法状态转移")
	ErrUnknownOrderType  = errors.Forbidden("UNKNOWN_ORDER_TYPE", "未知订单类型")
)

type OrderFSM struct {
	transitions map[string]map[int32][]int32
}

func NewOrderFSM() *OrderFSM {
	return &OrderFSM{
		transitions: map[string]map[int32][]int32{
			OrderTypeDeliveryIn: {
				OrderStatusPending:   {OrderStatusDeposited},
				OrderStatusDeposited: {OrderStatusCompleted, OrderStatusTimeout},
				OrderStatusTimeout:   {OrderStatusCompleted, OrderStatusCleared},
			},
			OrderTypeDeliveryOut: {
				OrderStatusPending:   {OrderStatusDeposited},
				OrderStatusDeposited: {OrderStatusCompleted},
			},
			OrderTypeStorage: {
				OrderStatusPending: {OrderStatusStoring},
				OrderStatusStoring: {OrderStatusCompleted, OrderStatusTimeout, OrderStatusStoring},
				OrderStatusTimeout: {OrderStatusCompleted, OrderStatusCleared},
			},
		},
	}
}

func (f *OrderFSM) CanTransition(orderType string, from, to int32) bool {
	states, ok := f.transitions[orderType]
	if !ok {
		return false
	}
	allowed, ok := states[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

func (f *OrderFSM) Transition(orderType string, order *StorageOrder, to int32) error {
	if !f.CanTransition(orderType, order.Status, to) {
		return fmt.Errorf("%w: %s %d→%d", ErrInvalidTransition, orderType, order.Status, to)
	}
	order.Status = to
	return nil
}
