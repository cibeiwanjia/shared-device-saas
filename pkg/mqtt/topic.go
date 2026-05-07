package mqtt

import "fmt"

// Topic 命名规范（五段式）：
//   {tenant_id}/device/{device_type}/{device_id}/{action}
//
// 为什么 topic 要这么设计：
// 1. tenant_id 在最前面——MQTT 的权限控制（ACL）按 topic 前缀匹配，
//    把 tenant_id 放第一段可以让 Broker 限制"租户 A 只能订阅 1/#"
// 2. device_type 在第三段——服务端可以用通配符订阅某一类设备，
//    比如 "1/device/locker/+/event" 订阅所有快递柜的事件
// 3. device_id 在第四段——精确定位到某一台设备
// 4. action 在最后——区分消息类型（指令/事件/状态/响应）
//
// MQTT topic 就是"地址"，没有数据库那种 WHERE 查询能力，
// 所以 topic 结构决定了你能怎么过滤消息，设计错了后面改不了。
const (
	ActionStatus    = "status"    // 设备主动上报的当前状态（在线/离线、电量等）
	ActionHeartbeat = "heartbeat" // 设备心跳，服务端用来判断设备是否在线
	ActionEvent     = "event"     // 设备事件（门开了、门关了、物品放入），业务层最关心这个
	ActionCommand   = "command"   // 服务→设备的指令（开柜门、强制释放），只发不回
	ActionResponse  = "response"  // 设备对 command 的回复（指令执行成功/失败），通过 msg_id 关联

	DeviceTypePowerBank = "power_bank" // 共享充电宝
	DeviceTypeBike      = "bike"       // 共享单车
	DeviceTypeLocker    = "locker"     // 智能快递柜

	// $events/ 是 EMQX Broker 的系统 topic，设备连接/断开时 Broker 自动发布
	// 不需要设备自己上报，Broker 知道谁连上来了、谁掉了
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

// BuildTenantDeviceWildcard 用 + 通配符订阅某租户下某类型所有设备的某个 action。
// "+" 匹配恰好一段（不能跨层级），所以 "+/device/locker/+/event" 会匹配
// "1/device/locker/CAB-001/event"、"2/device/locker/CAB-002/event" 等
// 但不会匹配 "1/device/locker/CAB-001/event/extra"（多了一层）
func BuildTenantDeviceWildcard(tenantID string, deviceType, action string) string {
	return fmt.Sprintf("%s/device/%s/+/%s", tenantID, deviceType, action)
}

// BuildAllDevicesWildcard 用两个 + 订阅某租户下所有类型所有设备的某个 action。
// 比如 "1/device/+/+/status" 会匹配充电宝、单车、快递柜所有设备的状态上报
func BuildAllDevicesWildcard(tenantID string, action string) string {
	return fmt.Sprintf("%s/device/+/+/%s", tenantID, action)
}

type TopicInfo struct {
	TenantID   string
	DeviceType string
	DeviceID   string
	Action     string
}

// ParseTopic 从收到的 topic 字符串反向解析出结构化信息。
// 用途：收到消息时，从 topic 就能知道是哪个租户的哪台设备发了什么，
// 不需要解析 payload。
func ParseTopic(topic string) (*TopicInfo, bool) {
	parts := splitTopic(topic)
	if len(parts) != 5 {
		return nil, false
	}
	// 第二段固定是 "device"，用来校验这是一个合法的设备 topic
	// 防止把系统 topic（$events/...）或其他 topic 误解析
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

// splitTopic 手动 split，不用 strings.Split 是因为避免分配空字符串。
// 比如 "a//b" 用 strings.Split 会得到 ["a", "", "b"]，
// 这里会跳过空段，得到 ["a", "b"]，更符合 MQTT topic 的语义
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
