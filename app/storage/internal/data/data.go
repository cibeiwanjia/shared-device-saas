package data

import (
	"context"
	"database/sql"
	"fmt"

	"shared-device-saas/app/storage/internal/conf"
	"shared-device-saas/pkg/redis"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"time"
)

var ProviderSet = wire.NewSet(
	NewData,
	NewRedisClient,
	NewCabinetRepo,
	NewCellRepo,
	NewOrderRepo,
	NewPricingRepo,
)

type Data struct {
	db    *sql.DB
	redis *redis.Client
}

func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	helper := log.NewHelper(logger)
	helper.Info("Initializing storage data layer...")

	var db *sql.DB
	dbCfg := c.GetDatabase()
	if dbCfg != nil && dbCfg.Driver != "" && dbCfg.Source != "" {
		helper.Info("Connecting to MySQL...")
		var err error
		db, err = sql.Open(dbCfg.Driver, dbCfg.Source)
		if err != nil {
			return nil, nil, fmt.Errorf("mysql open: %w", err)
		}
		if err := db.PingContext(context.Background()); err != nil {
			return nil, nil, fmt.Errorf("mysql ping: %w", err)
		}
		helper.Infof("MySQL connected: driver=%s", dbCfg.Driver)
	}

	cleanup := func() {
		helper.Info("Closing storage data resources...")
		if db != nil {
			db.Close()
		}
	}

	return &Data{db: db}, cleanup, nil
}

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
		"",
		0,
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

func (d *Data) DB() *sql.DB        { return d.db }
func (d *Data) Redis() *redis.Client { return d.redis }
