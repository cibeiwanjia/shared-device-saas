package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

// Client Redis客户端封装
type Client struct {
	client *redis.Client
	log    *log.Helper
}

// NewClient 创建Redis客户端
func NewClient(addr, password string, db int, readTimeout, writeTimeout time.Duration, logger log.Logger) (*Client, error) {
	helper := log.NewHelper(logger)

	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		helper.Errorf("Failed to connect Redis: %v", err)
		return nil, fmt.Errorf("failed to connect redis: %w", err)
	}

	helper.Infof("Redis connected successfully: addr=%s, db=%d", addr, db)
	return &Client{client: rdb, log: helper}, nil
}

// Close 关闭连接
func (c *Client) Close() error {
	return c.client.Close()
}

// Set 设置键值（带过期时间）
func (c *Client) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

// Get 获取键值
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	result, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // 键不存在返回空字符串，不报错
	}
	return result, err
}

// Del 删除键
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// Exists 检查键是否存在
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	result, err := c.client.Exists(ctx, key).Result()
	return result > 0, err
}

// Expire 设置键过期时间
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.client.Expire(ctx, key, expiration).Err()
}

// TTL 获取键剩余过期时间
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, key).Result()
}

// Incr 自增
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

// SetNX 设置键值（不存在时才设置）
func (c *Client) SetNX(ctx context.Context, key string, value string, expiration time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, value, expiration).Result()
}

// GetClient 获取原始Redis客户端（用于复杂操作）
func (c *Client) GetClient() *redis.Client {
	return c.client
}

// ========================================
// SMS 验证码相关 Key 操作
// ========================================

// SMSKeyPrefix 短信验证码相关 Key 前缀
const (
	SMSCodeKeyPrefix     = "sms:code:"      // 验证码
	SMSCooldownKeyPrefix = "sms:cooldown:"  // 发送冷却（60秒）
	SMSCountKeyPrefix    = "sms:count:"     // 日发送次数
)

// SMSCodeTTL 验证码有效期（5分钟）
const SMSCodeTTL = 5 * time.Minute

// SMSCooldownTTL 发送冷却时间（60秒）
const SMSCooldownTTL = 60 * time.Second

// SMSCountTTL 日发送次数统计有效期（24小时）
const SMSCountTTL = 24 * time.Hour

// SetSMSCode 设置短信验证码
func (c *Client) SetSMSCode(ctx context.Context, phone string, code string) error {
	key := SMSCodeKeyPrefix + phone
	return c.Set(ctx, key, code, SMSCodeTTL)
}

// GetSMSCode 获取短信验证码
func (c *Client) GetSMSCode(ctx context.Context, phone string) (string, error) {
	key := SMSCodeKeyPrefix + phone
	return c.Get(ctx, key)
}

// DelSMSCode 删除短信验证码（验证通过后）
func (c *Client) DelSMSCode(ctx context.Context, phone string) error {
	key := SMSCodeKeyPrefix + phone
	return c.Del(ctx, key)
}

// CheckSMSCooldown 检查短信发送冷却（返回 true 表示在冷却期内）
func (c *Client) CheckSMSCooldown(ctx context.Context, phone string) (bool, error) {
	key := SMSCooldownKeyPrefix + phone
	return c.Exists(ctx, key)
}

// SetSMSCooldown 设置短信发送冷却
func (c *Client) SetSMSCooldown(ctx context.Context, phone string) error {
	key := SMSCooldownKeyPrefix + phone
	return c.Set(ctx, key, "1", SMSCooldownTTL)
}

// GetSMSCount 获取今日发送次数
func (c *Client) GetSMSCount(ctx context.Context, phone string) (int64, error) {
	// 使用日期作为 Key 的一部分
	dateKey := time.Now().Format("2006-01-02")
	key := SMSCountKeyPrefix + phone + ":" + dateKey

	count, err := c.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	if count == "" {
		return 0, nil
	}

	// 转换为数字
	var result int64
	fmt.Sscanf(count, "%d", &result)
	return result, nil
}

// IncrSMSCount 增加今日发送次数
func (c *Client) IncrSMSCount(ctx context.Context, phone string) (int64, error) {
	dateKey := time.Now().Format("2006-01-02")
	key := SMSCountKeyPrefix + phone + ":" + dateKey

	count, err := c.Incr(ctx, key)
	if err != nil {
		return 0, err
	}

	// 第一次设置过期时间
	if count == 1 {
		c.Expire(ctx, key, SMSCountTTL)
	}

	return count, nil
}

// ========================================
// 密码锁定相关 Key 操作
// ========================================

const (
	PwdErrKeyPrefix  = "pwd:err:"  // 密码错误计数
	PwdLockKeyPrefix = "pwd:lock:" // 密码锁定
)

const PwdLockTTL = 15 * time.Minute // 密码锁定时间（15分钟）

// GetPwdErrCount 获取密码错误次数
func (c *Client) GetPwdErrCount(ctx context.Context, phone string) (int64, error) {
	key := PwdErrKeyPrefix + phone
	count, err := c.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	if count == "" {
		return 0, nil
	}
	var result int64
	fmt.Sscanf(count, "%d", &result)
	return result, nil
}

// IncrPwdErrCount 增加密码错误次数
func (c *Client) IncrPwdErrCount(ctx context.Context, phone string) (int64, error) {
	key := PwdErrKeyPrefix + phone
	count, err := c.Incr(ctx, key)
	if err != nil {
		return 0, err
	}
	// 设置15分钟过期
	if count == 1 {
		c.Expire(ctx, key, PwdLockTTL)
	}
	return count, nil
}

// DelPwdErrCount 清除密码错误计数（登录成功后）
func (c *Client) DelPwdErrCount(ctx context.Context, phone string) error {
	key := PwdErrKeyPrefix + phone
	return c.Del(ctx, key)
}

// CheckPwdLocked 检查密码是否被锁定
func (c *Client) CheckPwdLocked(ctx context.Context, phone string) (bool, error) {
	key := PwdLockKeyPrefix + phone
	return c.Exists(ctx, key)
}

// SetPwdLock 设置密码锁定
func (c *Client) SetPwdLock(ctx context.Context, phone string) error {
	key := PwdLockKeyPrefix + phone
	return c.Set(ctx, key, "locked", PwdLockTTL)
}

// ========================================
// Token Session 相关 Key 操作
// ========================================

const (
	SessionKeyPrefix   = "session:"    // Token会话
	TokenBlackKeyPrefix = "token:black:" // Token黑名单
)

// SetSession 设置Token会话
func (c *Client) SetSession(ctx context.Context, sessionID string, userID string, ttl time.Duration) error {
	key := SessionKeyPrefix + sessionID
	return c.Set(ctx, key, userID, ttl)
}

// GetSession 获取Token会话
func (c *Client) GetSession(ctx context.Context, sessionID string) (string, error) {
	key := SessionKeyPrefix + sessionID
	return c.Get(ctx, key)
}

// DelSession 删除Token会话
func (c *Client) DelSession(ctx context.Context, sessionID string) error {
	key := SessionKeyPrefix + sessionID
	return c.Del(ctx, key)
}

// SetTokenBlack 将Token加入黑名单
func (c *Client) SetTokenBlack(ctx context.Context, jti string, ttl time.Duration) error {
	key := TokenBlackKeyPrefix + jti
	return c.Set(ctx, key, "1", ttl)
}

// CheckTokenBlack 检查Token是否在黑名单中
func (c *Client) CheckTokenBlack(ctx context.Context, jti string) (bool, error) {
	key := TokenBlackKeyPrefix + jti
	return c.Exists(ctx, key)
}