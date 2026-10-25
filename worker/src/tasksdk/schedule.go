package tasksdk

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/czh0727/asy/taskutils/constant"
	"github.com/czh0727/asy/taskutils/rpc"
	"github.com/czh0727/asy/taskutils/rpc/model"
	"github.com/niuniumart/gosdk/redislock"
)

const DEFAULT_TIME_INTERVAL = 20

var (
	taskSvrHost string
	lockClient  *redislock.Client
)

// InitSvr 与原项目保持相同入口。
func InitSvr(
	taskServerHost string,
	lockServerHost string,
	lockPassword string,
) {
	taskSvrHost = taskServerHost
	lockClient = redislock.NewClient(
		lockServerHost,
		lockPassword,
	)
}

// TaskMgr 对应原项目的任务调度管理器。
type TaskMgr struct {
	IntervalTime  time.Duration
	TaskType      string
	ScheduleLimit int
}

// HoldOnce 本章的临时学习入口：
// 1. 读取任务调度配置
// 2. 获取同类任务的 Redis 锁
// 3. 调用 flowsvr 批量 Hold
// 4. 释放锁
// 5. 返回已领取任务，但本章不执行
//
// 下一章将把结果转换为 TaskIntf。
func (p *TaskMgr) HoldOnce(
	ctx context.Context,
) (tasks []*model.TaskData, err error) {
	if ctx == nil {
		return nil, errors.New("nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p == nil || p.TaskType == "" {
		return nil, errors.New("invalid task manager")
	}
	if taskSvrHost == "" || lockClient == nil {
		return nil, errors.New(
			"call InitSvr before HoldOnce",
		)
	}

	taskRPC := rpc.TaskRpc{
		Host: taskSvrHost,
	}

	// 1. 从 flowsvr 获取实时的任务类型配置。
	cfgResp, err := taskRPC.GetTaskScheduleCfgList()
	if err != nil {
		return nil, err
	}

	var cfg *model.TaskScheduleCfg

	for _, item := range cfgResp.ScheduleCfgList {
		if item != nil && item.TaskType == p.TaskType {
			cfg = item
			break
		}
	}

	if cfg == nil {
		return nil, fmt.Errorf(
			"schedule config for %q not found",
			p.TaskType,
		)
	}

	limit := cfg.ScheduleLimit
	if limit <= 0 {
		limit = model.SCHEDULE_TASK_LIMIT
	}

	// 2. 原项目使用 task_type 作为锁名。
	mutex := redislock.NewRedisLock(
		p.TaskType,
		lockClient,
		redislock.WithBlock(),
		redislock.WithWatchDogMode(),
		redislock.WithExpireSeconds(3),
	)

	// 3. 获取分布式锁。
	if err := mutex.Lock(ctx); err != nil {
		return nil, fmt.Errorf(
			"acquire redis lock: %w", err,
		)
	}

	// 一定要尝试释放锁，即使 HTTP Hold 返回错误。
	defer func() {
		unlockCtx, cancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cancel()

		if unlockErr := mutex.Unlock(unlockCtx); unlockErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf(
					"unlock redis lock: %w",
					unlockErr,
				),
			)
		}
	}()

	// 4. 持锁调用 flowsvr。
	result, err := taskRPC.HoldTasks(
		&model.HoldTasksReq{
			TaskType: p.TaskType,
			Limit:    limit,
		},
	)
	if err != nil {
		return nil, err
	}

	// 5. 原始服务端返回的是领取之前的快照。
	// Worker 将本地状态设置为 processing。
	tasks = result.TaskList

	for _, item := range tasks {
		if item != nil {
			item.Status = int(
				constant.TASK_STATUS_PROCESSING,
			)
		}
	}

	return tasks, nil
}
