package rtm

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/czh0727/asy/flowsvr/src/cache"
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

// subTableProcessing 自动检查是否需要分表。
func (p *TaskRuntime) subTableProcessing() {
	if db.DB == nil {
		return
	}

	maxRows := config.Conf.Task.TableMaxRows
	if maxRows <= 0 {
		return
	}

	// 1. 获取所有任务类型的分表位置。
	taskPosList, err :=
		db.TaskPosNsp.GetTaskPosList(db.DB)

	if err != nil {
		log.Printf("get task positions failed: %v", err)
		return
	}

	// 2. 遍历每种任务类型。
	for _, taskPos := range taskPosList {
		if taskPos == nil {
			continue
		}

		taskType := taskPos.TaskType

		endPos := strconv.Itoa(
			taskPos.ScheduleEndPos,
		)

		// 3. 统计当前写入表的数据量。
		count, err := db.TaskNsp.GetAllTaskCount(
			db.DB,
			taskType,
			endPos,
		)

		if err != nil {
			log.Printf(
				"get task count failed: %v",
				err,
			)
			continue
		}

		// 4. 未达到阈值，不分表。
		if count < maxRows {
			continue
		}

		// 5. 创建下一张任务表。
		nextPos := strconv.Itoa(
			taskPos.ScheduleEndPos + 1,
		)

		err = db.TaskNsp.CreateTable(
			db.DB,
			taskType,
			nextPos,
		)

		if err != nil {
			log.Printf(
				"create next task table failed: %v",
				err,
			)
			continue
		}

		// 6. CAS 推进写入位置。
		advanced, err :=
			db.TaskPosNsp.AdvanceScheduleEndPos(
				db.DB,
				taskType,
				taskPos.ScheduleEndPos,
			)

		if err != nil {
			log.Printf(
				"advance end pos failed: %v",
				err,
			)
			continue
		}

		if !advanced {
			continue
		}

		// 7. 更新 Redis 写入位置缓存。
		if err := cache.SetTaskWritePos(
			taskType,
			taskPos.ScheduleEndPos+1,
		); err != nil {
			log.Printf(
				"update task write pos cache failed: %v",
				err,
			)
		}

		log.Printf(
			"task table split success: type=%s old=%d new=%d",
			taskType,
			taskPos.ScheduleEndPos,
			taskPos.ScheduleEndPos+1,
		)
	}
}

// UpdateSchedulePosProcessing
// 当旧表所有任务进入终态后，推进调度读取位置。
func (p *TaskRuntime) UpdateSchedulePosProcessing() {
	if db.DB == nil {
		return
	}

	// 1. 获取所有任务类型的位置。
	taskPosList, err :=
		db.TaskPosNsp.GetTaskPosList(db.DB)

	if err != nil {
		log.Printf("get task position list failed: %v", err)
		return
	}

	for _, taskPos := range taskPosList {
		if taskPos == nil {
			continue
		}

		// 2. 没有旧表需要处理。
		if taskPos.ScheduleBeginPos >=
			taskPos.ScheduleEndPos {
			continue
		}

		taskType := taskPos.TaskType

		beginPos := strconv.Itoa(
			taskPos.ScheduleBeginPos,
		)

		// 3. 获取当前调度表的终态任务数量。
		finishNum, err :=
			db.TaskNsp.GetFinishTaskCount(
				db.DB,
				taskType,
				beginPos,
			)

		if err != nil {
			log.Printf(
				"get finish task count failed: %v",
				err,
			)
			continue
		}

		// 4. 获取当前调度表的任务总量。
		count, err :=
			db.TaskNsp.GetAllTaskCount(
				db.DB,
				taskType,
				beginPos,
			)

		if err != nil {
			log.Printf(
				"get all task count failed: %v",
				err,
			)
			continue
		}

		// 5. 还有任务没有完成，不能推进。
		if finishNum != count {
			continue
		}

		// 6. 推进 begin_pos。
		advanced, err :=
			db.TaskPosNsp.AdvanceScheduleBeginPos(
				db.DB,
				taskType,
				taskPos.ScheduleBeginPos,
			)

		if err != nil {
			log.Printf(
				"advance begin pos failed: %v",
				err,
			)
			continue
		}

		if advanced {
			log.Printf(
				"schedule begin pos advanced: type=%s pos=%d",
				taskType,
				taskPos.ScheduleBeginPos+1,
			)
		}
	}
}
func (p *TaskRuntime) RunContext(ctx context.Context) {
	if ctx == nil || config.Conf == nil {
		return
	}

	type periodicJob struct {
		name     string
		interval int
		run      func()
	}

	jobs := []periodicJob{
		{
			name:     "timeout recovery",
			interval: config.Conf.Task.LongProcessInterval,
			run:      p.dealTimeoutProcessing,
		},
		{
			name:     "sub table",
			interval: config.Conf.Task.SplitInterval,
			run:      p.subTableProcessing,
		},
		{
			name:     "schedule position",
			interval: config.Conf.Task.MoveInterval,
			run:      p.UpdateSchedulePosProcessing,
		},
	}

	var wg sync.WaitGroup

	for _, job := range jobs {
		if job.interval <= 0 {
			log.Printf(
				"skip runtime job %s: invalid interval",
				job.name,
			)
			continue
		}

		wg.Add(1)

		go func(job periodicJob) {
			defer wg.Done()

			ticker := time.NewTicker(
				time.Duration(job.interval) * time.Second,
			)
			defer ticker.Stop()

			log.Printf(
				"runtime job started: %s",
				job.name,
			)

			for {
				select {
				case <-ctx.Done():
					return

				case <-ticker.C:
					job.run()
				}
			}
		}(job)
	}

	wg.Wait()
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
