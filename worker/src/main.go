package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/czh0727/asy/worker/src/config"
	"github.com/czh0727/asy/worker/src/tasksdk"
)

func main() {
	mode := flag.String(
		"mode",
		"schedule",
		"schedule | once | hold-once",
	)
	taskType := flag.String(
		"type",
		"lark",
		"task type",
	)
	flag.Parse()

	// 第一章：加载 Worker 配置。
	config.Init()

	// 第五章：初始化 Redis 锁和 flowsvr 地址。
	tasksdk.InitSvr(
		config.Conf.FlowsvrAddr,
		config.Conf.RedisLockAddr,
		config.Conf.RedisLockPassword,
	)

	// 第六章：注册原始 LarkTask 处理器。
	if err := tasksdk.RegisterHandler(
		&tasksdk.TaskHandler{
			TaskType: "lark",
			NewProc: func() tasksdk.TaskIntf {
				return new(LarkTask)
			},
		},
	); err != nil {
		log.Fatal(err)
	}

	if *taskType != "lark" {
		log.Fatalf(
			"task type %q has no registered business handler",
			*taskType,
		)
	}

	manager := &tasksdk.TaskMgr{
		TaskType: *taskType,
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	switch *mode {
	case "once":
		// 调试模式：完整执行一轮。
		if err := manager.RunOnce(ctx); err != nil {
			log.Fatal(err)
		}
		log.Print("one schedule batch completed")

	case "hold-once":
		// 保留第五章功能，但只领取不执行。
		tasks, err := manager.HoldOnce(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("held %d tasks (not executed)\n", len(tasks))

	case "schedule":
		// 正常模式：长期运行原项目任务调度循环。
		log.Print("Asyncflow Worker started")

		err := manager.ScheduleContext(ctx)
		if err != nil &&
			!errors.Is(err, context.Canceled) {
			log.Fatal(err)
		}

		log.Print("Asyncflow Worker stopped")

	default:
		log.Fatalf("unknown mode %q", *mode)
	}
}
