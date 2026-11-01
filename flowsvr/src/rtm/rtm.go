
package rtm

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/czh0727/asy/flowsvr/src/config"
	"github.com/czh0727/asy/flowsvr/src/db"
)

const (
	// 每次最多检查1000条超时任务。
	TIMEOUT_TASK_LIMIT = 1000

	// 默认检查间隔。
	DEFAULT_LONG_PROCESS_INTERVAL = 10
)

// TaskRuntime 负责服务端任务治理。
type TaskRuntime struct {
}

// Run 启动任务治理。
func (p *TaskRuntime) Run() {
	go p.RunContext(context.Background())
}

// RunContext 可通过 context 控制治理协程生命周期。
func (p *TaskRuntime) RunContext(ctx context.Context) {
	if ctx == nil {
		log.Println("task runtime context is nil")
		return
	}

	if config.Conf == nil {
		log.Println("task runtime config is nil")
		return
	}

	interval := config.Conf.Task.LongProcessInterval

	if interval <= 0 {
		interval = DEFAULT_LONG_PROCESS_INTERVAL
	}

	ticker := time.NewTicker(
		time.Duration(interval) * time.Second,
	)
	defer ticker.Stop()

	log.Printf(
		"task runtime started, interval=%ds",
		interval,
	)

	for {
		select {
		case <-ctx.Done():
			log.Println("task runtime stopped")
			return

		case <-ticker.C:
			p.dealTimeoutProcessing()
		}
	}
}

// dealTimeoutProcessing 遍历所有任务类型，处理超时任务。
func (p *TaskRuntime) dealTimeoutProcessing() {
	if db.DB == nil {
		log.Println("task runtime: mysql is not initialized")
		return
	}

	// 1. 获取所有任务类型的调度配置。
	taskTypeCfgList, err :=
		db.TaskTypeCfgNsp.GetTaskTypeCfgList(db.DB)

	if err != nil {
		log.Printf(
			"get task type config failed: %v",
			err,
		)
		return
	}

	// 2. 遍历每一种任务类型。
	for _, taskCfg := range taskTypeCfgList {
		if taskCfg == nil {
			continue
		}

		p.dealTimeoutProcessingWithType(taskCfg)
	}
}

// dealTimeoutProcessingWithType
// 处理指定任务类型的超时任务。
func (p *TaskRuntime) dealTimeoutProcessingWithType(
	taskCfg *db.TaskScheduleCfg,
) {

	// 1. 获取当前任务类型的分表位置。
	taskPos, err := db.TaskPosNsp.GetTaskPos(
		db.DB,
		taskCfg.TaskType,
	)

	if err != nil {
		log.Printf(
			"get task position failed: %v",
			err,
		)
		return
	}

	// 2. 确定最大执行时间。
	maxProcessTime := getMaxProcessTime(
		taskCfg,
		config.Conf.Task.MaxProcessTime,
	)

	if maxProcessTime <= 0 {
		log.Printf(
			"invalid max process time: taskType=%s",
			taskCfg.TaskType,
		)
		return
	}

	// 3. 查询超时的 processing 任务。
	taskList, err :=
		db.TaskNsp.GetLongTimeProcessing(
			db.DB,
			taskCfg.TaskType,
			fmt.Sprintf(
				"%d",
				taskPos.ScheduleBeginPos,
			),
			maxProcessTime,
			TIMEOUT_TASK_LIMIT,
		)

	if err != nil {
		log.Printf(
			"get long time processing failed: %v",
			err,
		)
		return
	}

	if len(taskList) == 0 {
		return
	}

	log.Printf(
		"found %d timeout tasks, taskType=%s",
		len(taskList),
		taskCfg.TaskType,
	)

	// 4. 根据重试次数决定恢复还是失败。
	for _, task := range taskList {
		if task == nil {
			continue
		}

		if shouldFailTimeoutTask(
			task.CrtRetryNum,
			taskCfg.MaxRetryNum,
		) {

			// 超过最大重试次数，直接失败。
			err = db.TaskNsp.SetStatus(
				db.DB,
				task.TaskId,
				db.TASK_STATUS_FAILED,
			)

			if err != nil {
				log.Printf(
					"set timeout task failed: task=%s err=%v",
					task.TaskId,
					err,
				)
			}

			continue
		}

		// 未超过最大重试次数：
		// 重新设置为 pending，重试次数加一。
		err = db.TaskNsp.SetStatusAndRetryNumIncrement(
			db.DB,
			task.TaskId,
			db.TASK_STATUS_PENDING,
		)

		if err != nil {
			log.Printf(
				"recover timeout task failed: task=%s err=%v",
				task.TaskId,
				err,
			)
		}
	}
}

// getMaxProcessTime 优先使用任务类型自己的最大执行时间。
func getMaxProcessTime(
	taskCfg *db.TaskScheduleCfg,
	defaultMaxProcessTime int64,
) int64 {

	if taskCfg != nil &&
		taskCfg.MaxProcessingTime > 0 {
		return taskCfg.MaxProcessingTime
	}

	return defaultMaxProcessTime
}

// shouldFailTimeoutTask 判断超时任务是否已经耗尽重试次数。
func shouldFailTimeoutTask(
	currentRetryNum int,
	maxRetryNum int,
) bool {
	return currentRetryNum >= maxRetryNum
}
