package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/czh0727/asy/taskutils/rpc/model"
)

func TestTaskRpcHoldTasks(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if r.Method != http.MethodPost ||
				r.URL.Path != model.HOLD_TASKS {
				t.Errorf(
					"unexpected request: %s %s",
					r.Method, r.URL.Path,
				)
				w.WriteHeader(http.StatusNotFound)
				return
			}

			var req model.HoldTasksReq

			if err := json.NewDecoder(
				r.Body,
			).Decode(&req); err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if req.TaskType != "lark" || req.Limit != 2 {
				t.Errorf("unexpected hold request: %+v", req)
			}

			w.Header().Set(
				"Content-Type", "application/json",
			)

			_ = json.NewEncoder(w).Encode(
				model.HoldTasksResp{
					RespComm: model.RespComm{
						Code: 0,
						Msg:  "ok",
					},
					TaskList: []*model.TaskData{
						{TaskId: "task-a"},
						{TaskId: "task-b"},
					},
				},
			)
		}),
	)
	defer server.Close()

	client := &TaskRpc{Host: server.URL}

	result, err := client.HoldTasks(
		&model.HoldTasksReq{
			TaskType: "lark",
			Limit:    2,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.TaskList) != 2 {
		t.Fatalf(
			"task count=%d, want 2",
			len(result.TaskList),
		)
	}
}

func TestTaskRpcScheduleCfg(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if r.URL.Path !=
				model.GET_TASK_SCHEDULE_CFG_SUFFIX {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.Header().Set(
				"Content-Type", "application/json",
			)

			_ = json.NewEncoder(w).Encode(
				model.GetTaskScheduleCfgListResp{
					RespComm: model.RespComm{
						Code: 0,
						Msg:  "ok",
					},
					ScheduleCfgList: []*model.TaskScheduleCfg{
						{
							TaskType:      "lark",
							ScheduleLimit: 100,
						},
					},
				},
			)
		}),
	)
	defer server.Close()

	client := &TaskRpc{Host: server.URL}

	result, err := client.GetTaskScheduleCfgList()
	if err != nil {
		t.Fatal(err)
	}

	if len(result.ScheduleCfgList) != 1 ||
		result.ScheduleCfgList[0].ScheduleLimit != 100 {
		t.Fatalf("unexpected schedule cfg: %+v", result)
	}
}

func TestTaskRpcBusinessError(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			w.Header().Set(
				"Content-Type", "application/json",
			)

			_ = json.NewEncoder(w).Encode(
				model.HoldTasksResp{
					RespComm: model.RespComm{
						Code: 8038,
						Msg:  "get task list from db failed",
					},
				},
			)
		}),
	)
	defer server.Close()

	client := &TaskRpc{Host: server.URL}

	_, err := client.HoldTasks(
		&model.HoldTasksReq{
			TaskType: "lark",
			Limit:    1,
		},
	)

	if err == nil {
		t.Fatal("expected business error")
	}
}
