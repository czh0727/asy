package tasksdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/czh0727/asy/internal/task"
)

func concurrentTestWorker(
	t *testing.T,
	client *fakeClient,
	h HandlerFunc,
	n int,
	grace time.Duration,
) *Worker {
	t.Helper()

	r := NewRegistry()
	if err := r.Register("video.generate", h); err != nil {
		t.Fatal(err)
	}

	w, err := NewWorkerWithOptions(
		client,
		r,
		"video.generate",
		5*time.Millisecond,
		WorkerOptions{
			MaxConcurrent: n,
			ShutdownGrace: grace,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	return w
}

func makeQueue(n int) []*ClaimedTask {
	queue := make([]*ClaimedTask, n)

	for i := range queue {
		queue[i] = &ClaimedTask{
			ID:       fmt.Sprintf("%032x", i+1),
			TaskType: "video.generate",
			Status:   task.Processing,
		}
	}
	return queue
}

func waitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for worker")
	}
}

func TestBoundedConcurrency(t *testing.T) {
	const total = 8

	f := &fakeClient{
		queue:    makeQueue(total),
		finished: make(map[string]task.Status),
	}

	var active, maxActive atomic.Int32
	started := make(chan struct{}, total)
	release := make(chan struct{})

	h := HandlerFunc(func(
		ctx context.Context,
		_ json.RawMessage,
	) error {
		current := active.Add(1)

		// CAS 维护历史最大并发数。
		for {
			previous := maxActive.Load()

			if current <= previous ||
				maxActive.CompareAndSwap(previous, current) {
				break
			}
		}

		defer active.Add(-1)

		started <- struct{}{}

		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	w := concurrentTestWorker(
		t, f, h, 3, time.Second,
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- w.Run(ctx)
	}()

	// 等待前 3 个 Handler 启动。
	for i := 0; i < 3; i++ {
		waitSignal(t, started)
	}

	// 前三个未释放，第四个不应该运行。
	select {
	case <-started:
		t.Fatal(
			"fourth task started with only 3 slots",
		)
	case <-time.After(60 * time.Millisecond):
	}

	// 释放前三个任务，后续任务才能开始。
	close(release)

	deadline := time.After(3 * time.Second)

	for {
		f.mu.Lock()
		count := len(f.finished)
		f.mu.Unlock()

		if count == total {
			break
		}

		select {
		case <-deadline:
			t.Fatalf(
				"only %d/%d tasks completed",
				count, total,
			)
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not stop")
	}

	if m := maxActive.Load(); m > 3 || m < 2 {
		t.Fatalf(
			"max active=%d, expected between 2 and 3", m,
		)
	}
}

func TestGracefulShutdownDrainsAndStopsClaim(t *testing.T) {
	f := &fakeClient{
		queue:    makeQueue(2),
		finished: make(map[string]task.Status),
	}

	started := make(chan struct{})
	allow := make(chan struct{})

	h := HandlerFunc(func(
		context.Context,
		json.RawMessage,
	) error {
		close(started)
		<-allow
		return nil
	})

	w := concurrentTestWorker(
		t, f, h, 1, time.Second,
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	done := make(chan error, 1)

	go func() {
		done <- w.Run(ctx)
	}()

	waitSignal(t, started)

	// 当前任务仍在执行时停止调度。
	cancel()

	select {
	case err := <-done:
		t.Fatalf(
			"worker exited without draining: %v", err,
		)
	case <-time.After(30 * time.Millisecond):
	}

	// 允许当前任务完成。
	close(allow)

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("drain timed out")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// 第一条任务完成，第二条任务没有被领取。
	if len(f.finished) != 1 || len(f.queue) != 1 {
		t.Fatalf(
			"finished=%d queue=%d, expected one each",
			len(f.finished), len(f.queue),
		)
	}
}

func TestShutdownTimeoutCancelsInflight(t *testing.T) {
	f := &fakeClient{
		queue:    makeQueue(1),
		finished: make(map[string]task.Status),
	}

	started := make(chan struct{})
	stopped := make(chan struct{})

	h := HandlerFunc(func(
		ctx context.Context,
		_ json.RawMessage,
	) error {
		close(started)
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	})

	w := concurrentTestWorker(
		t, f, h, 1, 40*time.Millisecond,
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	done := make(chan error, 1)

	go func() {
		done <- w.Run(ctx)
	}()

	waitSignal(t, started)
	cancel()

	select {
	case err := <-done:
		if err == nil ||
			!strings.Contains(err.Error(), "grace") {
			t.Fatalf(
				"expected shutdown grace error, got %v",
				err,
			)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not return after grace")
	}

	waitSignal(t, stopped)

	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.finished) != 0 {
		t.Fatal(
			"canceled task must not be marked successful",
		)
	}
}

func TestNewWorkerWithOptionsRejectsInvalid(t *testing.T) {
	f := &fakeClient{}
	r := NewRegistry()

	_ = r.Register(
		"video.generate",
		HandlerFunc(func(
			context.Context,
			json.RawMessage,
		) error {
			return nil
		}),
	)

	_, err := NewWorkerWithOptions(
		f, r, "video.generate", time.Second,
		WorkerOptions{
			MaxConcurrent: 0,
			ShutdownGrace: time.Second,
		},
	)
	if err == nil {
		t.Fatal("expected invalid concurrency error")
	}

	_, err = NewWorkerWithOptions(
		f, r, "video.generate", time.Second,
		WorkerOptions{
			MaxConcurrent: 1,
			ShutdownGrace: 0,
		},
	)
	if err == nil {
		t.Fatal("expected invalid grace error")
	}
}
