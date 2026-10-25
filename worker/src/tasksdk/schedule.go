package tasksdk

import (
	"context"
	"errors"
	"fmt"
	"github.com/czh0727/asy/taskutils/constant"
	"github.com/czh0727/asy/taskutils/rpc"
	"github.com/czh0727/asy/taskutils/rpc/model"
	"github.com/google/uuid"
	"log"
	"math/rand"
	"runtime/debug"
	"sync"
	"time"

	"github.com/niuniumart/gosdk/redislock"
)

const (
	DEFAULT_TIME_INTERVAL = 20
	MAX_ERR_MSG_LEN       = 256
)

var (
	taskSvrHost string
	lockClient  *redislock.Client
)

// InitSvr 保留原项目初始化入口。
func InitSvr(taskServerHost, lockAddr, lockPassword string) {
	taskSvrHost = taskServerHost
	lockClient = redislock.NewClient(lockAddr, lockPassword)
}

type TaskMgr struct {
	IntervalTime  time.Duration
	TaskType      string
	ScheduleLimit int
}

// 原项目将调度配置缓存在进程内。
// 本次增加读写锁，避免定期刷新与调度读取发生 map 竞争。
var (
	scheduleCfgMu  sync.RWMutex
	scheduleCfgDic = make(map[string]*model.TaskScheduleCfg)

	scheduleInitMu      sync.Mutex
	scheduleInitialized bool
)

// LoadCfg 从 flowsvr 获取调度配置。
func LoadCfg() error {
	client := rpc.TaskRpc{Host: taskSvrHost}

	resp, err := client.GetTaskScheduleCfgList()
	if err != nil {
		return err
	}

	scheduleCfgMu.Lock()
	defer scheduleCfgMu.Unlock()

	for _, cfg := range resp.ScheduleCfgList {
		if cfg == nil || cfg.TaskType == "" {
			continue
		}

		// 存储副本，防止其他代码修改缓存对象。
		copyCfg := *cfg
		scheduleCfgDic[cfg.TaskType] = &copyCfg
	}

	return nil
}

func getCfg(taskType string) (*model.TaskScheduleCfg, bool) {
	scheduleCfgMu.RLock()
	defer scheduleCfgMu.RUnlock()

	cfg, ok := scheduleCfgDic[taskType]
	if !ok {
		return nil, false
	}

	copyCfg := *cfg
	return &copyCfg, true
}

// CycleReloadCfg 按原项目默认周期动态刷新配置。
func CycleReloadCfg(ctx context.Context) {
	ticker := time.NewTicker(
		DEFAULT_TIME_INTERVAL * time.Second,
	)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := LoadCfg(); err != nil {
				log.Printf(
					"reload schedule config failed: %v",
					err,
				)
			}
		}
	}
}

func initSchedule(ctx context.Context) error {
	scheduleInitMu.Lock()
	defer scheduleInitMu.Unlock()

	if scheduleInitialized {
		return nil
	}

	if err := LoadCfg(); err != nil {
		return fmt.Errorf(
			"initial load schedule cfg: %w",
			err,
		)
	}

	scheduleInitialized = true

	go func() {
		CycleReloadCfg(ctx)

		scheduleInitMu.Lock()
		scheduleInitialized = false
		scheduleInitMu.Unlock()
	}()

	return nil
}

// Schedule 与原项目一样，持续执行任务调度。
func (p *TaskMgr) Schedule() {
	if err := p.ScheduleContext(
		context.Background(),
	); err != nil {
		log.Printf("schedule stopped: %v", err)
	}
}

func (p *TaskMgr) validate() error {
	if p == nil {
		return errors.New("task manager is nil")
	}
	if p.TaskType == "" {
		return errors.New("task manager task type is empty")
	}
	if taskSvrHost == "" {
		return errors.New("task server host is empty")
	}
	if lockClient == nil {
		return errors.New("redis lock client is nil")
	}
	return nil
}

// ScheduleContext 可通过 context 终止调度。
func (p *TaskMgr) ScheduleContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("schedule context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.validate(); err != nil {
		return err
	}
	if err := initSchedule(ctx); err != nil {
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		cfg, ok := getCfg(p.TaskType)
		if !ok {
			return fmt.Errorf(
				"schedule cfg %q not found",
				p.TaskType,
			)
		}

		interval := time.Duration(
			cfg.ScheduleInterval,
		) * time.Second

		if cfg.ScheduleInterval <= 0 {
			interval = DEFAULT_TIME_INTERVAL * time.Second
		}

		// 保留原项目的 ±500ms 调度抖动。
		interval += time.Duration(
			RandNum(501),
		) * time.Millisecond

		if interval <= 0 {
			interval = time.Millisecond
		}

		timer := time.NewTimer(interval)

		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		if err := p.schedule(ctx); err != nil {
			log.Printf(
				"schedule task type %s: %v",
				p.TaskType, err,
			)
		}
	}
}

