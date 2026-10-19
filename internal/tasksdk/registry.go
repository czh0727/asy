
package tasksdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Handler 定义业务任务的执行接口。
type Handler interface {
	Handle(
		ctx context.Context,
		payload json.RawMessage,
	) error
}

// HandlerFunc 让普通函数也可以实现 Handler。
type HandlerFunc func(
	context.Context,
	json.RawMessage,
) error

func (f HandlerFunc) Handle(
	ctx context.Context,
	payload json.RawMessage,
) error {
	return f(ctx, payload)
}

// Registry 保存 task_type 与 Handler 的映射。
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

// Register 注册任务 Handler。
func (r *Registry) Register(
	taskType string,
	handler Handler,
) error {
	if r == nil ||
		strings.TrimSpace(taskType) == "" ||
		handler == nil {
		return errors.New("invalid handler registration")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[taskType]; exists {
		return fmt.Errorf(
			"handler already registered for %q",
			taskType,
		)
	}

	r.handlers[taskType] = handler
	return nil
}

// Get 根据任务类型查找 Handler。
func (r *Registry) Get(
	taskType string,
) (Handler, bool) {
	if r == nil {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	h, ok := r.handlers[taskType]
	return h, ok
}
