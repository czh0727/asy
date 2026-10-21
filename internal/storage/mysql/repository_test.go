package mysql

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/czh0727/asy/internal/task"
)

// 不设置 TEST_MYSQL_DSN 时跳过集成测试。
func testRepo(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()

	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to enable MySQL integration tests")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	db.SetMaxOpenConns(20)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	return New(db), db
}

func newTestTask(
	t *testing.T,
	runAt time.Time,
) *task.Task {
	t.Helper()

	tmp, err := task.NewTask(
		"test", nil, 5, runAt, 3,
	)
	if err != nil {
		t.Fatal(err)
	}

	// 每个测试使用独立的任务类型。
	tmp.Type = "test." + tmp.ID
	return tmp
}

func cleanupTask(
	t *testing.T,
	db *sql.DB,
	id string,
) {
	t.Helper()

	t.Cleanup(func() {
		_, err := db.ExecContext(
			context.Background(),
			`DELETE FROM tasks WHERE id = ?`,
			id,
		)
		if err != nil {
			t.Errorf("cleanup task %s: %v", id, err)
		}
	})
}

// 验证持久化、领取、完成和重复更新。
func TestCreateGetAndFinish(t *testing.T) {
	repo, db := testRepo(t)
	ctx := context.Background()

	created := newTestTask(
		t,
		time.Now().Add(-time.Minute),
	)
	cleanupTask(t, db, created.ID)

	if err := repo.Create(ctx, created); err != nil {
		t.Fatal(err)
	}

	stored, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}

	if stored.Status != task.Pending ||
		stored.ID != created.ID ||
		stored.Type != created.Type {
		t.Fatalf("wrong persisted task: %+v", stored)
	}

	// 主键应当防止重复插入相同的 ID。
	if err := repo.Create(ctx, created); err == nil {
		t.Fatal("duplicate ID should fail")
	}

	claimed, err := repo.ClaimNext(
		ctx, created.Type,
	)
	if err != nil {
		t.Fatal(err)
	}

	if claimed.ID != created.ID ||
		claimed.Status != task.Processing {
		t.Fatalf("wrong claim result: %+v", claimed)
	}

	if err := repo.Finish(
		ctx, claimed.ID, task.Success,
	); err != nil {
		t.Fatal(err)
	}

	// 任务已经 success，不能再次结束。
	err = repo.Finish(
		ctx, claimed.ID, task.Success,
	)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf(
			"second finish = %v, want ErrConflict",
			err,
		)
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != task.Success {
		t.Fatalf(
			"status = %s, want success", got.Status,
		)
	}
}

// 未到执行时间的任务不能被领取。
func TestDelayedTaskCannotBeClaimed(t *testing.T) {
	repo, db := testRepo(t)
	ctx := context.Background()

	created := newTestTask(
		t,
		time.Now().Add(time.Hour),
	)
	cleanupTask(t, db, created.ID)

	if err := repo.Create(ctx, created); err != nil {
		t.Fatal(err)
	}

	_, err := repo.ClaimNext(ctx, created.Type)

	if !errors.Is(err, ErrNoReadyTask) {
		t.Fatalf(
			"claim future task = %v, want ErrNoReadyTask",
			err,
		)
	}
}

// 12 个并发请求抢同一个任务，只允许一个成功。
func TestConcurrentClaimOnlyOneWinner(t *testing.T) {
	repo, db := testRepo(t)
	ctx := context.Background()

	created := newTestTask(
		t,
		time.Now().Add(-time.Minute),
	)
	cleanupTask(t, db, created.ID)

	if err := repo.Create(ctx, created); err != nil {
		t.Fatal(err)
	}

	const contenders = 12

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, contenders)

	for i := 0; i < contenders; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			// 等待所有协程一起开始竞争。
			<-start

			claimed, err := repo.ClaimNext(
				ctx, created.Type,
			)

			if err == nil &&
				claimed.ID != created.ID {
				results <- errors.New(
					"claimed unexpected task",
				)
				return
			}

			results <- err
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	successes := 0

	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrNoReadyTask) {
			t.Fatalf(
				"unexpected claim error: %v", err,
			)
		}
	}

	if successes != 1 {
		t.Fatalf(
			"claim successes = %d, want exactly 1",
			successes,
		)
	}
}
