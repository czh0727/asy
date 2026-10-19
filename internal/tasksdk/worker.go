
package tasksdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/czh0727/asy/internal/task"
)

type Worker struct {
	client       TaskClient
	registry     *Registry
	taskType     string
	pollInterval time.Duration
}

// 一个 Worker 实例当前串行处理一种任务类型。
func NewWorker(
	client TaskClient,
	registry *Registry,
	taskType string,
	pollInterval time.Duration,
) (*Worker, error) {
	if client == nil ||
		registry == nil ||
		strings.TrimSpace(taskType) == "" ||
		pollInterval <= 0 {
		return nil, errors.New(
			"invalid worker configuration",
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
	}, nil
}

// RunOnce 最多领取并执行一条任务。
// bool 表示本次是否领取到了任务。
func (w *Worker) RunOnce(
	ctx context.Context,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	// 第一步：从 flowsvr 领取任务
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

	// 第二步：查找对应业务 Handler
	handler, ok := w.registry.Get(
		claimed.TaskType,
	)
	if !ok {
		return true, fmt.Errorf(
			"handler for %q missing",
			claimed.TaskType,
		)
	}

	// 第三步：执行业务 Handler，并捕获 panic
	handleErr := callHandler(
		ctx, handler, claimed.Payload,
	)

	// 如果执行中收到了退出信号，
	// 不再将该任务确认为 success 或 failed。
	if err := ctx.Err(); err != nil {
		return true, fmt.Errorf(
			"task %s interrupted before finish: %w",
			claimed.ID,
			err,
		)
	}

	// 第四步：确定任务最终状态
	finalStatus := task.Success
	if handleErr != nil {
		finalStatus = task.Failed
	}

	// 第五步：回写任务执行结果。
	// 使用独立的短超时 Context 保护收尾操作。
	finishCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := w.client.Finish(
		finishCtx,
		claimed.ID,
		finalStatus,
	); err != nil {
		return true, fmt.Errorf(
			"finish task %s (result uncertain): %w",
			claimed.ID,
			err,
		)
	}

	if handleErr != nil {
		return true, fmt.Errorf(
			"task %s failed and marked failed: %w",
			claimed.ID,
			handleErr,
		)
	}

	slog.Info(
		"task succeeded",
		"id", claimed.ID,
		"type", claimed.TaskType,
	)

	return true, nil
}

// callHandler 统一执行 Handler，并恢复 panic。
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

// Run 是 Worker 的主调度循环。
func (w *Worker) Run(
	ctx context.Context,
) error {
	if ctx == nil {
		return errors.New("nil context")
	}

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		if ctx.Err() != nil {
			return nil
		}

		// 串行领取并执行一条任务。
		_, err := w.RunOnce(ctx)

		if err != nil && ctx.Err() == nil {
			slog.Error(
				"worker iteration failed",
				"task_type", w.taskType,
				"err", err,
			)
		}

		// 等待下一次轮询，或者收到退出信号。
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
