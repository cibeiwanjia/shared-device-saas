package mqtt

import "fmt"

// Topic format: {tenant_id}/device/{device_type}/{device_id}/{action}
const (
	ActionStatus    = "status"
	ActionHeartbeat = "heartbeat"
	ActionEvent     = "event"
	ActionCommand   = "command"
	ActionResponse  = "response"

	DeviceTypePowerBank = "power_bank"
	DeviceTypeBike      = "bike"
	DeviceTypeLocker    = "locker"

	SysTopicClientConnected    = "$events/client_connected"
	SysTopicClientDisconnected = "$events/client_disconnected"
)

func BuildStatusTopic(tenantID string, deviceType, deviceID string) string {
	return fmt.Sprintf("%s/device/%s/%s/%s", tenantID, deviceType, deviceID, ActionStatus)
}

func BuildHeartbeatTopic(tenantID string, deviceType, deviceID string) string {
	return fmt.Sprintf("%s/device/%s/%s/%s", tenantID, deviceType, deviceID, ActionHeartbeat)
}

func BuildEventTopic(tenantID string, deviceType, deviceID string) string {
	return fmt.Sprintf("%s/device/%s/%s/%s", tenantID, deviceType, deviceID, ActionEvent)
}

func BuildCommandTopic(tenantID string, deviceType, deviceID string) string {
	return fmt.Sprintf("%s/device/%s/%s/%s", tenantID, deviceType, deviceID, ActionCommand)
}

func BuildResponseTopic(tenantID string, deviceType, deviceID string) string {
	return fmt.Sprintf("%s/device/%s/%s/%s", tenantID, deviceType, deviceID, ActionResponse)
}

func BuildTenantDeviceWildcard(tenantID string, deviceType, action string) string {
	return fmt.Sprintf("%s/device/%s/+/%s", tenantID, deviceType, action)
}

func BuildAllDevicesWildcard(tenantID string, action string) string {
	return fmt.Sprintf("%s/device/+/+/%s", tenantID, action)
}

type TopicInfo struct {
	TenantID   string
	DeviceType string
	DeviceID   string
	Action     string
}

func ParseTopic(topic string) (*TopicInfo, bool) {
	parts := splitTopic(topic)
	if len(parts) != 5 {
		return nil, false
	}
	if parts[1] != "device" {
		return nil, false
	}
	return &TopicInfo{
		TenantID:   parts[0],
		DeviceType: parts[2],
		DeviceID:   parts[3],
		Action:     parts[4],
	}, true
}

func splitTopic(topic string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(topic); i++ {
		if i == len(topic) || topic[i] == '/' {
			if i > start {
				parts = append(parts, topic[start:i])
			}
			start = i + 1
		}
	}
	return parts
}
