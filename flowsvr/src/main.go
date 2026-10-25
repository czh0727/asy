package main

import (
	"fmt"
	"log"

	"github.com/czh0727/asy/flowsvr/src/cache"
	"github.com/czh0727/asy/flowsvr/src/config"
	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/czh0727/asy/flowsvr/src/initialize"
	"github.com/gin-gonic/gin"
)

func main() {
	// 1. 加载配置
	config.Init()

	// 2. 初始化 MySQL 和 Redis
	initialize.InitResource()

	defer func() {
		_ = cache.Close()

		if db.DB != nil {
			_ = db.DB.Close()
		}
	}()

	// 3. 初始化 HTTP Router
	router := gin.Default()

	// 4. 注册原项目接口
	initialize.RegisterRouter(router)

	// 5. 启动服务
	addr := fmt.Sprintf(
		"127.0.0.1:%d",
		config.Conf.Common.Port,
	)

	log.Printf("flowsvr listening on %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}
