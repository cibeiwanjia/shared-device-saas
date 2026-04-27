package mqtt

import "github.com/eclipse/paho.golang/paho"

func (c *Client) RegisterHandler(topic string, handler func(p *paho.Publish)) {
	c.router.RegisterHandler(topic, handler)
}

func (c *Client) DefaultHandler(handler func(p *paho.Publish)) {
	c.router.DefaultHandler(handler)
}
