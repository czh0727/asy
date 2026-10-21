package tasksdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/czh0727/asy/internal/task"
)

// WorkerOptions 控制每个 Worker 实例的运行参数。
type WorkerOptions struct {
	MaxConcurrent int
	ShutdownGrace time.Duration
}

type Worker struct {
	client       TaskClient
	registry     *Registry
	taskType     string
	pollInterval time.Duration
	options      WorkerOptions
}

// NewWorker 保留第四章的构造函数。
// 默认串行执行，保证之前的代码和测试兼容。
func NewWorker(
	client TaskClient,
	registry *Registry,
	taskType string,
	pollInterval time.Duration,
) (*Worker, error) {
	return NewWorkerWithOptions(
		client,
		registry,
		taskType,
		pollInterval,
		WorkerOptions{
			MaxConcurrent: 1,
			ShutdownGrace: 10 * time.Second,
		},
	)
}

// NewWorkerWithOptions 创建可配置并发数的 Worker。
func NewWorkerWithOptions(
	client TaskClient,
	registry *Registry,
	taskType string,
	pollInterval time.Duration,
	opts WorkerOptions,
) (*Worker, error) {
	if client == nil ||
		registry == nil ||
		strings.TrimSpace(taskType) == "" ||
		pollInterval <= 0 {
		return nil, errors.New(
			"invalid worker configuration",
		)
	}

	if opts.MaxConcurrent <= 0 ||
		opts.ShutdownGrace <= 0 {
		return nil, errors.New(
			"max concurrent and shutdown grace must be positive",
		)
	}

	if _, ok := registry.Get(taskType); !ok {
		return nil, fmt.Errorf(
			"handler for %q not registered",
			taskType,
		)
	}

	return &Worker{
		client:       client,
		registry:     registry,
		taskType:     taskType,
		pollInterval: pollInterval,
		options:      opts,
	}, nil
}

// RunOnce 保留第四章接口，仍然同步执行一条任务。
func (w *Worker) RunOnce(
	ctx context.Context,
) (bool, error) {
	if ctx == nil {
		return false, errors.New("nil context")
	}

	if err := ctx.Err(); err != nil {
		return false, err
	}

	claimed, err := w.client.Claim(
		ctx, w.taskType,
	)
	if err != nil {
		return false, fmt.Errorf(
			"claim task: %w", err,
		)
	}

	if claimed == nil {
		return false, nil
	}

	return true, w.execute(ctx, claimed)
}

// execute 负责 Handler 执行与任务结果回写。
// Run 会保证调用 execute 前已经取得并发槽位。

func (w *Worker) execute(
	ctx context.Context,
	claimed *ClaimedTask,
) error {
	handler, ok := w.registry.Get(
		claimed.TaskType,
	)
	if !ok {
		return fmt.Errorf(
			"handler for %q missing after claim",
			claimed.TaskType,
		)
	}

	handleErr := callHandler(
		ctx, handler, claimed.Payload,
	)

	// 如果任务执行过程中被取消，
	// 当前阶段不直接修改状态。
	// 后续由超时恢复机制处理。
	if err := ctx.Err(); err != nil {
		return fmt.Errorf(
			"task %s interrupted: %w",
			claimed.ID,
			err,
		)
	}

	// 所有结果回写都使用独立的短超时 Context。
	finishCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	// 一、临时性错误：交给 flowsvr 决定是否重试。
	if handleErr != nil && isRetryable(handleErr) {
		result, err := w.client.Retry(
			finishCtx,
			claimed.ID,
		)
		if err != nil {
			return fmt.Errorf(
				"retry task %s (result uncertain): %w",
				claimed.ID,
				err,
			)
		}

		if result.Status == task.Pending {
			slog.Warn(
				"task scheduled for retry",
				"id", claimed.ID,
				"retry_count", result.RetryCount,
				"max_retries", result.MaxRetries,
				"next_run_at", result.OrderTime,
				"reason", handleErr,
			)
			return nil
		}

		slog.Error(
			"task exhausted retries",
			"id", claimed.ID,
			"retry_count", result.RetryCount,
			"reason", handleErr,
		)

		return nil
	}

	// 二、正常成功，或者不可重试的业务错误。
	finalStatus := task.Success
	if handleErr != nil {
		finalStatus = task.Failed
	}

	if err := w.client.Finish(
		finishCtx,
		claimed.ID,
		finalStatus,
	); err != nil {
		return fmt.Errorf(
			"finish task %s (result uncertain): %w",
			claimed.ID,
			err,
		)
	}

	if handleErr != nil {
		return fmt.Errorf(
			"task %s permanently failed: %w",
			claimed.ID,
			handleErr,
		)
	}

	slog.Info(
		"task succeeded",
		"id", claimed.ID,
		"type", claimed.TaskType,
	)

	return nil
}

