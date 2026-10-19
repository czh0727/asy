
package tasksdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/czh0727/asy/internal/task"
)

type fakeClient struct {
	mu        sync.Mutex
	queue     []*ClaimedTask
	finished  map[string]task.Status
	finishErr error
}

func (f *fakeClient) Claim(
	_ context.Context,
	_ string,
) (*ClaimedTask, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.queue) == 0 {
		return nil, nil
	}

	v := f.queue[0]
	f.queue = f.queue[1:]
	return v, nil
}

func (f *fakeClient) Finish(
	_ context.Context,
	id string,
	status task.Status,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.finishErr != nil {
		return f.finishErr
	}

	f.finished[id] = status
	return nil
}

func newTestWorker(
	t *testing.T,
	handle HandlerFunc,
	client *fakeClient,
) *Worker {
	t.Helper()

	r := NewRegistry()
	if err := r.Register("video.generate", handle); err != nil {
		t.Fatal(err)
	}

	w, err := NewWorker(
		client, r, "video.generate", time.Millisecond,
	)
	if err != nil {
		t.Fatal(err)
	}

	return w
}

func TestRunOnceSuccessFailurePanic(t *testing.T) {
	cases := []struct {
		name      string
		handle    HandlerFunc
		want      task.Status
		wantError bool
	}{
		{
			"success",
			func(context.Context, json.RawMessage) error {
				return nil
			},
			task.Success,
			false,
		},
		{
			"failure",
			func(context.Context, json.RawMessage) error {
				return errors.New("bad input")
			},
			task.Failed,
			true,
		},
		{
			"panic",
			func(context.Context, json.RawMessage) error {
				panic("oops")
			},
			task.Failed,
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := strings.Repeat("a", 32)

			f := &fakeClient{
				queue: []*ClaimedTask{{
					ID:       id,
					TaskType: "video.generate",
					Status:   task.Processing,
				}},
				finished: make(map[string]task.Status),
			}

			w := newTestWorker(t, tc.handle, f)

			got, err := w.RunOnce(context.Background())
			if !got || (err != nil) != tc.wantError {
				t.Fatalf("got=%v err=%v", got, err)
			}

			if f.finished[id] != tc.want {
				t.Fatalf(
					"finished=%s want=%s",
					f.finished[id], tc.want,
				)
			}

			noJob, err := w.RunOnce(context.Background())
			if noJob || err != nil {
				t.Fatalf(
					"expected no task, got=%v %v",
					noJob, err,
				)
			}
		})
	}
}

func TestFinishErrorLeavesUncertainResult(t *testing.T) {
	f := &fakeClient{
		queue: []*ClaimedTask{{
			ID:       strings.Repeat("b", 32),
			TaskType: "video.generate",
		}},
		finishErr: errors.New("network timeout"),
	}

	w := newTestWorker(
		t,
		func(context.Context, json.RawMessage) error {
			return nil
		},
		f,
	)

	got, err := w.RunOnce(context.Background())

	if !got || err == nil ||
		!strings.Contains(err.Error(), "uncertain") {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	f := HandlerFunc(
		func(context.Context, json.RawMessage) error {
			return nil
		},
	)

	if err := r.Register("x", f); err != nil {
		t.Fatal(err)
	}

	if err := r.Register("x", f); err == nil {
		t.Fatal("duplicate should fail")
	}

	if _, ok := r.Get("x"); !ok {
		t.Fatal("missing handler")
	}
}

func TestHTTPClientClaimFinish(t *testing.T) {
	id := strings.Repeat("a", 32)

	var mu sync.Mutex
	finishes := 0

	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Worker-Token") != "secret" {
					w.WriteHeader(401)
					return
				}

				if r.Method != "POST" {
					w.WriteHeader(405)
					return
				}

				switch r.URL.Path {
				case "/v1/internal/tasks/claim":
					var body struct {
						TaskType string `json:"task_type"`
					}

					err := json.NewDecoder(r.Body).Decode(&body)

					if err != nil ||
						body.TaskType != "video.generate" {
						w.WriteHeader(400)
						return
					}

					w.Header().Set(
						"Content-Type",
						"application/json",
					)

					fmt.Fprintf(
						w,
						`{"id":"%s","task_type":"video.generate","status":"processing","payload":{"prompt":"cat"}}`,
						id,
					)

				case "/v1/internal/tasks/" + id + "/finish":
					var body struct {
						Status task.Status `json:"status"`
					}

					err := json.NewDecoder(r.Body).Decode(&body)

					if err != nil || body.Status != task.Success {
						w.WriteHeader(400)
						return
					}

					mu.Lock()
					finishes++
					mu.Unlock()

					w.WriteHeader(http.StatusNoContent)

				default:
					w.WriteHeader(404)
				}
			},
		),
	)
	defer srv.Close()

	c, err := NewHTTPClient(srv.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}

	got, err := c.Claim(
		context.Background(),
		"video.generate",
	)

	if err != nil || got == nil || got.ID != id {
		t.Fatalf("claim=%+v err=%v", got, err)
	}

	err = c.Finish(
		context.Background(), id, task.Success,
	)
	if err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()

	if finishes != 1 {
		t.Fatalf("finishes=%d", finishes)
	}
}

func TestHTTPClientEmptyAndUnauthorized(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Worker-Token") != "secret" {
					w.WriteHeader(401)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			},
		),
	)
	defer srv.Close()

	good, _ := NewHTTPClient(srv.URL, "secret")

	got, err := good.Claim(
		context.Background(), "x",
	)
	if got != nil || err != nil {
		t.Fatalf(
			"want empty result, got=%v err=%v",
			got, err,
		)
	}

	bad, _ := NewHTTPClient(srv.URL, "bad")

	_, err = bad.Claim(
		context.Background(), "x",
	)

	if err == nil ||
		!strings.Contains(err.Error(), "401") {
		t.Fatalf("want unauthorized: %v", err)
	}
}

func TestWorkerStopsOnCancel(t *testing.T) {
	f := &fakeClient{}

	w := newTestWorker(
		t,
		func(context.Context, json.RawMessage) error {
			return nil
		},
		f,
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Millisecond,
	)
	defer cancel()

	if err := w.Run(ctx); err != nil {
		t.Fatal(err)
	}
}
