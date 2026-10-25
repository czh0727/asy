package tasksdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/czh0727/asy/taskutils/constant"
	"github.com/czh0727/asy/taskutils/rpc/model"
)

// 测试业务任务，通过嵌入 TaskBase
// 自动获得大部分 TaskIntf 方法。
type testTask struct {
	TaskBase
}

func (p *testTask) HandleProcess() error {
	return nil
}

func (p *testTask) HandleFinish() {}

// resetHandlers 避免单元测试修改其他测试的任务注册表。
func resetHandlers(t *testing.T) {
	t.Helper()

	old := taskHandlerMap
	taskHandlerMap = make(map[string]*TaskHandler)

	t.Cleanup(func() {
		taskHandlerMap = old
	})
}

func TestRegisterHandler(t *testing.T) {
	resetHandlers(t)

	if err := RegisterHandler(nil); err == nil {
		t.Fatal("nil handler should fail")
	}

	if err := RegisterHandler(
		&TaskHandler{TaskType: "lark"},
	); err == nil {
		t.Fatal("missing NewProc should fail")
	}

	handler := &TaskHandler{
		TaskType: "lark",
		NewProc: func() TaskIntf {
			return new(testTask)
		},
	}

	if err := RegisterHandler(handler); err != nil {
		t.Fatal(err)
	}

	if err := RegisterHandler(handler); err == nil {
		t.Fatal("duplicate handler should fail")
	}

	got, code := GetHandler("lark")
	if code != 0 || got == nil {
		t.Fatalf("handler=%v code=%d", got, code)
	}

	if _, code := GetHandler("unknown"); code != -1 {
		t.Fatalf("unknown handler code=%d", code)
	}
}

func TestGetTaskInfoFromStorage(t *testing.T) {
	resetHandlers(t)

	if err := RegisterHandler(&TaskHandler{
		TaskType: "lark",
		NewProc: func() TaskIntf {
			return new(testTask)
		},
	}); err != nil {
		t.Fatal(err)
	}

	priority := 20

	storage := &model.TaskData{
		TaskId:           "task-001",
		TaskType:         "lark",
		TaskStage:        "sendmsg",
		TaskContext:      `{"step":1}`,
		Status:           int(constant.TASK_STATUS_PROCESSING),
		Priority:         &priority,
		UserId:           "user-001",
		CrtRetryNum:      1,
		MaxRetryNum:      3,
		MaxRetryInterval: 10,
		ScheduleLog: `{
			"LastData": {
				"TraceId": "trace-001",
				"ErrMsg": "",
				"Cost": "10ms"
			},
			"HistoryDatas": []
		}`,
	}

	got, err := GetTaskInfoFromStorage(storage)
	if err != nil {
		t.Fatal(err)
	}

	base := got.Base()

	if base.TaskId != "task-001" ||
		base.TaskStage != "sendmsg" ||
		base.TaskContext != `{"step":1}` {
		t.Fatalf("task fields mismatch: %+v", base)
	}

	if base.Status != int(constant.TASK_STATUS_PROCESSING) ||
		base.Priority != 20 ||
		base.CrtRetryNum != 1 {
		t.Fatalf("task status mismatch: %+v", base)
	}

	if base.ScheduleLog == nil ||
		base.ScheduleLog.LastData.TraceId != "trace-001" {
		t.Fatalf(
			"schedule log mismatch: %+v",
			base.ScheduleLog,
		)
	}

	if _, ok := got.(*testTask); !ok {
		t.Fatalf("unexpected task type: %T", got)
	}
}

func TestGetTaskInfoFromStorageInvalid(t *testing.T) {
	resetHandlers(t)

	cases := []*model.TaskData{
		nil,
		{},
		{TaskType: "unknown"},
	}

	for _, item := range cases {
		if _, err := GetTaskInfoFromStorage(item); err == nil {
			t.Fatal("expected validation error")
		}
	}
}

