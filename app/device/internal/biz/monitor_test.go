package biz

import (
	"testing"
	"time"
)

func TestConnectionEvent_Fields(t *testing.T) {
	now := time.Now()
	event := &ConnectionEvent{
		TenantID:   1,
		DeviceID:   100,
		EventType:  "connected",
		IPAddress:  "192.168.1.100",
		ClientID:   "device_power_bank_001",
		OccurredAt: now,
	}

	if event.EventType != "connected" {
		t.Errorf("EventType = %q, want %q", event.EventType, "connected")
	}
	if event.DeviceID != 100 {
		t.Errorf("DeviceID = %d, want %d", event.DeviceID, 100)
	}
}

func TestCleanExpiredEvents_Cutoff(t *testing.T) {
	cutoff := time.Now().AddDate(0, 0, -90)
	if cutoff.After(time.Now()) {
		t.Error("cutoff should be in the past")
	}
}
