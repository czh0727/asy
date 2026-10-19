
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/czh0727/asy/internal/storage/mysql"
	"github.com/czh0727/asy/internal/task"
	"github.com/gin-gonic/gin"
)

// Store 描述 HTTP 层依赖的数据库能力。
type Store interface {
	Create(context.Context, *task.Task) error
	Get(context.Context, string) (*task.Task, error)
	ClaimNext(context.Context, string) (*task.Task, error)
	Finish(context.Context, string, task.Status) error
}

type Handler struct {
	store       Store
	workerToken string
	ready       func(context.Context) error
}

func NewHandler(
	store Store,
	workerToken string,
	ready func(context.Context) error,
) *Handler {
	return &Handler{
		store:       store,
		workerToken: workerToken,
		ready:       ready,
	}
}

// Router 注册所有 HTTP 路由。
func (h *Handler) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "alive",
		})
	})
	r.GET("/readyz", h.readyz)

	v1 := r.Group("/v1")

	// 业务侧接口
	v1.POST("/tasks", h.createTask)
	v1.GET("/tasks/:id", h.getTask)

	// Worker 内部接口，要求 Token 鉴权
	internal := v1.Group(
		"/internal",
		h.checkWorkerToken,
	)
	internal.POST("/tasks/claim", h.claimTask)
	internal.POST("/tasks/:id/finish", h.finishTask)

	return r
}

// 请求数据结构

type createRequest struct {
	TaskType   string          `json:"task_type"`
	Payload    json.RawMessage `json:"payload"`
	Priority   int             `json:"priority"`
	RunAt      *time.Time      `json:"run_at"`
	MaxRetries int             `json:"max_retries"`
}

type claimRequest struct {
	TaskType string `json:"task_type"`
}

type finishRequest struct {
	Status task.Status `json:"status"`
}

