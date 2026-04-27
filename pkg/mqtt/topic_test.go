package mqtt

import "testing"

func TestBuildStatusTopic(t *testing.T) {
	got := BuildStatusTopic("1", "power_bank", "dev001")
	want := "1/device/power_bank/dev001/status"
	if got != want {
		t.Errorf("BuildStatusTopic() = %q, want %q", got, want)
	}
}

func TestBuildHeartbeatTopic(t *testing.T) {
	got := BuildHeartbeatTopic("1", "bike", "dev002")
	want := "1/device/bike/dev002/heartbeat"
	if got != want {
		t.Errorf("BuildHeartbeatTopic() = %q, want %q", got, want)
	}
}

func TestBuildCommandTopic(t *testing.T) {
	got := BuildCommandTopic("2", "locker", "dev003")
	want := "2/device/locker/dev003/command"
	if got != want {
		t.Errorf("BuildCommandTopic() = %q, want %q", got, want)
	}
}

func TestBuildTenantDeviceWildcard(t *testing.T) {
	got := BuildTenantDeviceWildcard("1", "power_bank", "status")
	want := "1/device/power_bank/+/status"
	if got != want {
		t.Errorf("BuildTenantDeviceWildcard() = %q, want %q", got, want)
	}
}

func TestBuildAllDevicesWildcard(t *testing.T) {
	got := BuildAllDevicesWildcard("1", "status")
	want := "1/device/+/+/status"
	if got != want {
		t.Errorf("BuildAllDevicesWildcard() = %q, want %q", got, want)
	}
}

func TestParseTopic(t *testing.T) {
	info, ok := ParseTopic("1/device/power_bank/dev001/status")
	if !ok {
		t.Fatal("ParseTopic() returned false")
	}
	if info.TenantID != "1" {
		t.Errorf("TenantID = %q, want %q", info.TenantID, "1")
	}
	if info.DeviceType != "power_bank" {
		t.Errorf("DeviceType = %q, want %q", info.DeviceType, "power_bank")
	}
	if info.DeviceID != "dev001" {
		t.Errorf("DeviceID = %q, want %q", info.DeviceID, "dev001")
	}
	if info.Action != "status" {
		t.Errorf("Action = %q, want %q", info.Action, "status")
	}
}

func TestParseTopic_Invalid(t *testing.T) {
	_, ok := ParseTopic("invalid/topic")
	if ok {
		t.Error("ParseTopic() should return false for invalid topic")
	}

	_, ok = ParseTopic("1/notdevice/power_bank/dev001/status")
	if ok {
		t.Error("ParseTopic() should return false when second segment is not 'device'")
	}
}

func TestParseTopic_TooManySegments(t *testing.T) {
	_, ok := ParseTopic("1/device/power_bank/dev001/status/extra")
	if ok {
		t.Error("ParseTopic() should return false for too many segments")
	}
}

func TestSplitTopic(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"a/b/c/d/e", 5},
		{"/a/b/c/d/e", 5},
		{"a/b/c/d/e/", 5},
		{"a//b/c", 3},
		{"", 0},
	}
	for _, tt := range tests {
		parts := splitTopic(tt.input)
		if len(parts) != tt.want {
			t.Errorf("splitTopic(%q) returned %d parts, want %d", tt.input, len(parts), tt.want)
		}
	}
}