func TestTaskBaseSetTaskRPC(t *testing.T) {
	received := make(chan model.SetTaskReq, 1)

	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if r.Method != http.MethodPost ||
				r.URL.Path != model.SET_TASK_SUFFIX {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			var req model.SetTaskReq

			if err := json.NewDecoder(
				r.Body,
			).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			received <- req

			w.Header().Set(
				"Content-Type", "application/json",
			)

			_ = json.NewEncoder(w).Encode(
				model.SetTaskResp{
					RespComm: model.RespComm{
						Code: 0,
						Msg:  "ok",
					},
				},
			)
		}),
	)
	defer server.Close()

	oldHost := taskSvrHost
	taskSvrHost = server.URL
	t.Cleanup(func() {
		taskSvrHost = oldHost
	})

	base := &TaskBase{
		TaskId:      "task-001",
		TaskType:    "lark",
		TaskStage:   "record",
		TaskContext: `{"step":1}`,
		Status:      int(constant.TASK_STATUS_SUCC),
		ScheduleLog: &ScheduleLog{
			LastData: ScheduleData{
				TraceId: "trace-002",
				Cost:    "120ms",
			},
		},
	}

	// 在内存中更新上下文。
	base.SetContextLocal(map[string]interface{}{
		"step": float64(2),
		"msg":  "hello",
	})

	if err := base.SetTask(); err != nil {
		t.Fatal(err)
	}

	request := <-received

	if request.TaskData.TaskId != "task-001" ||
		request.TaskData.Status != int(constant.TASK_STATUS_SUCC) ||
		request.TaskData.TaskStage != "record" {
		t.Fatalf(
			"unexpected SetTask request: %+v",
			request,
		)
	}

	var contextData map[string]interface{}
	if err := json.Unmarshal(
		[]byte(request.TaskData.TaskContext),
		&contextData,
	); err != nil {
		t.Fatal(err)
	}

	if contextData["step"] != float64(2) ||
		contextData["msg"] != "hello" {
		t.Fatalf("context=%v", contextData)
	}

	var scheduleLog ScheduleLog
	if err := json.Unmarshal(
		[]byte(request.TaskData.ScheduleLog),
		&scheduleLog,
	); err != nil {
		t.Fatal(err)
	}

	if scheduleLog.LastData.TraceId != "trace-002" {
		t.Fatalf("schedule log=%+v", scheduleLog)
	}
}

func TestTaskBaseCreateTaskRPC(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if r.URL.Path != model.CREATE_TASK_SUFFIX {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			var req model.CreateTaskReq
			if err := json.NewDecoder(
				r.Body,
			).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if req.TaskData.TaskType != "lark" ||
				req.TaskData.Status != int(
					constant.TASK_STATUS_PENDING,
				) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			_ = json.NewEncoder(w).Encode(
				model.CreateTaskResp{
					RespComm: model.RespComm{
						Code: 0,
						Msg:  "ok",
					},
					TaskId: "created-001",
				},
			)
		}),
	)
	defer server.Close()

	oldHost := taskSvrHost
	taskSvrHost = server.URL
	t.Cleanup(func() {
		taskSvrHost = oldHost
	})

	base := &TaskBase{
		TaskType:    "lark",
		TaskContext: "{}",
	}

	id, err := base.CreateTask()
	if err != nil {
		t.Fatal(err)
	}

	if id != "created-001" {
		t.Fatalf("task ID=%s", id)
	}
}

func TestTaskBaseGetTaskRPC(t *testing.T) {
	resetHandlers(t)

	if err := RegisterHandler(&TaskHandler{
		TaskType: "lark",
		NewProc: func() TaskIntf {
			return new(testTask)
		},
	}); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if r.Method != http.MethodGet ||
				r.URL.Path != model.GET_TASK_SUFFIX ||
				r.URL.Query().Get("task_id") != "task-001" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			_ = json.NewEncoder(w).Encode(
				model.GetTaskResp{
					RespComm: model.RespComm{
						Code: 0,
						Msg:  "ok",
					},
					TaskData: &model.TaskData{
						TaskId:    "task-001",
						TaskType:  "lark",
						TaskStage: "sendmsg",
					},
				},
			)
		}),
	)
	defer server.Close()

	oldHost := taskSvrHost
	taskSvrHost = server.URL
	t.Cleanup(func() {
		taskSvrHost = oldHost
	})

	task, err := GetTask("task-001")
	if err != nil {
		t.Fatal(err)
	}

	if task.Base().TaskType != "lark" ||
		task.Base().TaskStage != "sendmsg" {
		t.Fatalf(
			"unexpected task: %+v",
			task.Base(),
		)
	}

	if !strings.Contains(
		fmtTaskType(task), "testTask",
	) {
		t.Fatalf("unexpected concrete task: %T", task)
	}
}

func fmtTaskType(task TaskIntf) string {
	return strings.TrimPrefix(
		strings.TrimSpace(
			reflectTypeName(task),
		),
		"*",
	)
}

func reflectTypeName(task TaskIntf) string {
	switch task.(type) {
	case *testTask:
		return "testTask"
	default:
		return "unexpected"
	}
}
