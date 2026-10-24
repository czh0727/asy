
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/czh0727/asy/flowsvr/src/config"
	"github.com/niuniumart/gosdk/goredis"
)

var (
	rdb        *goredis.RedisCli
	prefix     = "asyncflow_czh_"
	expireTime = 24 * time.Hour
)

// InitCache 初始化 Redis 客户端。
func InitCache() error {
	if config.Conf == nil {
		return fmt.Errorf("config is not initialized")
	}

	// 使用原项目 gosdk 的 Redis 工厂。
	goredis.Factory.MaxIdleConn = config.Conf.Redis.MaxIdle
	goredis.Factory.MaxConn = config.Conf.Redis.MaxActive
	goredis.Factory.IdleTimeout =
		time.Duration(config.Conf.Redis.IdleTimeout) *
			time.Second

	cli, err := goredis.Factory.CreateRedisCli(
		config.Conf.Redis.Auth,
		config.Conf.Redis.Url,
	)
	if err != nil {
		return fmt.Errorf("create redis client: %w", err)
	}

	rdb = cli

	// 防止重复初始化时把过期时间反复相乘。
	expireTime = 24 * time.Hour

	if config.Conf.Redis.CacheTimeoutDay > 0 {
		expireTime *= time.Duration(
			config.Conf.Redis.CacheTimeoutDay,
		)
	}

	return nil
}

// Ping 用于验证 Redis 当前是否可用。
// 这是我们为本章集成测试增加的小辅助方法。
func Ping(ctx context.Context) error {
	if rdb == nil {
		return fmt.Errorf("redis client is not initialized")
	}

	return rdb.RedisPool.Ping(ctx).Err()
}

// Close 释放 Redis 连接池。
func Close() error {
	if rdb == nil {
		return nil
	}

	err := rdb.RedisPool.Close()
	rdb = nil
	return err
}