// RunOnce 执行一次完整的领取 + 业务处理。
// 这是为了本地联调额外提供的入口。
func (p *TaskMgr) RunOnce(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.validate(); err != nil {
		return err
	}
	if err := LoadCfg(); err != nil {
		return err
	}
	return p.schedule(ctx)
}

// schedule 实现原项目的三步：
// 1. Redis 加锁
// 2. Hold 并解锁
// 3. 并发执行业务任务
func (p *TaskMgr) schedule(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	mutex := redislock.NewRedisLock(
		p.TaskType,
		lockClient,
		redislock.WithBlock(),
		redislock.WithWatchDogMode(),
		redislock.WithExpireSeconds(3),
	)

	if err := mutex.Lock(ctx); err != nil {
		return fmt.Errorf("redis lock: %w", err)
	}

	// 锁只覆盖任务领取过程。
	tasks, holdErr := p.hold()

	unlockCtx, cancel := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)
	unlockErr := mutex.Unlock(unlockCtx)
	cancel()

	if unlockErr != nil {
		return fmt.Errorf(
			"redis unlock: %w",
			unlockErr,
		)
	}
	if holdErr != nil {
		return holdErr
	}
	if len(tasks) == 0 {
		return nil
	}

	// 释放锁以后才开始业务执行。
	return p.runTasks(tasks)
}

