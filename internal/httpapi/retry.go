package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/czh0727/asy/internal/storage/mysql"
	"github.com/gin-gonic/gin"
)

// retryTask 处理 Worker 上报的可重试失败。
func (h *Handler) retryTask(c *gin.Context) {
	id := c.Param("id")

	if !validID(id) {
		fail(
			c,
			http.StatusBadRequest,
			"INVALID_ARGUMENT",
			"invalid task id",
		)
		return
	}

	result, err := h.store.Retry(
		c.Request.Context(),
		id,
	)

	if errors.Is(err, mysql.ErrConflict) {
		fail(
			c,
			http.StatusConflict,
			"STATE_CONFLICT",
			"task is not in a retryable processing state",
		)
		return
	}

	if err != nil {
		slog.Error(
			"retry task",
			"id", id,
			"err", err,
		)
		fail(
			c,
			http.StatusInternalServerError,
			"INTERNAL",
			"failed to update retry state",
		)
		return
	}

	c.JSON(http.StatusOK, result)
}