// HTTP 返回的数据结构
type taskResponse struct {
	ID         string          `json:"id"`
	TaskType   string          `json:"task_type"`
	Status     task.Status     `json:"status"`
	Payload    json.RawMessage `json:"payload"`
	Priority   int             `json:"priority"`
	OrderTime  time.Time       `json:"order_time"`
	RetryCount int             `json:"retry_count"`
	MaxRetries int             `json:"max_retries"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

func responseOf(t *task.Task) taskResponse {
	return taskResponse{
		ID:         t.ID,
		TaskType:   t.Type,
		Status:     t.Status,
		Payload:    t.Payload,
		Priority:   t.Priority,
		OrderTime:  t.OrderTime,
		RetryCount: t.RetryCount,
		MaxRetries: t.MaxRetries,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
}

// 统一错误响应
func fail(
	c *gin.Context,
	status int,
	code string,
	message string,
) {
	c.JSON(status, gin.H{
		"code":    code,
		"message": message,
	})
}

// 限制请求大小、拒绝未知字段及多个 JSON 值。
func decodeJSON(c *gin.Context, out any) bool {
	if c.ContentType() != "application/json" {
		fail(c, http.StatusUnsupportedMediaType,
			"UNSUPPORTED_MEDIA_TYPE",
			"Content-Type must be application/json")
		return false
	}

	c.Request.Body = http.MaxBytesReader(
		c.Writer, c.Request.Body, 1<<20,
	)

	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()

	if err := d.Decode(out); err != nil {
		var maxErr *http.MaxBytesError

		if errors.As(err, &maxErr) {
			fail(c, http.StatusRequestEntityTooLarge,
				"TOO_LARGE",
				"request body exceeds 1 MiB")
		} else {
			fail(c, http.StatusBadRequest,
				"INVALID_JSON",
				"invalid JSON request body")
		}
		return false
	}

	// 拒绝连续发送两个 JSON 对象。
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		fail(c, http.StatusBadRequest,
			"INVALID_JSON",
			"body must contain exactly one JSON object")
		return false
	}

	return true
}

func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

// 创建任务：POST /v1/tasks
func (h *Handler) createTask(c *gin.Context) {
	var req createRequest
	if !decodeJSON(c, &req) {
		return
	}

	var at time.Time
	if req.RunAt != nil {
		at = *req.RunAt
	}

	t, err := task.NewTask(
		req.TaskType,
		req.Payload,
		req.Priority,
		at,
		req.MaxRetries,
	)
	if err != nil {
		fail(c, http.StatusBadRequest,
			"INVALID_ARGUMENT", err.Error())
		return
	}

	if len(t.Type) > 128 || t.MaxRetries > 1000 {
		fail(c, http.StatusBadRequest,
			"INVALID_ARGUMENT",
			"task_type too long or max_retries too large")
		return
	}

	if err := h.store.Create(c.Request.Context(), t); err != nil {
		slog.Error("create task", "err", err)
		fail(c, http.StatusInternalServerError,
			"INTERNAL", "failed to create task")
		return
	}

	c.Header("Location", "/v1/tasks/"+t.ID)
	c.JSON(http.StatusCreated, responseOf(t))
}

// 查询任务：GET /v1/tasks/:id
func (h *Handler) getTask(c *gin.Context) {
	id := c.Param("id")

	if !validID(id) {
		fail(c, http.StatusBadRequest,
			"INVALID_ARGUMENT", "invalid task id")
		return
	}

	t, err := h.store.Get(c.Request.Context(), id)

	if errors.Is(err, mysql.ErrNotFound) {
		fail(c, http.StatusNotFound,
			"NOT_FOUND", "task not found")
		return
	}
	if err != nil {
		slog.Error("get task", "err", err)
		fail(c, http.StatusInternalServerError,
			"INTERNAL", "failed to get task")
		return
	}

	c.JSON(http.StatusOK, responseOf(t))
}

// Worker 内部 Token 鉴权
func (h *Handler) checkWorkerToken(c *gin.Context) {
	token := c.GetHeader("X-Worker-Token")

	if h.workerToken == "" ||
		subtle.ConstantTimeCompare(
			[]byte(token),
			[]byte(h.workerToken),
		) != 1 {
		fail(c, http.StatusUnauthorized,
			"UNAUTHORIZED", "invalid worker token")
		c.Abort()
		return
	}

	c.Next()
}

// 领取任务：POST /v1/internal/tasks/claim
func (h *Handler) claimTask(c *gin.Context) {
	var req claimRequest
	if !decodeJSON(c, &req) {
		return
	}

	if strings.TrimSpace(req.TaskType) == "" ||
		len(req.TaskType) > 128 {
		fail(c, http.StatusBadRequest,
			"INVALID_ARGUMENT",
			"task_type must be 1..128 characters")
		return
	}

	t, err := h.store.ClaimNext(
		c.Request.Context(), req.TaskType,
	)

	if errors.Is(err, mysql.ErrNoReadyTask) {
		c.Status(http.StatusNoContent)
		return
	}
	if err != nil {
		slog.Error("claim task", "err", err)
		fail(c, http.StatusInternalServerError,
			"INTERNAL", "failed to claim task")
		return
	}

	c.JSON(http.StatusOK, responseOf(t))
}

// 完成任务：POST /v1/internal/tasks/:id/finish
func (h *Handler) finishTask(c *gin.Context) {
	id := c.Param("id")

	if !validID(id) {
		fail(c, http.StatusBadRequest,
			"INVALID_ARGUMENT", "invalid task id")
		return
	}

	var req finishRequest
	if !decodeJSON(c, &req) {
		return
	}

	if req.Status != task.Success &&
		req.Status != task.Failed {
		fail(c, http.StatusBadRequest,
			"INVALID_ARGUMENT",
			"status must be success or failed")
		return
	}

	err := h.store.Finish(
		c.Request.Context(), id, req.Status,
	)

	if errors.Is(err, mysql.ErrConflict) {
		fail(c, http.StatusConflict,
			"STATE_CONFLICT",
			"task is not processing or does not exist")
		return
	}
	if err != nil {
		slog.Error("finish task", "err", err)
		fail(c, http.StatusInternalServerError,
			"INTERNAL", "failed to finish task")
		return
	}

	c.Status(http.StatusNoContent)
}

// 数据库就绪检查
func (h *Handler) readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		2*time.Second,
	)
	defer cancel()

	if h.ready == nil || h.ready(ctx) != nil {
		fail(c, http.StatusServiceUnavailable,
			"NOT_READY", "database unavailable")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}
