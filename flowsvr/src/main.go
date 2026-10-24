package main

import (
	"fmt"

	"github.com/czh0727/asy/flowsvr/src/config"
)

func main() {
	config.Init()

	fmt.Printf(
		"flowsvr config OK: port=%d mysql=%s redis=%s\n",
		config.Conf.Common.Port,
		config.Conf.MySQL.Url,
		config.Conf.Redis.Url,
	)

	// 第二章：初始化 MySQL、Redis。
	// 后续章节：注册 HTTP 路由、启动任务治理。
}
