package mqtt

import (
	"context"
	"fmt"
	"time"

	"github.com/eclipse/paho.golang/paho"
)

var defaultPublishTimeout = 5 * time.Second

// Publish 发送消息到指定 topic。
//
// QoS 参数的含义：
//   QoS 0 = 至多发一次，不保证到达（fire and forget）。适合心跳、状态上报——丢了无所谓
//   QoS 1 = 至少一次，保证到达但可能重复。适合开柜指令——必须到达，重复了业务层用 msg_id 幂等去重
//   QoS 2 = 恰好一次，四次握手延迟高。本项目不用——QoS 1 + 业务层幂等效果相同且更快
//
// Retained 参数：
//   true = Broker 保存这条消息，新订阅者订阅时立即收到最后一条 retained 消息
//   用途：设备上线状态（新订阅者立即知道设备当前是开还是关）
//   false = 普通消息，只有当前订阅者能收到
func (c *Client) Publish(ctx context.Context, topic string, qos uint8, retained bool, payload []byte) error {
	if c.connection == nil {
		return ErrNotConnected
	}

	// 5 秒超时保护。publish 是网络操作，如果 Broker 响应慢或网络抖动，
	// 不能让业务请求无限等下去。超时后业务层可以决定重试或降级。
	pubCtx, cancel := context.WithTimeout(ctx, defaultPublishTimeout)
	defer cancel()

	_, err := c.connection.Publish(pubCtx, &paho.Publish{
		Topic:   topic,
		QoS:     qos,
		Retain:  retained,
		Payload: payload,
	})
	if err != nil {
		return fmt.Errorf("publish to %s: %w", topic, err)
	}
	return nil
}
