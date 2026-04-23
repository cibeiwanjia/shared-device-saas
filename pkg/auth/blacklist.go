package auth

import (
	"context"
	"fmt"
	"time"
)

// Blacklist Token 黑名单接口（Redis 实现）
type Blacklist interface {
	// Add 将 jti 加入黑名单，TTL 为 Token 剩余有效期
	Add(ctx context.Context, jti string, ttl time.Duration) error
	// IsBlacklisted 检查 jti 是否在黑名单中
	IsBlacklisted(ctx context.Context, jti string) (bool, error)
}

// RedisBlacklist 基于 Redis 的 Token 黑名单实现
type RedisBlacklist struct {
	client RedisClient
}

// RedisClient 最小化的 Redis 客户端接口
type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Exists(ctx context.Context, key string) (bool, error)
}

// NewRedisBlacklist 创建 Redis 黑名单
func NewRedisBlacklist(client RedisClient) *RedisBlacklist {
	return &RedisBlacklist{client: client}
}

// Add 将 jti 加入黑名单
func (b *RedisBlacklist) Add(ctx context.Context, jti string, ttl time.Duration) error {
	key := fmt.Sprintf("jwt:blacklist:%s", jti)
	return b.client.Set(ctx, key, "1", ttl)
}

// IsBlacklisted 检查 jti 是否在黑名单中
func (b *RedisBlacklist) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	key := fmt.Sprintf("jwt:blacklist:%s", jti)
	return b.client.Exists(ctx, key)
}

// RedisBlacklistAdapter 适配器 - 将 pkg/redis.Client 转换为 RedisClient 接口
type RedisBlacklistAdapter struct {
	client *redisClientAdapter
}

// redisClientAdapter 内部适配器
type redisClientAdapter struct {
	setFunc    func(ctx context.Context, key string, value string, ttl time.Duration) error
	existsFunc func(ctx context.Context, key string) (bool, error)
}

func (a *redisClientAdapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// 将 interface{} 转换为 string
	strValue, ok := value.(string)
	if !ok {
		strValue = "1" // 默认值
	}
	return a.setFunc(ctx, key, strValue, ttl)
}

func (a *redisClientAdapter) Exists(ctx context.Context, key string) (bool, error) {
	return a.existsFunc(ctx, key)
}

// NewRedisBlacklistAdapter 创建适配器（接收实际的 redis.Client）
// 这里的参数类型声明为 interface{} 是为了避免循环导入
func NewRedisBlacklistAdapter(redisClient interface{}) *RedisBlacklistAdapter {
	// 类型断言：假设 redisClient 有 Set 和 Exists 方法
	// 实际类型是 pkg/redis.Client，但我们不能直接导入它（循环导入）
	type redisClientLike interface {
		Set(ctx context.Context, key string, value string, expiration time.Duration) error
		Exists(ctx context.Context, key string) (bool, error)
	}

	if client, ok := redisClient.(redisClientLike); ok {
		adapter := &redisClientAdapter{
			setFunc:    client.Set,
			existsFunc: client.Exists,
		}
		return &RedisBlacklistAdapter{client: adapter}
	}
	return nil
}

// Add 将 jti 加入黑名单
func (b *RedisBlacklistAdapter) Add(ctx context.Context, jti string, ttl time.Duration) error {
	key := fmt.Sprintf("jwt:blacklist:%s", jti)
	return b.client.Set(ctx, key, "1", ttl)
}

// IsBlacklisted 检查 jti 是否在黑名单中
func (b *RedisBlacklistAdapter) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	key := fmt.Sprintf("jwt:blacklist:%s", jti)
	return b.client.Exists(ctx, key)
}
