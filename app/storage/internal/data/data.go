package data

import (
	"context"
	"time"

	"shared-device-saas/app/storage/internal/conf"
	"shared-device-saas/pkg/auth"
	"shared-device-saas/pkg/redis"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData,
	NewExpressRepo,
	NewCourierRepo,
	NewZoneRepo,
	NewVerificationLogRepo,
	NewRedisClient,
	NewRedisBlacklist,
	// Phase 3 新增
	NewStationRepo,
	NewCabinetRepo,
	NewCabinetGridRepo,
)

// Data .
type Data struct {
	mongoClient   *mongo.Client
	mongoDatabase *mongo.Database
	redisClient   *redis.Client
}

// NewData .
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	helper := log.NewHelper(logger)
	helper.Info("Initializing data layer...")

	// MongoDB 连接
	mongoCfg := c.GetMongodb()
	var mongoClient *mongo.Client
	var mongoDatabase *mongo.Database

	if mongoCfg != nil {
		helper.Info("Connecting to MongoDB...")
		clientOptions := options.Client().ApplyURI(mongoCfg.Uri)
		client, err := mongo.Connect(context.Background(), clientOptions)
		if err != nil {
			helper.Errorf("Failed to connect MongoDB: %v", err)
			return nil, nil, err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err = client.Ping(ctx, nil); err != nil {
			helper.Errorf("Failed to ping MongoDB: %v", err)
			return nil, nil, err
		}

		helper.Infof("MongoDB connected: database=%s", mongoCfg.Database)
		mongoClient = client
		mongoDatabase = client.Database(mongoCfg.Database)
	} else {
		helper.Warn("MongoDB config not found")
	}

	cleanup := func() {
		helper.Info("Closing data layer connections...")
		if mongoClient != nil {
			if err := mongoClient.Disconnect(context.Background()); err != nil {
				helper.Errorf("Failed to disconnect MongoDB: %v", err)
			}
		}
	}

	return &Data{
		mongoClient:   mongoClient,
		mongoDatabase: mongoDatabase,
	}, cleanup, nil
}

// NewRedisClient 创建 Redis 客户端
func NewRedisClient(c *conf.Data, logger log.Logger) (*redis.Client, func(), error) {
	helper := log.NewHelper(logger)

	redisCfg := c.GetRedis()
	if redisCfg == nil {
		helper.Warn("Redis config not found")
		return nil, func() {}, nil
	}

	readTimeout := 200 * time.Millisecond
	if redisCfg.ReadTimeout != nil {
		readTimeout = redisCfg.ReadTimeout.AsDuration()
	}

	writeTimeout := 200 * time.Millisecond
	if redisCfg.WriteTimeout != nil {
		writeTimeout = redisCfg.WriteTimeout.AsDuration()
	}

	client, err := redis.NewClient(
		redisCfg.Addr,
		redisCfg.Password,
		int(redisCfg.Db),
		readTimeout,
		writeTimeout,
		logger,
	)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		helper.Info("Closing Redis connection...")
		if client != nil {
			client.Close()
		}
	}

	return client, cleanup, nil
}

// GetCollection 获取 MongoDB 集合
func (d *Data) GetCollection(name string) *mongo.Collection {
	if d.mongoDatabase == nil {
		return nil
	}
	return d.mongoDatabase.Collection(name)
}

// GetRedisClient 获取 Redis 客户端
func (d *Data) GetRedisClient() *redis.Client {
	return d.redisClient
}

// SetRedisClient 设置 Redis 客户端（用于 wire 注入）
func (d *Data) SetRedisClient(client *redis.Client) {
	d.redisClient = client
}

// NewRedisBlacklist 创建 Redis Token 黑名单
func NewRedisBlacklist(redisClient *redis.Client, logger log.Logger) auth.Blacklist {
	helper := log.NewHelper(logger)
	if redisClient == nil {
		helper.Warn("Redis client not available, blacklist will not work")
		return nil
	}
	helper.Info("Redis blacklist initialized")
	return auth.NewRedisBlacklistAdapter(redisClient)
}