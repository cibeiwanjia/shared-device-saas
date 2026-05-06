package biz

import "testing"

func TestCanTransition_DeliveryIn(t *testing.T) {
	fsm := NewOrderFSM()

	tests := []struct {
		from, to int32
		want     bool
	}{
		{OrderStatusPending, OrderStatusDeposited, true},
		{OrderStatusDeposited, OrderStatusCompleted, true},
		{OrderStatusDeposited, OrderStatusTimeout, true},
		{OrderStatusTimeout, OrderStatusCompleted, true},
		{OrderStatusTimeout, OrderStatusCleared, true},
		{OrderStatusPending, OrderStatusCompleted, false},
		{OrderStatusCompleted, OrderStatusPending, false},
	}
	for _, tt := range tests {
		got := fsm.CanTransition(OrderTypeDeliveryIn, tt.from, tt.to)
		if got != tt.want {
			t.Errorf("CanTransition(delivery_in, %d, %d) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestCanTransition_Storage(t *testing.T) {
	fsm := NewOrderFSM()

	tests := []struct {
		from, to int32
		want     bool
	}{
		{OrderStatusPending, OrderStatusStoring, true},
		{OrderStatusStoring, OrderStatusCompleted, true},
		{OrderStatusStoring, OrderStatusStoring, true},
		{OrderStatusStoring, OrderStatusTimeout, true},
		{OrderStatusPending, OrderStatusDeposited, false},
	}
	for _, tt := range tests {
		got := fsm.CanTransition(OrderTypeStorage, tt.from, tt.to)
		if got != tt.want {
			t.Errorf("CanTransition(storage, %d, %d) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestCanTransition_DeliveryOut(t *testing.T) {
	fsm := NewOrderFSM()

	tests := []struct {
		from, to int32
		want     bool
	}{
		{OrderStatusPending, OrderStatusDeposited, true},
		{OrderStatusDeposited, OrderStatusCompleted, true},
		{OrderStatusDeposited, OrderStatusTimeout, false},
	}
	for _, tt := range tests {
		got := fsm.CanTransition(OrderTypeDeliveryOut, tt.from, tt.to)
		if got != tt.want {
			t.Errorf("CanTransition(delivery_out, %d, %d) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestTransition_ModifiesStatus(t *testing.T) {
	fsm := NewOrderFSM()
	order := &StorageOrder{OrderType: OrderTypeDeliveryIn, Status: OrderStatusPending}

	err := fsm.Transition(order.OrderType, order, OrderStatusDeposited)
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}
	if order.Status != OrderStatusDeposited {
		t.Errorf("order.Status = %d, want %d", order.Status, OrderStatusDeposited)
	}
}

func TestTransition_InvalidReturnsError(t *testing.T) {
	fsm := NewOrderFSM()
	order := &StorageOrder{OrderType: OrderTypeDeliveryIn, Status: OrderStatusCompleted}

	err := fsm.Transition(order.OrderType, order, OrderStatusPending)
	if err == nil {
		t.Error("expected error for invalid transition, got nil")
	}
}
