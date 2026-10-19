
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/czh0727/asy/internal/storage/mysql"
	"github.com/czh0727/asy/internal/task"
	"github.com/gin-gonic/gin"
)

// fakeStore 模拟数据库存储。
type fakeStore struct {
	mu    sync.Mutex
	items map[string]*task.Task
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		items: make(map[string]*task.Task),
	}
}

func (f *fakeStore) Create(
	_ context.Context,
	t *task.Task,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.items[t.ID]; ok {
		return errors.New("duplicate")
	}

	cp := *t
	f.items[t.ID] = &cp
	return nil
}

func (f *fakeStore) Get(
	_ context.Context,
	id string,
) (*task.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	t, ok := f.items[id]
	if !ok {
		return nil, mysql.ErrNotFound
	}

	cp := *t
	return &cp, nil
}

func (f *fakeStore) ClaimNext(
	_ context.Context,
	typ string,
) (*task.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, t := range f.items {
		if t.Type != typ ||
			!t.Runnable(time.Now()) {
			continue
		}

		_ = t.Transition(task.Processing)
		cp := *t
		return &cp, nil
	}

	return nil, mysql.ErrNoReadyTask
}

func (f *fakeStore) Finish(
	_ context.Context,
	id string,
	to task.Status,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	t := f.items[id]
	if t == nil || t.Status != task.Processing {
		return mysql.ErrConflict
	}

	return t.Transition(to)
}

// call 模拟一次 HTTP 请求。
func call(
	r http.Handler,
	method string,
	path string,
	body string,
	token string,
) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()

	req := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)

	if body != "" {
		req.Header.Set(
			"Content-Type",
			"application/json",
		)
	}
	if token != "" {
		req.Header.Set("X-Worker-Token", token)
	}

	r.ServeHTTP(w, req)
	return w
}

// 测试完整 HTTP 任务生命周期。
func TestHTTPTaskLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := newFakeStore()
	r := NewHandler(
		store,
		"local-secret",
		func(context.Context) error { return nil },
	).Router()

	// 1. 创建任务
	w := call(
		r, "POST", "/v1/tasks",
		`{"task_type":"email.send","payload":{"to":"a@b.com"},"max_retries":3}`,
		"",
	)

	if w.Code != http.StatusCreated {
		t.Fatalf(
			"create code=%d body=%s", w.Code, w.Body,
		)
	}

	var created taskResponse
	if err := json.NewDecoder(
		bytes.NewReader(w.Body.Bytes()),
	).Decode(&created); err != nil {
		t.Fatal(err)
	}

	if created.ID == "" || created.Status != task.Pending {
		t.Fatalf("created: %+v", created)
	}

	// 2. 查询任务
	w = call(
		r, "GET",
		"/v1/tasks/"+created.ID,
		"", "",
	)
	if w.Code != http.StatusOK {
		t.Fatalf(
			"get code=%d body=%s", w.Code, w.Body,
		)
	}

	// 3. 没有 Token 不能领取
	claimBody := `{"task_type":"email.send"}`

	w = call(
		r, "POST",
		"/v1/internal/tasks/claim",
		claimBody, "",
	)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing token=%d", w.Code)
	}

	// 4. 正确 Token 可以领取
	w = call(
		r, "POST",
		"/v1/internal/tasks/claim",
		claimBody, "local-secret",
	)
	if w.Code != http.StatusOK {
		t.Fatalf(
			"claim code=%d body=%s", w.Code, w.Body,
		)
	}

	var claimed taskResponse
	if err := json.NewDecoder(
		bytes.NewReader(w.Body.Bytes()),
	).Decode(&claimed); err != nil {
		t.Fatal(err)
	}

	if claimed.ID != created.ID ||
		claimed.Status != task.Processing {
		t.Fatalf("claimed: %+v", claimed)
	}

	// 5. 再次领取，应该无可执行任务
	w = call(
		r, "POST",
		"/v1/internal/tasks/claim",
		claimBody, "local-secret",
	)
	if w.Code != http.StatusNoContent {
		t.Fatalf(
			"second claim code=%d body=%s",
			w.Code, w.Body,
		)
	}

	finishPath := "/v1/internal/tasks/" +
		created.ID + "/finish"

	// 6. 非法终态不能提交
	w = call(
		r, "POST",
		finishPath,
		`{"status":"pending"}`,
		"local-secret",
	)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid finish code=%d", w.Code)
	}

	// 7. 完成任务
	w = call(
		r, "POST",
		finishPath,
		`{"status":"success"}`,
		"local-secret",
	)
	if w.Code != http.StatusNoContent {
		t.Fatalf(
			"finish code=%d body=%s", w.Code, w.Body,
		)
	}

	// 8. 不能重复完成
	w = call(
		r, "POST",
		finishPath,
		`{"status":"success"}`,
		"local-secret",
	)
	if w.Code != http.StatusConflict {
		t.Fatalf(
			"repeat finish code=%d body=%s",
			w.Code, w.Body,
		)
	}

	// 9. 查询最终状态
	w = call(
		r, "GET",
		"/v1/tasks/"+created.ID,
		"", "",
	)

	var done taskResponse
	if err := json.NewDecoder(
		bytes.NewReader(w.Body.Bytes()),
	).Decode(&done); err != nil {
		t.Fatal(err)
	}

	if done.Status != task.Success {
		t.Fatalf("done status=%s", done.Status)
	}
}

func TestBadInputsAndReady(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewHandler(
		newFakeStore(),
		"secret",
		func(context.Context) error {
			return errors.New("database down")
		},
	).Router()

	tests := []struct {
		method string
		path   string
		body   string
		want   int
	}{
		{"POST", "/v1/tasks",
			`{"task_type":""}`, 400},
		{"POST", "/v1/tasks",
			`{"task_type":"x","unknown":1}`, 400},
		{"POST", "/v1/tasks",
			`{"task_type":"x"} {}`, 400},
		{"GET", "/v1/tasks/not-an-id", "", 400},
		{"GET", "/v1/tasks/123456789012345678901234567890ab", "", 404},
		{"GET", "/readyz", "", 503},
		{"GET", "/healthz", "", 200},
	}

	for _, tt := range tests {
		w := call(
			r, tt.method, tt.path, tt.body, "",
		)
		if w.Code != tt.want {
			t.Errorf(
				"%s %s got %d, want %d",
				tt.method, tt.path, w.Code, tt.want,
			)
		}
	}
}
