package tasksdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/czh0727/asy/internal/task"
)

// RetryableError 表示允许由调度框架重试的业务错误。
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// Retryable 将普通 error 标记为可重试错误。
func Retryable(err error) error {
	if err == nil {
		return nil
	}

	return &RetryableError{Err: err}
}

func isRetryable(err error) bool {
	var target *RetryableError
	return errors.As(err, &target)
}

// Retry 通知 flowsvr 持久化一次可重试失败。
// Worker 不决定重试次数，也不决定下次执行时间。
func (c *HTTPClient) Retry(
	ctx context.Context,
	id string,
) (*task.RetryResult, error) {
	if !validTaskID(id) {
		return nil, errors.New("invalid task id")
	}

	resp, err := c.post(
		ctx,
		"/v1/internal/tasks/"+id+"/retry",
		struct{}{},
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpStatusError("retry", resp)
	}

	var result task.RetryResult

	err = json.NewDecoder(
		io.LimitReader(resp.Body, 1<<20),
	).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf(
			"decode retry result: %w", err,
		)
	}

	if result.Status != task.Pending &&
		result.Status != task.Failed {
		return nil, fmt.Errorf(
			"unexpected retry result: %s",
			result.Status,
		)
	}

	return &result, nil
}
