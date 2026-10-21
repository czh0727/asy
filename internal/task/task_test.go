package task

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewTask(t *testing.T) {
	payload := json.RawMessage(`{"prompt":"cat"}`)

	got, err := NewTask(
		"video.generate",
		payload,
		10,
		time.Time{},
		3,
	)
	if err != nil {
		t.Fatal(err)
	}

	if got.ID == "" ||
		got.Type != "video.generate" ||
		got.Status != Pending ||
		got.MaxRetries != 3 {
		t.Fatalf("unexpected task: %+v", got)
	}

	payload[0] = 'x'

	if !json.Valid(got.Payload) {
		t.Fatal("payload must be copied, not aliased")
	}
}

func TestNewTaskRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name       string
		taskType   string
		payload    json.RawMessage
		priority   int
		maxRetries int
	}{
		{"empty type", "", nil, 0, 0},
		{"bad JSON", "email", json.RawMessage(`{bad}`), 0, 0},
		{"negative priority", "email", nil, -1, 0},
		{"priority too large", "email", nil, 101, 0},
		{"negative retries", "email", nil, 0, -1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewTask(
				tc.taskType,
				tc.payload,
				tc.priority,
				time.Time{},
				tc.maxRetries,
			)

			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestCanTransition(t *testing.T) {
	cases := []struct {
		from Status
		to   Status
		want bool
	}{
		{Pending, Processing, true},
		{Processing, Pending, true},
		{Processing, Success, true},
		{Processing, Failed, true},
		{Pending, Success, false},
		{Pending, Pending, false},
		{Success, Processing, false},
		{Failed, Pending, false},
		{Status("unknown"), Pending, false},
	}

	for _, tc := range cases {
		got := CanTransition(tc.from, tc.to)

		if got != tc.want {
			t.Errorf(
				"CanTransition(%q,%q)=%v, want %v",
				tc.from,
				tc.to,
				got,
				tc.want,
			)
		}
	}
}

func TestTransitionAndRunnable(t *testing.T) {
	now := time.Now()

	task, err := NewTask(
		"email.send",
		nil,
		0,
		now.Add(time.Hour),
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	// 延迟任务未到执行时间
	if task.Runnable(now) {
		t.Fatal("delayed task must not be runnable yet")
	}

	// 到达执行时间
	if !task.Runnable(now.Add(2 * time.Hour)) {
		t.Fatal("pending task should be runnable after due time")
	}

	// pending -> processing
	if err := task.Transition(Processing); err != nil {
		t.Fatal(err)
	}

	if task.Runnable(now.Add(2 * time.Hour)) {
		t.Fatal("processing task must not be runnable")
	}

	// processing -> success
	if err := task.Transition(Success); err != nil {
		t.Fatal(err)
	}

	// success 不能重新进入 processing
	if err := task.Transition(Processing); err == nil ||
		task.Status != Success {
		t.Fatal("a completed task cannot be restarted")
	}
}