// hold 批量领取 TaskData，然后转换为 TaskIntf。
func (p *TaskMgr) hold() ([]TaskIntf, error) {
	cfg, ok := getCfg(p.TaskType)
	if !ok {
		return nil, fmt.Errorf(
			"task type %q not found",
			p.TaskType,
		)
	}

	client := rpc.TaskRpc{Host: taskSvrHost}

	resp, err := client.HoldTasks(&model.HoldTasksReq{
		TaskType: p.TaskType,
		Limit:    cfg.ScheduleLimit,
	})
	if err != nil {
		return nil, err
	}

	tasks := make([]TaskIntf, 0, len(resp.TaskList))

	for _, storage := range resp.TaskList {
		task, err := GetTaskInfoFromStorage(storage)
		if err != nil {
			return nil, err
		}

		// 原始 HoldTasks 返回的快照可能仍为 pending。
		task.Base().Status = int(
			constant.TASK_STATUS_PROCESSING,
		)

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// runTasks 使用 Semaphore 控制 Goroutine 数量。
func (p *TaskMgr) runTasks(tasks []TaskIntf) error {
	cfg, ok := getCfg(p.TaskType)
	if !ok {
		return fmt.Errorf("task schedule config missing")
	}

	limit := scheduleConcurrency(cfg, len(tasks))
	sema := make(chan struct{}, limit)

	var wg sync.WaitGroup

	for _, task := range tasks {
		// 发送令牌：如果达到并发上限，这里会阻塞。
		sema <- struct{}{}
		wg.Add(1)

		go func(t TaskIntf) {
			defer wg.Done()
			defer func() { <-sema }()

			defer func() {
				if value := recover(); value != nil {
					log.Printf(
						"task panic: %v\n%s",
						value, debug.Stack(),
					)
				}
			}()

			runTask(t, cfg)
		}(task)
	}

	wg.Wait()
	return nil
}

// 作为可替换入口，方便测试并发调度。
var runTask = run

func scheduleConcurrency(
	cfg *model.TaskScheduleCfg,
	taskCount int,
) int {
	if taskCount <= 0 {
		return 1
	}

	limit := taskCount
	if cfg != nil && cfg.ScheduleLimit > 0 {
		limit = cfg.ScheduleLimit
	}
	if limit > taskCount {
		return taskCount
	}
	return limit
}

// RandNum 返回 [-num+1, num-1] 的随机波动。
func RandNum(num int) int {
	if num <= 0 {
		return 0
	}
	step := rand.Intn(num)
	if rand.Intn(2) == 0 {
		return -step
	}
	return step
}

// HoldOnce 保留第五章的一次性领取演示。
// 注意：它只 Hold，不执行业务任务。
func (p *TaskMgr) HoldOnce(
	ctx context.Context,
) (tasks []*model.TaskData, err error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := p.validate(); err != nil {
		return nil, err
	}

	if err := LoadCfg(); err != nil {
		return nil, err
	}

	cfg, ok := getCfg(p.TaskType)
	if !ok {
		return nil, fmt.Errorf(
			"task cfg %q not found", p.TaskType,
		)
	}

	mutex := redislock.NewRedisLock(
		p.TaskType,
		lockClient,
		redislock.WithBlock(),
		redislock.WithWatchDogMode(),
		redislock.WithExpireSeconds(3),
	)

	if err := mutex.Lock(ctx); err != nil {
		return nil, err
	}

	defer func() {
		unlockCtx, cancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cancel()

		if unlockErr := mutex.Unlock(unlockCtx); unlockErr != nil {
			err = errors.Join(err, unlockErr)
		}
	}()

	client := rpc.TaskRpc{Host: taskSvrHost}

	resp, err := client.HoldTasks(&model.HoldTasksReq{
		TaskType: p.TaskType,
		Limit:    cfg.ScheduleLimit,
	})
	if err != nil {
		return nil, err
	}

	tasks = resp.TaskList
	for _, task := range tasks {
		if task != nil {
			task.Status = int(
				constant.TASK_STATUS_PROCESSING,
			)
		}
	}

	return tasks, nil
}

// run 是原 Asyncflow 的单任务执行生命周期。
func run(task TaskIntf, cfg *model.TaskScheduleCfg) {
	base := task.Base()

	if base.ScheduleLog == nil {
		base.ScheduleLog = &ScheduleLog{}
	}

	// 原项目将收尾与持久化放在 defer 中。
	// 即使 HandleProcess 提前 return，也会尝试回写。
	defer func() {
		if base.Status == int(
			constant.TASK_STATUS_FAILED,
		) {
			// 必须成功的失败收尾。
			if err := task.HandleFailedMust(); err != nil {
				base.Status = int(
					constant.TASK_STATUS_PROCESSING,
				)
				log.Printf(
					"HandleFailedMust failed: %v", err,
				)
				// 保留原始行为：此时不调用 SetTask。
				return
			}

			// 允许失败的失败收尾。
			if err := task.HandleFinishError(); err != nil {
				log.Printf(
					"HandleFinishError failed: %v", err,
				)
				// 原项目在这里也会提前返回。
				return
			}
		}

		if base.Status == int(
			constant.TASK_STATUS_FAILED,
		) || base.Status == int(
			constant.TASK_STATUS_SUCC,
		) {
			task.HandleFinish()
		}

		// 持久化任务状态、阶段、上下文。
		if err := task.SetTask(); err != nil {
			log.Printf(
				"SetTask failed, retry once: %v", err,
			)
			if retryErr := task.SetTask(); retryErr != nil {
				log.Printf(
					"SetTask retry failed: %v", retryErr,
				)
			}
		}
	}()

	// 第一步：加载具体业务上下文。
	if err := task.ContextLoad(); err != nil {
		base.Status = int(
			constant.TASK_STATUS_PENDING,
		)
		log.Printf(
			"ContextLoad failed task=%s: %v",
			base.TaskId, err,
		)
		return
	}

	// 第二步：执行具体业务。
	begin := time.Now()
	err := task.HandleProcess()
	cost := time.Since(begin)

	// 第三步：追加调度历史，最多保留最近三条。
	base.ScheduleLog.HistoryDatas = append(
		base.ScheduleLog.HistoryDatas,
		base.ScheduleLog.LastData,
	)

	if len(base.ScheduleLog.HistoryDatas) > 3 {
		base.ScheduleLog.HistoryDatas =
			base.ScheduleLog.HistoryDatas[len(base.ScheduleLog.HistoryDatas)-3:]
	}

	// 没有设置终态时，任务重新进入待调度状态。
	if base.Status == int(
		constant.TASK_STATUS_PROCESSING,
	) {
		base.Status = int(
			constant.TASK_STATUS_PENDING,
		)
	}

	// 记录本轮调度信息。
	base.ScheduleLog.LastData = ScheduleData{
		TraceId: uuid.NewString(),
		Cost:    fmt.Sprintf("%dms", cost.Milliseconds()),
		ErrMsg:  "",
	}

	// 默认下一次按优先级调度。
	base.OrderTime = time.Now().Unix() - base.Priority

	if err == nil {
		return
	}

	// 第四步：处理业务执行失败与重试。
	if cfg == nil {
		cfg = &model.TaskScheduleCfg{}
	}

	// 原项目采用递增的 MaxRetryInterval。
	if base.MaxRetryInterval > 0 {
		if base.CrtRetryNum < 30 {
			base.MaxRetryInterval <<= 1
		} else {
			base.MaxRetryInterval = cfg.MaxRetryInterval
		}

		if base.MaxRetryInterval > cfg.MaxRetryInterval {
			base.MaxRetryInterval = cfg.MaxRetryInterval
		}
	}

	delay := base.MaxRetryInterval
	if delay < 0 {
		delay = -delay
	}

	// 如果有重试延迟，则使用未来的 OrderTime。
	// 此时优先级不再提前该次重试。
	if delay != 0 {
		base.OrderTime = time.Now().Unix() + int64(delay)
	}

	errMsg := err.Error()
	if len(errMsg) > MAX_ERR_MSG_LEN {
		errMsg = errMsg[:MAX_ERR_MSG_LEN]
	}

	base.ScheduleLog.LastData.ErrMsg = errMsg

	log.Printf(
		"HandleProcess task=%s failed: %s",
		base.TaskId, errMsg,
	)

	// MaxRetryNum == 0 表示不重试。
	// CrtRetryNum >= MaxRetryNum 表示已用完重试次数。
	if base.MaxRetryNum == 0 ||
		base.CrtRetryNum >= base.MaxRetryNum {
		base.Status = int(
			constant.TASK_STATUS_FAILED,
		)
		return
	}

	if base.Status != int(
		constant.TASK_STATUS_FAILED,
	) {
		base.CrtRetryNum++
	}
}
