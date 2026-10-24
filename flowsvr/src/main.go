
package main

import (
	"fmt"

	"github.com/czh0727/asy/flowsvr/src/cache"
	"github.com/czh0727/asy/flowsvr/src/config"
	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/czh0727/asy/flowsvr/src/initialize"
)

func main() {
	// 第一章：初始化配置。
	config.Init()

	// 第二章：初始化 MySQL、Redis。
	initialize.InitResource()

	// 本章的程序只是资源初始化验证入口。
	defer func() {
		if db.DB != nil {
			_ = db.DB.Close()
		}
		_ = cache.Close()
	}()

	fmt.Println("flowsvr MySQL and Redis initialized successfully")

	// 后续章节：
	// rtm.TaskRuntime.Run()
	// initialize.RegisterRouter(router)
	// 启动 HTTP Server
}
