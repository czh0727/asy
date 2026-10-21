package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/czh0727/asy/internal/task"
)

// Retry 处理 processing 任务的一次可重试失败。
//
// 有重试额度：processing -> pending，增加 retry_count，
//
//	更新 order_time 到未来。
//
// 无重试额度：processing -> failed。
//
// 数据库负责决定次数和时间，Worker 不直接操作这些字段。
func (r *Repository) Retry(
	ctx context.Context,
	id string,
) (*task.RetryResult, error) {
	tx, err := r.db.BeginTx(
		ctx,
		&sql.TxOptions{
			Isolation: sql.LevelReadCommitted,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"begin retry transaction: %w", err,
		)
	}
	defer tx.Rollback()

	// 1. 加行锁，读取当前执行状态及重试次数。
	var retryCount, maxRetries int

	err = tx.QueryRowContext(ctx, `
        SELECT retry_count, max_retries
          FROM tasks
         WHERE id = ?
           AND status = ?
         FOR UPDATE`,
		id, task.Processing,
	).Scan(&retryCount, &maxRetries)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, fmt.Errorf(
			"lock task for retry: %w", err,
		)
	}

	if retryCount < 0 || maxRetries < 0 {
		return nil, errors.New(
			"invalid retry counters in database",
		)
	}

	var result sql.Result

	if retryCount < maxRetries {
		// 2A. 还有额度：计算退避时间。
		delay := task.RetryDelay(retryCount)

		// 使用数据库时钟更新 order_time，
		// 避免 Worker 与数据库之间的时钟偏差。
		result, err = tx.ExecContext(ctx, `
            UPDATE tasks
               SET status = ?,
                   retry_count = retry_count + 1,
                   order_time = DATE_ADD(
                       UTC_TIMESTAMP(6),
                       INTERVAL ? MICROSECOND
                   ),
                   updated_at = UTC_TIMESTAMP(6)
             WHERE id = ?
               AND status = ?
               AND retry_count = ?`,
			task.Pending,
			delay.Microseconds(),
			id,
			task.Processing,
			retryCount,
		)
	} else {
		// 2B. 额度耗尽：进入最终失败状态。
		result, err = tx.ExecContext(ctx, `
            UPDATE tasks
               SET status = ?,
                   updated_at = UTC_TIMESTAMP(6)
             WHERE id = ?
               AND status = ?
               AND retry_count = ?`,
			task.Failed,
			id,
			task.Processing,
			retryCount,
		)
	}

	if err != nil {
		return nil, fmt.Errorf(
			"update retry state: %w", err,
		)
	}

	// 3. 确认恰好修改一条任务。
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, ErrConflict
	}

	// 4. 读取本次事务修改后的真实数据库状态。
	out := &task.RetryResult{}

	err = tx.QueryRowContext(ctx, `
        SELECT status, retry_count, max_retries, order_time
          FROM tasks
         WHERE id = ?`,
		id,
	).Scan(
		&out.Status,
		&out.RetryCount,
		&out.MaxRetries,
		&out.OrderTime,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"read retry result: %w", err,
		)
	}

	// 5. 事务提交成功后才能返回结果。
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf(
			"commit retry transaction: %w", err,
		)
	}

	return out, nil
}
