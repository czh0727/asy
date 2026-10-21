package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/czh0727/asy/internal/task"
	"github.com/gin-gonic/gin"
)

func TestRetryHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := newFakeStore()

	r := NewHandler(
		store,
		"secret",
		func(context.Context) error { return nil },
	).Router()

	// 创建任务
	w := call(
		r,
		http.MethodPost,
		"/v1/tasks",
		`{"task_type":"email.send","max_retries":1}`,
		"",
	)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}

	var created taskResponse
	if err := json.Unmarshal(
		w.Body.Bytes(),
		&created,
	); err != nil {
		t.Fatal(err)
	}

	// 领取任务
	w = call(
		r,
		http.MethodPost,
		"/v1/internal/tasks/claim",
		`{"task_type":"email.send"}`,
		"secret",
	)
	if w.Code != http.StatusOK {
		t.Fatalf("claim: %d %s", w.Code, w.Body)
	}

	path := "/v1/internal/tasks/" +
		created.ID + "/retry"

	// 未授权的 Retry 应该被拒绝
	w = call(r, http.MethodPost, path, "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized retry: %d", w.Code)
	}

	// 正确上报可重试失败
	w = call(r, http.MethodPost, path, "", "secret")
	if w.Code != http.StatusOK {
		t.Fatalf("retry: %d %s", w.Code, w.Body)
	}

	var result task.RetryResult
	if err := json.Unmarshal(
		w.Body.Bytes(),
		&result,
	); err != nil {
		t.Fatal(err)
	}

	if result.Status != task.Pending ||
		result.RetryCount != 1 {
		t.Fatalf("retry result: %+v", result)
	}

	// 同一次失败不能重复 Retry
	w = call(r, http.MethodPost, path, "", "secret")
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate retry: %d", w.Code)
	}
}
