package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/czh0727/asy/internal/task"
	_ "github.com/go-sql-driver/mysql"
)

var (
	ErrNotFound    = errors.New("task not found")
	ErrNoReadyTask = errors.New("no ready task")
	ErrConflict    = errors.New("task state conflict")
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create 将新任务保存到 MySQL。
func (r *Repository) Create(
	ctx context.Context,
	t *task.Task,
) error {
	if t == nil ||
		t.ID == "" ||
		strings.TrimSpace(t.Type) == "" ||
		t.Status != task.Pending ||
		t.OrderTime.IsZero() {
		return errors.New("invalid new task")
	}

	payload := t.Payload
	if len(payload) == 0 {
		payload = json.RawMessage("null")
	}

	_, err := r.db.ExecContext(ctx, `
        INSERT INTO tasks
          (id, task_type, status, payload, priority, order_time,
           retry_count, max_retries, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID,
		t.Type,
		t.Status,
		string(payload),
		t.Priority,
		t.OrderTime,
		t.RetryCount,
		t.MaxRetries,
		t.CreatedAt,
		t.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("insert task %s: %w", t.ID, err)
	}
	return nil
}

// Get 根据任务 ID 查询完整任务。
func (r *Repository) Get(
	ctx context.Context,
	id string,
) (*task.Task, error) {
	t, err := scanTask(r.db.QueryRowContext(ctx, `
        SELECT id, task_type, status, payload, priority, order_time,
               retry_count, max_retries, created_at, updated_at
          FROM tasks
         WHERE id = ?`, id))

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task %s: %w", id, err)
	}
	return t, nil
}

// ClaimNext 原子领取一条已经到期的 pending 任务。
// 要求 MySQL 8.0+，使用 FOR UPDATE SKIP LOCKED。
func (r *Repository) ClaimNext(
	ctx context.Context,
	taskType string,
) (*task.Task, error) {
	if strings.TrimSpace(taskType) == "" {
		return nil, errors.New("task type cannot be empty")
	}

	// 1. 开启事务
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return nil, fmt.Errorf(
			"begin claim transaction: %w", err,
		)
	}
	defer tx.Rollback()

	// 2. 查询候选任务，并锁住选中的任务行。
	t, err := scanTask(tx.QueryRowContext(ctx, `
        SELECT id, task_type, status, payload, priority, order_time,
               retry_count, max_retries, created_at, updated_at
          FROM tasks
         WHERE task_type = ?
           AND status = ?
           AND order_time <= UTC_TIMESTAMP(6)
         ORDER BY order_time ASC, priority DESC, id ASC
         LIMIT 1
         FOR UPDATE SKIP LOCKED`,
		taskType, task.Pending,
	))

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoReadyTask
	}
	if err != nil {
		return nil, fmt.Errorf("select ready task: %w", err)
	}

	// 3. 条件 UPDATE，要求任务仍然是 pending。
	result, err := tx.ExecContext(ctx, `
        UPDATE tasks
           SET status = ?,
               updated_at = UTC_TIMESTAMP(6)
         WHERE id = ?
           AND status = ?`,
		task.Processing, t.ID, task.Pending,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"claim task %s: %w", t.ID, err,
		)
	}

	// 4. 检查受影响的行数。
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf(
			"claim affected rows: %w", err,
		)
	}

	if affected != 1 {
		return nil, ErrConflict
	}

	// 5. 获取数据库实际写入的更新时间。
	err = tx.QueryRowContext(ctx, `
        SELECT updated_at
          FROM tasks
         WHERE id = ?`, t.ID,
	).Scan(&t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf(
			"read claim timestamp: %w", err,
		)
	}

	// 6. 提交事务，提交成功才认为领取成功。
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf(
			"commit claim transaction: %w", err,
		)
	}

	t.Status = task.Processing
	return t, nil
}

// Finish 将正在执行的任务标记为成功或最终失败。
func (r *Repository) Finish(
	ctx context.Context,
	id string,
	to task.Status,
) error {
	if to != task.Success && to != task.Failed {
		return fmt.Errorf(
			"finish target %q is not terminal", to,
		)
	}

	if !task.CanTransition(task.Processing, to) {
		return fmt.Errorf(
			"invalid state transition: processing -> %s", to,
		)
	}

	result, err := r.db.ExecContext(ctx, `
        UPDATE tasks
           SET status = ?,
               updated_at = UTC_TIMESTAMP(6)
         WHERE id = ?
           AND status = ?`,
		to, id, task.Processing,
	)
	if err != nil {
		return fmt.Errorf("finish task %s: %w", id, err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("finish affected rows: %w", err)
	}
	if n != 1 {
		return ErrConflict
	}

	return nil
}

// 让同一套解析逻辑支持 sql.Row 等实现 Scan 的对象。
type scanner interface {
	Scan(dest ...any) error
}

func scanTask(row scanner) (*task.Task, error) {
	var t task.Task
	var payload []byte

	err := row.Scan(
		&t.ID,
		&t.Type,
		&t.Status,
		&payload,
		&t.Priority,
		&t.OrderTime,
		&t.RetryCount,
		&t.MaxRetries,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	t.Payload = append(
		json.RawMessage(nil),
		payload...,
	)

	return &t, nil
}
