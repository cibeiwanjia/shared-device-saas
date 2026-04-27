package mqtt

import (
	"context"
	"fmt"
	"sync"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/go-kratos/kratos/v2/log"
)

type Client struct {
	config     *ClientConfig
	connection *autopaho.ConnectionManager
	router     *paho.StandardRouter
	logger     *log.Helper

	mu          sync.RWMutex
	subscriptions map[string]uint8
	connected   bool
}

func NewClient(ctx context.Context, cfg *ClientConfig, logger log.Logger) (*Client, error) {
	helper := log.NewHelper(logger)

	autoCfg, err := buildAutoPahoConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build mqtt config: %w", err)
	}

	c := &Client{
		config:       cfg,
		logger:       helper,
		subscriptions: make(map[string]uint8),
	}

	c.router = paho.NewStandardRouter()

	autoCfg.OnConnectionUp = func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
		c.mu.Lock()
		c.connected = true
		subs := make(map[string]uint8, len(c.subscriptions))
		for k, v := range c.subscriptions {
			subs[k] = v
		}
		c.mu.Unlock()

		for topic, qos := range subs {
			if _, err := cm.Subscribe(ctx, &paho.Subscribe{
				Subscriptions: []paho.SubscribeOptions{{Topic: topic, QoS: qos}},
			}); err != nil {
				helper.Errorf("resubscribe %s failed: %v", topic, err)
			}
		}

		if cfg.OnConnectionUp != nil {
			cfg.OnConnectionUp(cm, connAck)
		}
		helper.Info("MQTT connection established")
	}

	autoCfg.ClientConfig.OnPublishReceived = []func(paho.PublishReceived) (bool, error){
		func(pr paho.PublishReceived) (bool, error) {
			c.router.Route(pr.Packet.Packet())
			return true, nil
		},
	}

	cm, err := autopaho.NewConnection(ctx, autoCfg)
	if err != nil {
		return nil, fmt.Errorf("create mqtt connection: %w", err)
	}

	if err := cm.AwaitConnection(ctx); err != nil {
		return nil, fmt.Errorf("await mqtt connection: %w", err)
	}

	c.connection = cm
	return c, nil
}

func (c *Client) ConnectionManager() *autopaho.ConnectionManager {
	return c.connection
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	if c.connection != nil {
		return c.connection.Disconnect(ctx)
	}
	return nil
}

func (c *Client) Done() <-chan struct{} {
	if c.connection != nil {
		return c.connection.Done()
	}
	ch := make(chan struct{})
	close(ch)
	return ch
}
