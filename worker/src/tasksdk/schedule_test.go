package tasksdk

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/czh0727/asy/taskutils/constant"
	"github.com/czh0727/asy/taskutils/rpc/model"
)

// runnerTask 专门用于测试调度生命周期。
type runnerTask struct {
	TaskBase

	processErr  error
	onProcess   func(*runnerTask)
	setCount    int
	finishCount int
	failedCount int

	failedMustErr error
}

// TaskBase 已提供 Base、ContextLoad、CreateTask 等方法。
func (p *runnerTask) HandleProcess() error {
	if p.onProcess != nil {
		p.onProcess(p)
	}
	return p.processErr
}

func (p *runnerTask) HandleFinish() {
	p.finishCount++
}

func (p *runnerTask) HandleFinishError() error {
	p.failedCount++
	return nil
}

func (p *runnerTask) HandleFailedMust() error {
	return p.failedMustErr
}

// 测试中不发送真实 SetTask HTTP 请求。
func (p *runnerTask) SetTask() error {
	p.setCount++
	return nil
}

func newRunnerTask() *runnerTask {
	return &runnerTask{
		TaskBase: TaskBase{
			TaskId:           "test-task",
			TaskType:         "lark",
			Status:           int(constant.TASK_STATUS_PROCESSING),
			MaxRetryNum:      3,
			MaxRetryInterval: 5,
			ScheduleLog:      &ScheduleLog{},
		},
	}
}

func TestRunSuccess(t *testing.T) {
	job := newRunnerTask()

	job.onProcess = func(p *runnerTask) {
		p.TaskStage = "record"
		p.Status = int(constant.TASK_STATUS_SUCC)
	}

	run(job, &model.TaskScheduleCfg{
		MaxRetryInterval: 30,
	})

	if job.Status != int(constant.TASK_STATUS_SUCC) {
		t.Fatalf("status=%d, want success", job.Status)
	}

	if job.TaskStage != "record" {
		t.Fatalf("stage=%s", job.TaskStage)
	}

	if job.finishCount != 1 || job.setCount != 1 {
		t.Fatalf(
			"finish=%d set=%d",
			job.finishCount,
			job.setCount,
		)
	}

	if job.ScheduleLog.LastData.TraceId == "" {
		t.Fatal("missing schedule trace id")
	}
}

func TestRunContinuesToNextSchedule(t *testing.T) {
	job := newRunnerTask()

	// HandleProcess 返回 nil，
	// 但没有显式设置 success。
	run(job, &model.TaskScheduleCfg{
		MaxRetryInterval: 30,
	})

	if job.Status != int(constant.TASK_STATUS_PENDING) {
		t.Fatalf(
			"status=%d, want pending",
			job.Status,
		)
	}

	if job.finishCount != 0 || job.setCount != 1 {
		t.Fatalf(
			"finish=%d set=%d",
			job.finishCount,
			job.setCount,
		)
	}
}

func TestRunRetryThenFailed(t *testing.T) {
	job := newRunnerTask()
	job.processErr = errors.New("temporary error")

	cfg := &model.TaskScheduleCfg{
		MaxRetryInterval: 30,
	}

	expectedDelays := []int{10, 20, 30}

	for i, delay := range expectedDelays {
		job.Status = int(constant.TASK_STATUS_PROCESSING)

		before := time.Now().Unix()
		run(job, cfg)

		if job.Status != int(constant.TASK_STATUS_PENDING) {
			t.Fatalf(
				"round=%d status=%d",
				i, job.Status,
			)
		}

		if job.CrtRetryNum != i+1 {
			t.Fatalf(
				"round=%d retry=%d",
				i, job.CrtRetryNum,
			)
		}

		if job.MaxRetryInterval != delay {
			t.Fatalf(
				"round=%d delay=%d, want=%d",
				i, job.MaxRetryInterval, delay,
			)
		}

		if job.OrderTime < before+int64(delay) {
			t.Fatalf(
				"round=%d order_time=%d",
				i, job.OrderTime,
			)
		}
	}

	// 第四次失败：重试预算已经耗尽。
	job.Status = int(constant.TASK_STATUS_PROCESSING)
	run(job, cfg)

	if job.Status != int(constant.TASK_STATUS_FAILED) {
		t.Fatalf(
			"status=%d, want failed",
			job.Status,
		)
	}

	if job.CrtRetryNum != 3 {
		t.Fatalf(
			"retry count=%d, want 3",
			job.CrtRetryNum,
		)
	}

	if job.failedCount != 1 ||
		job.finishCount != 1 ||
		job.setCount != 4 {
		t.Fatalf(
			"failed=%d finish=%d set=%d",
			job.failedCount,
			job.finishCount,
			job.setCount,
		)
	}

	if len(job.ScheduleLog.HistoryDatas) != 3 {
		t.Fatalf(
			"history count=%d",
			len(job.ScheduleLog.HistoryDatas),
		)
	}
}

