package data

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"shared-device-saas/app/user/internal/conf"
	"shared-device-saas/pkg/amap"
	"shared-device-saas/pkg/auth"
	"shared-device-saas/pkg/redis"
	"shared-device-saas/pkg/sms"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData,
	NewUserRepo,
	NewOrderRepo,
	NewWalletRepo,
	NewRechargeRepo,
	NewInventoryRepo,
	NewRedisClient,
	NewSMSClient,
	NewRedisBlacklist,
	NewStationRepo,
	NewAmapClient,
)

// Data 数据层
type Data struct {
	mongoClient   *mongo.Client
	mongoDatabase *mongo.Database
	sqlDB         *sql.DB
	redisClient   *redis.Client
}

// NewData 初始化数据层
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	helper := log.NewHelper(logger)
	helper.Info("Initializing data layer...")

	var mongoClient *mongo.Client
	var mongoDatabase *mongo.Database

	// MongoDB 连接
	mongoCfg := c.GetMongodb()
	if mongoCfg != nil {
		helper.Info("Connecting to MongoDB...")
		clientOptions := options.Client().ApplyURI(mongoCfg.Uri)

		// 支持认证配置
		if mongoCfg.Username != "" {
			clientOptions.SetAuth(options.Credential{
				Username:   mongoCfg.Username,
				Password:   mongoCfg.Password,
				AuthSource: mongoCfg.AuthSource,
			})
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		client, err := mongo.Connect(ctx, clientOptions)
		if err != nil {
			helper.Errorf("Failed to connect MongoDB: %v", err)
			return nil, nil, fmt.Errorf("mongodb connect: %w", err)
		}

		if err := client.Ping(ctx, nil); err != nil {
			helper.Errorf("Failed to ping MongoDB: %v", err)
			return nil, nil, fmt.Errorf("mongodb ping: %w", err)
		}

		mongoDatabase = client.Database(mongoCfg.Database)

		// 创建索引
		ensureIndexes(ctx, mongoDatabase, helper)

		helper.Infof("MongoDB connected: database=%s", mongoCfg.Database)
		mongoClient = client
	} else {
		helper.Warn("MongoDB config not found")
	}

	// MySQL 连接
	var sqlDB *sql.DB
	dbCfg := c.GetDatabase()
	if dbCfg != nil && dbCfg.Driver != "" && dbCfg.Source != "" {
		helper.Info("Connecting to MySQL...")
		db, err := sql.Open(dbCfg.Driver, dbCfg.Source)
		if err != nil {
			helper.Errorf("Failed to open MySQL: %v", err)
			return nil, nil, fmt.Errorf("mysql open: %w", err)
		}
		if err := db.PingContext(context.Background()); err != nil {
			helper.Errorf("Failed to ping MySQL: %v", err)
			return nil, nil, fmt.Errorf("mysql ping: %w", err)
		}
		sqlDB = db
		helper.Infof("MySQL connected: driver=%s", dbCfg.Driver)
	} else {
		helper.Warn("MySQL config not found")
	}

	cleanup := func() {
		helper.Info("Closing data layer connections...")
		if mongoClient != nil {
			if err := mongoClient.Disconnect(context.Background()); err != nil {
				helper.Errorf("Failed to disconnect MongoDB: %v", err)
			}
		}
		if sqlDB != nil {
			if err := sqlDB.Close(); err != nil {
				helper.Errorf("Failed to close MySQL: %v", err)
			}
		}
	}

	return &Data{
		mongoClient:   mongoClient,
		mongoDatabase: mongoDatabase,
		sqlDB:         sqlDB,
	}, cleanup, nil
}

