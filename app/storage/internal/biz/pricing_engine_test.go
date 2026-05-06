package biz

import (
	"context"
	"testing"
)

func TestCalculateFee_FreePeriod(t *testing.T) {
	engine := NewPricingEngine(&mockPricingRepo{})
	rule := &PricingRule{FreeHours: 24, PricePerHour: 100, MaxFee: 5000}
	fee, err := engine.CalculateFee(context.Background(), rule, 0)
	if err != nil {
		t.Fatal(err)
	}
	if fee != 0 {
		t.Errorf("fee = %d, want 0 within free period", fee)
	}
}

func TestCalculateFee_Overtime(t *testing.T) {
	engine := NewPricingEngine(&mockPricingRepo{})
	rule := &PricingRule{FreeHours: 24, PricePerHour: 100, MaxFee: 5000}
	fee, err := engine.CalculateFee(context.Background(), rule, 180)
	if err != nil {
		t.Fatal(err)
	}
	if fee != 300 {
		t.Errorf("fee = %d, want 300 (3h * 100)", fee)
	}
}

func TestCalculateFee_MaxFeeCap(t *testing.T) {
	engine := NewPricingEngine(&mockPricingRepo{})
	rule := &PricingRule{FreeHours: 0, PricePerHour: 100, MaxFee: 500}
	fee, err := engine.CalculateFee(context.Background(), rule, 6000)
	if err != nil {
		t.Fatal(err)
	}
	if fee != 500 {
		t.Errorf("fee = %d, want 500 (capped at MaxFee)", fee)
	}
}

func TestCalculateFee_NilRule(t *testing.T) {
	engine := NewPricingEngine(&mockPricingRepo{})
	fee, err := engine.CalculateFee(context.Background(), nil, 60)
	if err != nil {
		t.Fatal(err)
	}
	if fee != 0 {
		t.Errorf("fee = %d, want 0 for nil rule", fee)
	}
}

type mockPricingRepo struct{}

func (m *mockPricingRepo) FindByTenantAndType(_ context.Context, _ int64, _ string, _ int32) (*PricingRule, error) {
	return nil, nil
}
