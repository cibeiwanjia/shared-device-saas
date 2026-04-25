package errx

// 业务错误码
const (
	// 通用
	CodeSuccess      = 0
	CodeUnknown      = 10000
	CodeInvalidParam = 10001
	CodeUnauthorized = 10002
	CodeForbidden    = 10003
	CodeNotFound     = 10004
	CodeConflict     = 10005
	CodeInternal     = 10006

	// 用户服务 20xxx
	CodeUserNotFound = 20001
	CodeUserDisabled = 20002
	CodeTokenExpired = 20003
	CodeTokenInvalid = 20004

	// 订单服务 30xxx
	CodeOrderNotFound  = 30001
	CodeOrderStatusErr = 30002

	// 钱包服务 40xxx
	CodeWalletNotFound  = 40001
	CodeBalanceLow      = 40002
	CodeVersionConflict = 40003

	// 充值服务 50xxx
	CodeRechargeFailed = 50001
	CodePaymentFailed  = 50002

	// 上传服务 60xxx
	CodeFileTypeErr = 60001
	CodeFileSizeErr = 60002
)

// 错误码对应的默认消息
var codeMessages = map[int32]string{
	CodeSuccess:         "成功",
	CodeUnknown:         "未知错误",
	CodeInvalidParam:    "参数错误",
	CodeUnauthorized:    "未授权",
	CodeForbidden:       "禁止访问",
	CodeNotFound:        "资源不存在",
	CodeConflict:        "资源冲突",
	CodeInternal:        "内部错误",
	CodeUserNotFound:    "用户不存在",
	CodeUserDisabled:    "用户已禁用",
	CodeTokenExpired:    "Token 已过期",
	CodeTokenInvalid:    "Token 无效",
	CodeOrderNotFound:   "订单不存在",
	CodeOrderStatusErr:  "订单状态异常",
	CodeWalletNotFound:  "钱包不存在",
	CodeBalanceLow:      "余额不足",
	CodeVersionConflict: "数据版本冲突",
	CodeRechargeFailed:  "充值失败",
	CodePaymentFailed:   "支付失败",
	CodeFileTypeErr:     "文件类型不支持",
	CodeFileSizeErr:     "文件大小超限",
}

// GetMessage 获取错误码对应的消息
func GetMessage(code int32) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
