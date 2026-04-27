package mqtt

import (
	"context"
	"fmt"
	"time"

	"github.com/eclipse/paho.golang/paho"
)

var defaultPublishTimeout = 5 * time.Second

func (c *Client) Publish(ctx context.Context, topic string, qos uint8, retained bool, payload []byte) error {
	if c.connection == nil {
		return ErrNotConnected
	}

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