// callHandler 捕获当前 Goroutine 中的 Handler panic。
func callHandler(
	ctx context.Context,
	handler Handler,
	payload json.RawMessage,
) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf(
				"handler panic: %v", v,
			)
		}
	}()

	return handler.Handle(ctx, payload)
}

// Run：并发调度入口。
// 关键原则：先获取槽位，再领取任务。
func (w *Worker) Run(
	ctx context.Context,
) error {
	if ctx == nil {
		return errors.New("nil context")
	}

	// 收到停止领取信号时，不立即取消正在执行的任务。
	// 继承 Context 中的 Value，但不继承取消和 Deadline。
	jobCtx, cancelJobs := context.WithCancel(
		context.WithoutCancel(ctx),
	)
	defer cancelJobs()

	// 有界 Channel 实现 Semaphore。
	slots := make(
		chan struct{},
		w.options.MaxConcurrent,
	)

	var wg sync.WaitGroup

	for {
		// 第一步：先申请一个执行槽位。
		select {
		case <-ctx.Done():
			return w.drain(cancelJobs, &wg)

		case slots <- struct{}{}:
			// 成功获取槽位
		}

		// 避免停止信号和槽位同时就绪时继续 Claim。
		if ctx.Err() != nil {
			<-slots
			return w.drain(cancelJobs, &wg)
		}

		// 第二步：有槽位才请求 flowsvr。
		claimed, err := w.client.Claim(
			ctx, w.taskType,
		)

		if err != nil || claimed == nil {
			// 没领到任务，立即释放槽位。
			<-slots

			if err != nil && ctx.Err() == nil {
				slog.Error(
					"claim failed",
					"task_type", w.taskType,
					"err", err,
				)
			}

			// 无任务或网络异常：等待下一轮。
			if !waitPoll(ctx, w.pollInterval) {
				return w.drain(cancelJobs, &wg)
			}
			continue
		}

		// 第三步：成功领取，启动 Goroutine。
		wg.Add(1)

		go func(t *ClaimedTask) {
			defer wg.Done()

			// 执行结束后归还槽位。
			defer func() {
				<-slots
			}()

			if err := w.execute(
				jobCtx, t,
			); err != nil {
				slog.Error(
					"task execution failed",
					"task_id", t.ID,
					"err", err,
				)
			}
		}(claimed)

		// 当前还有任务时立即尝试下一次领取。
		// 如果槽位满了，会在循环顶部等待。
	}
}

// waitPoll 支持 Context 取消的等待。
func waitPoll(
	ctx context.Context,
	interval time.Duration,
) bool {
	t := time.NewTimer(interval)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// drain：收到停止信号后，等待在途任务完成。
func (w *Worker) drain(
	cancelJobs context.CancelFunc,
	wg *sync.WaitGroup,
) error {
	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	timer := time.NewTimer(
		w.options.ShutdownGrace,
	)
	defer timer.Stop()

	select {
	case <-done:
		// 所有任务在宽限期内完成。
		return nil

	case <-timer.C:
		// 宽限期耗尽，通知未完成任务取消。
		cancelJobs()

		return fmt.Errorf(
			"shutdown grace %s exceeded; "+
				"in-flight tasks may remain processing",
			w.options.ShutdownGrace,
		)
	}
}
