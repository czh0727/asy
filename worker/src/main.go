package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/czh0727/asy/worker/src/config"
	"github.com/czh0727/asy/worker/src/tasksdk"
)

func main() {
	holdOnce := flag.Bool(
		"hold-once", false,
		"hold one batch of tasks for learning",
	)

	taskType := flag.String(
		"type", "lark",
		"task type to hold",
	)

	flag.Parse()

	config.Init()

	if !*holdOnce {
		fmt.Printf(
			"worker config OK: flowsvr=%s redis-lock=%s\n",
			config.Conf.FlowsvrAddr,
			config.Conf.RedisLockAddr,
		)
		return
	}

	// 连接 Redis 锁服务。
	tasksdk.InitSvr(
		config.Conf.FlowsvrAddr,
		config.Conf.RedisLockAddr,
		config.Conf.RedisLockPassword,
	)

	manager := &tasksdk.TaskMgr{
		TaskType: *taskType,
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	tasks, err := manager.HoldOnce(ctx)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf(
		"HoldOnce finished: task_type=%s count=%d",
		*taskType,
		len(tasks),
	)

	for _, task := range tasks {
		log.Printf(
			"held task: id=%s status=%d stage=%s",
			task.TaskId,
			task.Status,
			task.TaskStage,
		)
	}

	log.Print(
		"chapter 5 only holds tasks; " +
			"TaskIntf execution will be added later",
	)
}
