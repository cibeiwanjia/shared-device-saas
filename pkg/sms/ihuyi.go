package sms

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// IhuyiClient 互亿无线短信客户端
type IhuyiClient struct {
	apiURL     string
	account    string // APIID
	password   string // APIKEY
	templateID string // 短信模板ID
	log        *log.Helper
}

// NewIhuyiClient 创建互亿无线短信客户端
func NewIhuyiClient(apiURL, account, password, templateID string, logger log.Logger) *IhuyiClient {
	return &IhuyiClient{
		apiURL:     apiURL,
		account:    account,
		password:   password,
		templateID: templateID,
		log:        log.NewHelper(logger),
	}
}

// IhuyiResponse 互亿无线API响应
type IhuyiResponse struct {
	Code  int    `json:"code"`  // 2表示成功
	Msg   string `json:"msg"`   // 错误信息
	SmsID string `json:"smsid"` // 短信ID
}

// SendCode 发送验证码短信
// 返回：验证码, 错误
func (c *IhuyiClient) SendCode(phone string) (string, error) {
	// 生成6位随机验证码
	code := generateCode(6)

	// 调用互亿无线API
	resp, err := c.send(phone, code)
	if err != nil {
		return "", err
	}

	// 检查响应
	if resp.Code != 2 {
		c.log.Errorf("SMS send failed: code=%d, msg=%s, phone=%s", resp.Code, resp.Msg, phone)
		return "", fmt.Errorf("sms send failed: %s (code=%d)", resp.Msg, resp.Code)
	}

	c.log.Infof("SMS sent successfully: phone=%s, smsid=%s", phone, resp.SmsID)
	return code, nil
}

// send 调用互亿无线API发送短信
func (c *IhuyiClient) send(phone, content string) (*IhuyiResponse, error) {
	// 构建请求参数
	params := url.Values{}
	params.Set("account", c.account)
	params.Set("password", c.password)
	params.Set("mobile", phone)
	params.Set("content", content)
	params.Set("templateid", c.templateID)

	// 创建带超时的 HTTP 客户端（10秒超时）
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 发送HTTP请求
	resp, err := client.PostForm(c.apiURL, params)
	if err != nil {
		c.log.Errorf("SMS HTTP request failed: %v", err)
		return nil, fmt.Errorf("sms http request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	// 解析JSON响应
	var result IhuyiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response json failed: %w", err)
	}

	return &result, nil
}

// generateCode 生成指定长度的随机数字验证码
func generateCode(length int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	code := ""
	for i := 0; i < length; i++ {
		code += fmt.Sprintf("%d", r.Intn(10))
	}
	return code
}

// ValidatePhone 校验手机号格式（中国大陆11位）
func ValidatePhone(phone string) bool {
	if len(phone) != 11 {
		return false
	}
	// 检查是否全为数字
	for _, c := range phone {
		if c < '0' || c > '9' {
			return false
		}
	}
	// 检查是否以1开头
	return phone[0] == '1'
}
