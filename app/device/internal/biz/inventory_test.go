package biz

import (
	"encoding/json"
	"testing"
)

func TestDeviceErrorDefinitions(t *testing.T) {
	if ErrDeviceNotFound == nil {
		t.Error("ErrDeviceNotFound should not be nil")
	}
	if ErrDeviceAlreadyExists == nil {
		t.Error("ErrDeviceAlreadyExists should not be nil")
	}
	if ErrDeviceBusy == nil {
		t.Error("ErrDeviceBusy should not be nil")
	}
}

func TestDeviceStatusConstants(t *testing.T) {
	if DeviceStatusTTL.Seconds() != 300 {
		t.Errorf("DeviceStatusTTL = %v, want 5 minutes", DeviceStatusTTL)
	}
	if DeviceLockTTL.Seconds() != 10 {
		t.Errorf("DeviceLockTTL = %v, want 10 seconds", DeviceLockTTL)
	}
}

func TestDeviceStatus_MarshalUnmarshal(t *testing.T) {
	status := &DeviceStatus{
		DeviceID:     "dev001",
		DeviceType:   "power_bank",
		Status:       1,
		BatteryLevel: 85,
		TenantID:     "1",
	}

	ds, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed DeviceStatus
	if err := json.Unmarshal(ds, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed.DeviceID != status.DeviceID {
		t.Errorf("DeviceID = %q, want %q", parsed.DeviceID, status.DeviceID)
	}
	if parsed.Status != status.Status {
		t.Errorf("Status = %d, want %d", parsed.Status, status.Status)
	}
	if parsed.BatteryLevel != status.BatteryLevel {
		t.Errorf("BatteryLevel = %d, want %d", parsed.BatteryLevel, status.BatteryLevel)
	}
}
