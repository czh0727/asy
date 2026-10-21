package tasksdk

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/czh0727/asy/internal/task"
)

// 可重试错误应调用 Retry，而不是 Finish(failed)。
func TestWorkerSchedulesRetry(t *testing.T) {
	id := strings.Repeat("a", 32)

	f := &fakeClient{
		queue: []*ClaimedTask{{
			ID:       id,
			TaskType: "video.generate",
			Status:   task.Processing,
		}},
		finished: make(map[string]task.Status),
	}

	w := newTestWorker(
		t,
		func(
			context.Context,
			json.RawMessage,
		) error {
			return Retryable(
				errors.New("temporary upstream error"),
			)
		},
		f,
	)

	claimed, err := w.RunOnce(context.Background())
	if err != nil || !claimed {
		t.Fatalf(
			"claimed=%v err=%v", claimed, err,
		)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.retries) != 1 || f.retries[0] != id {
		t.Fatalf("unexpected retries: %v", f.retries)
	}

	if len(f.finished) != 0 {
		t.Fatal(
			"retryable error must not call Finish",
		)
	}
}

// HTTPClient.Retry 应正确解析 flowsvr 返回的结果。
func TestHTTPClientRetry(t *testing.T) {
	id := strings.Repeat("b", 32)

	srv := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if r.Header.Get("X-Worker-Token") != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if r.Method != http.MethodPost ||
				r.URL.Path != "/v1/internal/tasks/"+id+"/retry" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.Header().Set(
				"Content-Type",
				"application/json",
			)

			_ = json.NewEncoder(w).Encode(
				task.RetryResult{
					Status:     task.Pending,
					RetryCount: 1,
					MaxRetries: 3,
					OrderTime: time.Now().Add(
						2 * time.Second,
					),
				},
			)
		}),
	)
	defer srv.Close()

	client, err := NewHTTPClient(srv.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.Retry(
		context.Background(),
		id,
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != task.Pending ||
		result.RetryCount != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
