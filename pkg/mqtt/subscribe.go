package mqtt

import (
	"context"
	"fmt"

	"github.com/eclipse/paho.golang/paho"
)

func (c *Client) Subscribe(ctx context.Context, topic string, qos uint8) error {
	if c.connection == nil {
		return ErrNotConnected
	}

	_, err := c.connection.Subscribe(ctx, &paho.Subscribe{
		Subscriptions: []paho.SubscribeOptions{{Topic: topic, QoS: qos}},
	})
	if err != nil {
		return fmt.Errorf("subscribe %s: %w", topic, err)
	}

	c.mu.Lock()
	c.subscriptions[topic] = qos
	c.mu.Unlock()

	return nil
}

func (c *Client) Unsubscribe(ctx context.Context, topics ...string) error {
	if c.connection == nil {
		return ErrNotConnected
	}

	_, err := c.connection.Unsubscribe(ctx, &paho.Unsubscribe{
		Topics: topics,
	})
	if err != nil {
		return fmt.Errorf("unsubscribe: %w", err)
	}

	c.mu.Lock()
	for _, t := range topics {
		delete(c.subscriptions, t)
	}
	c.mu.Unlock()

	return nil
}
