package main

import (
	"fmt"

	"github.com/czh0727/asy/worker/src/config"
)

func main() {
	config.Init()

	fmt.Printf(
		"worker config OK: flowsvr=%s redis-lock=%s\n",
		config.Conf.FlowsvrAddr,
		config.Conf.RedisLockAddr,
	)

	// 后续章节：
	// 初始化 Redis 锁。
	// 初始化 TaskSDK。
	// 注册 LarkTask。
	// 启动 TaskMgr 调度循环。
}
