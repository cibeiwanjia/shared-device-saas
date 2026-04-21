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
