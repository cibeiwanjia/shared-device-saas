package mqtt

import (
	"net/url"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

type OnConnectionUpFunc func(cm *autopaho.ConnectionManager, connAck *paho.Connack)
type OnConnectionLostFunc func(err error)
type OnConnectErrorFunc func(err error)

// ClientConfig 是我们自己的配置结构，屏蔽 autopaho/paho 的复杂配置。
// 关键参数解释：
//   - CleanStart: false 表示重连时恢复之前的会话（Broker 记住了你的订阅）
//     如果设为 true，每次连接 Broker 都当你是个全新客户端，之前的订阅和离线消息全丢
//   - SessionExpiry: 会话过期时间（秒），即使断开了 Broker 也保留这么久的会话状态
//     设成 3600 = 断线 1 小时内重连，Broker 还记得你，离线期间收到的 QoS 1/2 消息会补发
//   - KeepAlive: 心跳间隔，Broker 在 1.5 倍 KeepAlive 内没收到任何报文就认为设备掉线
//     60 秒意味着 90 秒无响应判定离线
type ClientConfig struct {
	Brokers             []string          // Broker 地址列表，支持多个做高可用（连第一个失败的会试第二个）
	ClientID            string            // 客户端唯一标识，Broker 靠这个区分连接。同名 ClientID 后连接的会踢掉前面的
	Username            string            // Broker 认证用户名（EMQX 支持用户名密码认证）
	Password            string            // Broker 认证密码
	KeepAlive           uint16            // 心跳间隔（秒）
	CleanStart          bool              // false=恢复会话，true=全新会话
	SessionExpiry       uint32            // 会话保留时间（秒），断线后 Broker 保留订阅和离线消息多久
	MaxReconnectDelay   time.Duration     // 重连最大退避时间，避免 Broker 挂了时疯狂重连
	OnConnectionUp      OnConnectionUpFunc
	OnConnectionLost    OnConnectionLostFunc
	OnConnectError      OnConnectErrorFunc
}

func defaultConfig(cfg *ClientConfig) {
	if cfg.KeepAlive == 0 {
		cfg.KeepAlive = 60
	}
	if cfg.MaxReconnectDelay == 0 {
		cfg.MaxReconnectDelay = 30 * time.Second
	}
}

// buildAutoPahoConfig 把 ClientConfig 转成 autopaho.ClientConfig。
// autopaho 是 paho 的上层封装，多了自动重连能力。
// paho 本身只管 MQTT 协议，不管重连——断线就断了。
func buildAutoPahoConfig(cfg *ClientConfig) (autopaho.ClientConfig, error) {
	defaultConfig(cfg)

	// 解析 Broker URL，支持 "tcp://host:port" 和 "ws://host:port" 格式
	// tcp:// 用于原生 MQTT，ws:// 用于浏览器 WebSocket 连接
	var serverURLs []*url.URL
	for _, broker := range cfg.Brokers {
		u, err := url.Parse(broker)
		if err != nil {
			return autopaho.ClientConfig{}, err
		}
		serverURLs = append(serverURLs, u)
	}

	autoCfg := autopaho.ClientConfig{
		ServerUrls:                    serverURLs,
		KeepAlive:                     cfg.KeepAlive,
		CleanStartOnInitialConnection: cfg.CleanStart,
		SessionExpiryInterval:         cfg.SessionExpiry,
		OnConnectError: func(err error) {
			if cfg.OnConnectError != nil {
				cfg.OnConnectError(err)
			}
		},
		ClientConfig: paho.ClientConfig{
			ClientID: cfg.ClientID,
			// OnClientError 是连接建立后的运行时错误（比如 Broker 主动断开）
			OnClientError: func(err error) {
				if cfg.OnConnectionLost != nil {
					cfg.OnConnectionLost(err)
				}
			},
			// OnServerDisconnect 是服务端发 DISCONNECT 报文时的回调
			// 目前空实现，因为 autopaho 会自动重连
			OnServerDisconnect: func(d *paho.Disconnect) {},
		},
	}

	if cfg.Username != "" {
		autoCfg.ConnectUsername = cfg.Username
	}
	if cfg.Password != "" {
		autoCfg.ConnectPassword = []byte(cfg.Password)
	}

	return autoCfg, nil
}