// NewRedisClient 创建 Redis 客户端
func NewRedisClient(c *conf.Data, logger log.Logger) (*redis.Client, func(), error) {
	helper := log.NewHelper(logger)

	redisCfg := c.GetRedis()
	if redisCfg == nil {
		helper.Warn("Redis config not found, using in-memory fallback")
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

// NewSMSClient 创建 SMS 客户端
func NewSMSClient(c *conf.Data, logger log.Logger) *sms.IhuyiClient {
	helper := log.NewHelper(logger)

	smsCfg := c.GetSms()
	if smsCfg == nil {
		helper.Warn("SMS config not found")
		return nil
	}

	helper.Infof("SMS client initialized: api_url=%s", smsCfg.ApiUrl)
	return sms.NewIhuyiClient(
		smsCfg.ApiUrl,
		smsCfg.Account,
		smsCfg.Password,
		smsCfg.TemplateId,
		logger,
	)
}

// NewAmapClient 创建高德地图客户端
func NewAmapClient(c *conf.Data, logger log.Logger) *amap.Client {
	helper := log.NewHelper(logger)

	amapCfg := c.GetAmap()
	if amapCfg == nil || amapCfg.ApiKey == "" {
		helper.Warn("Amap config not found or api_key empty")
		return nil
	}

	helper.Info("Amap client initialized")
	return amap.NewClient(amapCfg.ApiKey, logger)
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

// GetSqlDB 获取 MySQL 连接
func (d *Data) GetSqlDB() *sql.DB {
	return d.sqlDB
}

// SetRedisClient 设置 Redis 客户端（用于 wire 注入）
func (d *Data) SetRedisClient(client *redis.Client) {
	d.redisClient = client
}

// ========================================
// MongoDB 索引管理（dev 分支）
// ========================================

type indexSpec struct {
	collection    string
	keys          bson.D
	unique        bool
	sparse        bool
	ttlSeconds    int32 // 0 表示无 TTL
}

func ensureIndexes(ctx context.Context, db *mongo.Database, helper *log.Helper) {
	indexes := []indexSpec{
		// ===== users =====
		{collection: "users", keys: bson.D{{Key: "phone", Value: 1}}, unique: true},
		{collection: "users", keys: bson.D{{Key: "username", Value: 1}}, unique: true},

		// ===== orders 已迁移到 MySQL，不再使用 MongoDB =====

		// ===== wallet_transactions =====
		{collection: "wallet_transactions", keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},

		// ===== recharge_orders =====
		{collection: "recharge_orders", keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
		{collection: "recharge_orders", keys: bson.D{{Key: "order_no", Value: 1}}, unique: true},

		// ===== jwt_sessions =====
		{collection: "jwt_sessions", keys: bson.D{{Key: "user_id", Value: 1}, {Key: "device_id", Value: 1}}},
		{collection: "jwt_sessions", keys: bson.D{{Key: "token_hash", Value: 1}}, unique: true},
		{collection: "jwt_sessions", keys: bson.D{{Key: "expires_at", Value: 1}}, ttlSeconds: 0}, // TTL 索引

		// ===== upload_files =====
		{collection: "upload_files", keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
		{collection: "upload_files", keys: bson.D{{Key: "file_key", Value: 1}}, unique: true},

		// ===== operation_logs =====
		{collection: "operation_logs", keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
		{collection: "operation_logs", keys: bson.D{{Key: "created_at", Value: 1}}, ttlSeconds: 90 * 86400}, // 90 天 TTL

		// ===== sms_codes =====
		{collection: "sms_codes", keys: bson.D{{Key: "phone", Value: 1}}},
		{collection: "sms_codes", keys: bson.D{{Key: "expires_at", Value: 1}}, ttlSeconds: 0},
	}

	for _, idx := range indexes {
		opt := options.Index()
		if idx.unique {
			opt.SetUnique(true)
		}
		if idx.sparse {
			opt.SetSparse(true)
		}
		if idx.ttlSeconds > 0 {
			opt.SetExpireAfterSeconds(idx.ttlSeconds)
		}

		_, err := db.Collection(idx.collection).Indexes().CreateOne(ctx, mongo.IndexModel{Keys: idx.keys, Options: opt})
		if err != nil {
			helper.Warnf("ensure index %s.%v: %v", idx.collection, idx.keys, err)
		}
	}
}
