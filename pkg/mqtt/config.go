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

type ClientConfig struct {
	Brokers             []string
	ClientID            string
	Username            string
	Password            string
	KeepAlive           uint16
	CleanStart          bool
	SessionExpiry       uint32
	MaxReconnectDelay   time.Duration
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

func buildAutoPahoConfig(cfg *ClientConfig) (autopaho.ClientConfig, error) {
	defaultConfig(cfg)

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
		OnConnectError:                func(err error) {
			if cfg.OnConnectError != nil {
				cfg.OnConnectError(err)
			}
		},
		ClientConfig: paho.ClientConfig{
			ClientID: cfg.ClientID,
			OnClientError: func(err error) {
				if cfg.OnConnectionLost != nil {
					cfg.OnConnectionLost(err)
				}
			},
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
