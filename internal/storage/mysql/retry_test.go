package mysql

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/czh0727/asy/internal/task"
)

// 验证：失败 -> 延迟待执行 -> 再领取 -> 次数耗尽。
func TestRetryLifecycle(t *testing.T) {
	repo, db := testRepo(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	created := newTestTask(
		t,
		time.Now().Add(-time.Minute),
	)
	created.MaxRetries = 3

	cleanupTask(t, db, created.ID)

	if err := repo.Create(ctx, created); err != nil {
		t.Fatal(err)
	}

	for attempt := 0; attempt <= 3; attempt++ {
		claimed, err := repo.ClaimNext(
			ctx, created.Type,
		)
		if err != nil {
			t.Fatalf(
				"attempt %d claim: %v",
				attempt, err,
			)
		}

		if claimed.ID != created.ID {
			t.Fatal("unexpected claimed task")
		}

		now := time.Now()

		result, err := repo.Retry(ctx, created.ID)
		if err != nil {
			t.Fatalf(
				"attempt %d retry: %v",
				attempt, err,
			)
		}

		if attempt < 3 {
			if result.Status != task.Pending {
				t.Fatalf(
					"attempt %d status=%s want pending",
					attempt, result.Status,
				)
			}

			if result.RetryCount != attempt+1 {
				t.Fatalf(
					"retry count=%d want=%d",
					result.RetryCount,
					attempt+1,
				)
			}

			// 确认下次执行时间已推进到未来。
			if !result.OrderTime.After(now) {
				t.Fatal("retry order_time must be in future")
			}

			// 立刻领取应该没有结果。
			_, err = repo.ClaimNext(ctx, created.Type)
			if !errors.Is(err, ErrNoReadyTask) {
				t.Fatalf(
					"premature claim error=%v", err,
				)
			}

			// 仅测试使用：把调度时间快速推进到过去，
			// 无需真的等待 2s、4s、8s。
			_, err = db.ExecContext(ctx, `
                UPDATE tasks
                   SET order_time = DATE_SUB(
                       UTC_TIMESTAMP(6),
                       INTERVAL 1 SECOND
                   )
                 WHERE id = ?
                   AND status = ?`,
				created.ID,
				task.Pending,
			)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			// 第四次执行依然失败，额度耗尽。
			if result.Status != task.Failed ||
				result.RetryCount != 3 {
				t.Fatalf(
					"final result: %+v", result,
				)
			}
		}
	}

	stored, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}

	if stored.Status != task.Failed ||
		stored.RetryCount != 3 {
		t.Fatalf("unexpected final task: %+v", stored)
	}

	// 终态不能重复 Retry。
	_, err = repo.Retry(ctx, created.ID)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf(
			"retry terminal task=%v, want conflict", err,
		)
	}
}

// 验证两个并发 Retry 请求，只有一个能消费重试额度。
func TestConcurrentRetryOnlyOneWinner(t *testing.T) {
	repo, db := testRepo(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	created := newTestTask(
		t,
		time.Now().Add(-time.Minute),
	)
	created.MaxRetries = 2

	cleanupTask(t, db, created.ID)

	if err := repo.Create(ctx, created); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.ClaimNext(
		ctx, created.Type,
	); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)

	var wg sync.WaitGroup

	for i := 0; i < 2; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			<-start

			_, err := repo.Retry(ctx, created.ID)
			results <- err
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	success := 0
	conflicts := 0

	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrConflict):
			conflicts++
		default:
			t.Fatalf("unexpected retry error: %v", err)
		}
	}

	if success != 1 || conflicts != 1 {
		t.Fatalf(
			"success=%d conflicts=%d",
			success, conflicts,
		)
	}

	stored, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}

	if stored.RetryCount != 1 ||
		stored.Status != task.Pending {
		t.Fatalf(
			"retry must increment once: %+v", stored,
		)
	}
}
