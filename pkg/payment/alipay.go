package payment

import "fmt"

// AlipayChannel 支付宝桩实现
type AlipayChannel struct{}

// NewAlipayChannel 创建支付宝渠道
func NewAlipayChannel() *AlipayChannel {
	return &AlipayChannel{}
}

func (c *AlipayChannel) CreateOrder(orderNo string, amount int64, desc string) (string, error) {
	// TODO: 接入支付宝 SDK
	return fmt.Sprintf(`{"trade_no":"mock_%s","amount":%d}`, orderNo, amount), nil
}

func (c *AlipayChannel) QueryOrder(orderNo string) (bool, string, error) {
	// TODO: 接入支付宝查询
	return false, "", nil
}

func (c *AlipayChannel) VerifyCallback(payload, signature string) (string, bool, error) {
	// TODO: 验证支付宝回调签名
	return "", false, fmt.Errorf("not implemented")
}

func (c *AlipayChannel) Refund(orderNo string, amount int64) error {
	// TODO: 接入支付宝退款
	return nil
}
