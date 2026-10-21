package task

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Status string

const (
	Pending    Status = "pending"
	Processing Status = "processing"
	Success    Status = "success"
	Failed     Status = "failed"
)

// Task 是任务的领域模型，不依赖 MySQL、Redis 或 HTTP。
type Task struct {
	ID         string
	Type       string
	Status     Status
	Payload    json.RawMessage
	Priority   int
	OrderTime  time.Time
	RetryCount int
	MaxRetries int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewTask 创建任务；不执行任务，也不负责持久化。
func NewTask(
	taskType string,
	payload json.RawMessage,
	priority int,
	runAt time.Time,
	maxRetries int,
) (*Task, error) {

	if strings.TrimSpace(taskType) == "" {
		return nil, fmt.Errorf("task type cannot be empty")
	}

	if len(payload) > 0 && !json.Valid(payload) {
		return nil, fmt.Errorf("task payload must be valid JSON")
	}

	if priority < 0 || priority > 100 {
		return nil, fmt.Errorf("priority must be between 0 and 100")
	}

	if maxRetries < 0 {
		return nil, fmt.Errorf("max retries cannot be negative")
	}

	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("generate task id: %w", err)
	}

	now := time.Now().UTC()

	if runAt.IsZero() {
		runAt = now
	}

	return &Task{
		ID:         hex.EncodeToString(idBytes),
		Type:       taskType,
		Status:     Pending,
		Payload:    append(json.RawMessage(nil), payload...),
		Priority:   priority,
		OrderTime:  runAt.UTC(),
		MaxRetries: maxRetries,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// CanTransition 判断状态转换是否合法。
// 这里只判断业务规则，不保证并发安全。
func CanTransition(from, to Status) bool {
	switch from {
	case Pending:
		return to == Processing

	case Processing:
		return to == Pending ||
			to == Success ||
			to == Failed

	default:
		return false
	}
}

// Transition 修改任务状态。
func (t *Task) Transition(to Status) error {
	if t == nil {
		return fmt.Errorf("cannot transition a nil task")
	}

	if !CanTransition(t.Status, to) {
		return fmt.Errorf(
			"invalid task transition: %s -> %s",
			t.Status,
			to,
		)
	}

	t.Status = to
	t.UpdatedAt = time.Now().UTC()

	return nil
}

// Runnable 判断任务现在是否可以执行。
func (t *Task) Runnable(now time.Time) bool {
	return t != nil &&
		t.Status == Pending &&
		!now.Before(t.OrderTime)
}
