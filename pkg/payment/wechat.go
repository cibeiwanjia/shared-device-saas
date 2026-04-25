package payment

import "fmt"

// WechatPayChannel 微信支付桩实现
type WechatPayChannel struct{}

// NewWechatPayChannel 创建微信支付渠道
func NewWechatPayChannel() *WechatPayChannel {
	return &WechatPayChannel{}
}

func (c *WechatPayChannel) CreateOrder(orderNo string, amount int64, desc string) (string, error) {
	// TODO: 接入微信支付 SDK
	return fmt.Sprintf(`{"prepay_id":"mock_%s","amount":%d}`, orderNo, amount), nil
}

func (c *WechatPayChannel) QueryOrder(orderNo string) (bool, string, error) {
	// TODO: 接入微信支付查询
	return false, "", nil
}

func (c *WechatPayChannel) VerifyCallback(payload, signature string) (string, bool, error) {
	// TODO: 验证微信回调签名
	return "", false, fmt.Errorf("not implemented")
}

func (c *WechatPayChannel) Refund(orderNo string, amount int64) error {
	// TODO: 接入微信退款
	return nil
}
