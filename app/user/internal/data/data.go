package data

import (
	"context"
	"fmt"
	"time"

	"shared-device-saas/app/user/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData,
	NewOrderRepo,
	NewWalletRepo,
	NewRechargeRepo,
)

// Data .
type Data struct {
	mdb *mongo.Database
}

// NewData .
func NewData(c *conf.Data) (*Data, func(), error) {
	var cleanup func() = func() {}

	// 初始化 MongoDB
	if c.Mongo != nil {
		clientOpts := options.Client().ApplyURI(c.Mongo.GetUri())

		if c.Mongo.GetUsername() != "" {
			clientOpts.SetAuth(options.Credential{
				Username:   c.Mongo.GetUsername(),
				Password:   c.Mongo.GetPassword(),
				AuthSource: c.Mongo.GetAuthSource(),
			})
		}

		if c.Mongo.GetMaxPoolSize() > 0 {
			clientOpts.SetMaxPoolSize(uint64(c.Mongo.GetMaxPoolSize()))
		}
		if c.Mongo.GetMinPoolSize() > 0 {
			clientOpts.SetMinPoolSize(uint64(c.Mongo.GetMinPoolSize()))
		}
		if c.Mongo.GetConnectTimeout() != nil {
			clientOpts.SetConnectTimeout(c.Mongo.GetConnectTimeout().AsDuration())
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		client, err := mongo.Connect(clientOpts)
		if err != nil {
			return nil, nil, fmt.Errorf("mongodb connect: %w", err)
		}

		if err := client.Ping(ctx, nil); err != nil {
			return nil, nil, fmt.Errorf("mongodb ping: %w", err)
		}

		db := client.Database(c.Mongo.GetDatabase())

		ensureIndexes(ctx, db)

		cleanup = func() {
			log.Info("closing the data resources")
			_ = client.Disconnect(context.Background())
		}

		log.Infof("mongodb connected: %s/%s", c.Mongo.GetUri(), c.Mongo.GetDatabase())
		return &Data{mdb: db}, cleanup, nil
	}

	// MongoDB 未配置时返回空 Data（桩模式）
	return &Data{}, cleanup, nil
}

// MongoCollection 获取 MongoDB 集合
func (d *Data) MongoCollection(name string) *mongo.Collection {
	if d.mdb == nil {
		return nil
	}
	return d.mdb.Collection(name)
}

type indexSpec struct {
	collection string
	keys       bson.D
	unique     bool
}

func ensureIndexes(ctx context.Context, db *mongo.Database) {
	indexes := []indexSpec{
		{collection: "orders", keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
		{collection: "orders", keys: bson.D{{Key: "order_no", Value: 1}}, unique: true},
		{collection: "wallet_transactions", keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
		{collection: "recharge_orders", keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
		{collection: "recharge_orders", keys: bson.D{{Key: "order_no", Value: 1}}, unique: true},
		{collection: "jwt_sessions", keys: bson.D{{Key: "user_id", Value: 1}, {Key: "device_id", Value: 1}}},
		{collection: "jwt_sessions", keys: bson.D{{Key: "token_hash", Value: 1}}, unique: true},
		{collection: "upload_files", keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
	}

	for _, idx := range indexes {
		opt := options.Index().SetUnique(idx.unique)
		_, err := db.Collection(idx.collection).Indexes().CreateOne(ctx, mongo.IndexModel{Keys: idx.keys, Options: opt})
		if err != nil {
			log.Warnf("ensure index %s.%v: %v", idx.collection, idx.keys, err)
		}
	}
}