func TestRunFailedMustBlocksFinalWrite(t *testing.T) {
	job := newRunnerTask()
	job.MaxRetryNum = 0
	job.processErr = errors.New("permanent error")
	job.failedMustErr = errors.New("cleanup failed")

	run(job, &model.TaskScheduleCfg{
		MaxRetryInterval: 30,
	})

	// 原项目在必要失败收尾失败时，
	// 不进行最终失败状态的 SetTask。
	if job.Status != int(constant.TASK_STATUS_PROCESSING) {
		t.Fatalf("status=%d", job.Status)
	}
	if job.setCount != 0 {
		t.Fatalf("setCount=%d, want 0", job.setCount)
	}
}

func TestScheduleConcurrency(t *testing.T) {
	cases := []struct {
		cfg   *model.TaskScheduleCfg
		count int
		want  int
	}{
		{nil, 0, 1},
		{nil, 3, 3},
		{&model.TaskScheduleCfg{}, 3, 3},
		{&model.TaskScheduleCfg{ScheduleLimit: 2}, 5, 2},
		{&model.TaskScheduleCfg{ScheduleLimit: 10}, 5, 5},
	}

	for _, tc := range cases {
		got := scheduleConcurrency(tc.cfg, tc.count)
		if got != tc.want {
			t.Fatalf(
				"count=%d got=%d want=%d",
				tc.count, got, tc.want,
			)
		}
	}
}

func TestRunTasksRespectsConcurrency(t *testing.T) {
	// 设置一份临时调度配置。
	scheduleCfgMu.Lock()
	oldCfg := scheduleCfgDic
	scheduleCfgDic = map[string]*model.TaskScheduleCfg{
		"lark": {
			TaskType:      "lark",
			ScheduleLimit: 2,
		},
	}
	scheduleCfgMu.Unlock()

	oldRun := runTask

	t.Cleanup(func() {
		runTask = oldRun

		scheduleCfgMu.Lock()
		scheduleCfgDic = oldCfg
		scheduleCfgMu.Unlock()
	})

	var active int32
	var maximum int32
	var processed int32

	// 替换执行函数，用于测量实际并发数。
	runTask = func(
		job TaskIntf,
		cfg *model.TaskScheduleCfg,
	) {
		now := atomic.AddInt32(&active, 1)

		for {
			old := atomic.LoadInt32(&maximum)
			if now <= old ||
				atomic.CompareAndSwapInt32(
					&maximum, old, now,
				) {
				break
			}
		}

		time.Sleep(20 * time.Millisecond)

		atomic.AddInt32(&processed, 1)
		atomic.AddInt32(&active, -1)
	}

	jobs := make([]TaskIntf, 5)
	for i := range jobs {
		jobs[i] = newRunnerTask()
	}

	mgr := &TaskMgr{TaskType: "lark"}

	if err := mgr.runTasks(jobs); err != nil {
		t.Fatal(err)
	}

	if processed != 5 {
		t.Fatalf(
			"processed=%d, want 5", processed,
		)
	}

	if maximum > 2 {
		t.Fatalf(
			"maximum concurrency=%d, want <=2",
			maximum,
		)
	}
}

func TestScheduleContextNil(t *testing.T) {
	mgr := &TaskMgr{TaskType: "lark"}

	if err := mgr.ScheduleContext(nil); err == nil {
		t.Fatal("nil context should return error")
	}
}

func TestCycleReloadCfgStops(t *testing.T) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	done := make(chan struct{})

	go func() {
		CycleReloadCfg(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("CycleReloadCfg did not stop")
	}
}
