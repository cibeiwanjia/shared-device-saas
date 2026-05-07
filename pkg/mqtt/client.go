package mqtt

import (
	"context"
	"fmt"
	"sync"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/go-kratos/kratos/v2/log"
)

// Client 是对 MQTT 连接的封装。
// 设备服务和业务服务都用这个 Client，一个服务实例对应一个 Client。
//
// 核心设计考量：
// - 物理设备在 NAT 后面，服务不可能直连设备，所以用 MQTT Broker 做中转
// - autopaho 库负责自动重连，但重连后 subscription 会丢失，需要手动重新订阅
// - subscriptions map 就是为了在重连时知道"我之前订阅了哪些 topic"
type Client struct {
	config     *ClientConfig
	connection *autopaho.ConnectionManager // autopaho 管理连接生命周期（自动重连）
	router     *paho.StandardRouter        // 收到消息后按 topic 分发到对应 handler
	logger     *log.Helper

	mu          sync.RWMutex        // 保护下面两个字段的并发访问
	subscriptions map[string]uint8  // 记录所有已订阅的 topic → QoS 等级，重连时用来恢复订阅
	connected   bool                // 连接状态标记，供外部判断是否可以 publish
}

func NewClient(ctx context.Context, cfg *ClientConfig, logger log.Logger) (*Client, error) {
	helper := log.NewHelper(logger)

	// 把我们的 ClientConfig 转成 autopaho 需要的配置格式
	autoCfg, err := buildAutoPahoConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build mqtt config: %w", err)
	}

	c := &Client{
		config:       cfg,
		logger:       helper,
		subscriptions: make(map[string]uint8),
	}

	// StandardRouter 是 paho 自带的路由器，支持通配符匹配（+/#
	// 比如订阅了 "1/device/locker/+/event"，收到 "1/device/locker/CAB-001/event" 时能匹配上
	c.router = paho.NewStandardRouter()

	// OnConnectionUp 是 autopaho 的回调：每次连接建立（包括重连）时触发。
	// 这是最关键的设计——重连后必须重新订阅，否则收不到消息。
	// 原因：如果 CleanStart=true，Broker 会清空之前的订阅；
	// 即使 CleanStart=false，客户端侧的订阅状态也不可靠，显式重建更安全。
	autoCfg.OnConnectionUp = func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
		// 标记已连接，供 IsConnected() 判断
		c.mu.Lock()
		c.connected = true
		// 复制一份 subscriptions，避免在锁内做网络调用（Subscribe 是网络操作）
		subs := make(map[string]uint8, len(c.subscriptions))
		for k, v := range c.subscriptions {
			subs[k] = v
		}
		c.mu.Unlock()

		// 逐个重新订阅。这里不并发订阅，因为 MQTT 协议本身是按序的，
		// 且重连不会频繁发生，串行可接受。
		for topic, qos := range subs {
			if _, err := cm.Subscribe(ctx, &paho.Subscribe{
				Subscriptions: []paho.SubscribeOptions{{Topic: topic, QoS: qos}},
			}); err != nil {
				helper.Errorf("resubscribe %s failed: %v", topic, err)
			}
		}

		// 允许调用方在连接建立后做额外操作（比如发一个状态查询）
		if cfg.OnConnectionUp != nil {
			cfg.OnConnectionUp(cm, connAck)
		}
		helper.Info("MQTT connection established")
	}

	// OnPublishReceived 是所有入站消息的入口。
	// autopaho 收到任何 publish 报文都会调用这个函数。
	// 我们把消息交给 router，router 根据 topic 匹配到对应的 handler 执行。
	autoCfg.ClientConfig.OnPublishReceived = []func(paho.PublishReceived) (bool, error){
		func(pr paho.PublishReceived) (bool, error) {
			c.router.Route(pr.Packet.Packet())
			return true, nil // 返回 true 表示消息已处理
		},
	}

	// autopaho.NewConnection 创建连接管理器，此时还没有真正连接
	cm, err := autopaho.NewConnection(ctx, autoCfg)
	if err != nil {
		return nil, fmt.Errorf("create mqtt connection: %w", err)
	}

	// AwaitConnection 阻塞等待第一次连接建立。
	// 如果 Broker 不可用，这里会超时返回错误，服务启动失败——这是正确的，
	// 因为连不上 Broker 的服务没有意义。
	if err := cm.AwaitConnection(ctx); err != nil {
		return nil, fmt.Errorf("await mqtt connection: %w", err)
	}

	c.connection = cm
	return c, nil
}

// ConnectionManager 暴露底层连接管理器，用于需要直接操作 autopaho 的场景（比如自定义订阅）
func (c *Client) ConnectionManager() *autopaho.ConnectionManager {
	return c.connection
}

// IsConnected 供业务层判断是否可以 publish。
// 比如 DeviceCommandService 在 publish 前检查：
//   if client != nil && client.IsConnected() { ... } else { log.Warn(...) }
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Close 优雅关闭：先标记断开，再发 DISCONNECT 报文给 Broker。
// DISCONNECT 报文告诉 Broker "我是主动断开的，不要触发 Will 消息"。
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	if c.connection != nil {
		return c.connection.Disconnect(ctx)
	}
	return nil
}

// Done 返回一个 channel，连接彻底断开时关闭。
// 用法：select { case <-client.Done(): // 连接已死 }
func (c *Client) Done() <-chan struct{} {
	if c.connection != nil {
		return c.connection.Done()
	}
	// 如果 connection 为 nil（比如创建失败），返回一个已关闭的 channel
	ch := make(chan struct{})
	close(ch)
	return ch
}
