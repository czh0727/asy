package initialize

import (
	"fmt"

	"github.com/czh0727/asy/flowsvr/src/cache"
	"github.com/czh0727/asy/flowsvr/src/db"
)

// InitResource 初始化 flowsvr 依赖的外部资源。
func InitResource() {
	// 第一步：MySQL
	if err := db.InitDB(); err != nil {
		panic(
			fmt.Sprintf("InitDB err: %v", err),
		)
	}

	// 第二步：Redis
	if err := cache.InitCache(); err != nil {
		// 避免 Redis 初始化失败时留下 MySQL 连接。
		if db.DB != nil {
			_ = db.DB.Close()
		}

		panic(
			fmt.Sprintf("InitCache err: %v", err),
		)
	}
}
