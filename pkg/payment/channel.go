package payment

// PaymentChannel 支付渠道接口
type PaymentChannel interface {
	// CreateOrder 创建支付订单，返回支付参数（JSON）
	CreateOrder(orderNo string, amount int64, desc string) (payParams string, err error)
	// QueryOrder 查询支付状态
	QueryOrder(orderNo string) (paid bool, channelOrderNo string, err error)
	// VerifyCallback 验证回调签名
	VerifyCallback(payload, signature string) (orderNo string, paid bool, err error)
	// Refund 退款
	Refund(orderNo string, amount int64) error
}
