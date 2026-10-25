package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCreateTaskJSONContract(t *testing.T) {
	priority := 0

	req := CreateTaskReq{TaskData: TaskData{
		TaskType:    "lark",
		TaskStage:   "sendmsg",
		Priority:    &priority,
		TaskContext: `{"msg":"hello"}`,
	}}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(
		doc["task_data"], &payload,
	); err != nil {
		t.Fatal(err)
	}

	// 显式传入 0 与 nil 不一样。
	if string(payload["priority"]) != "0" {
		t.Fatalf(
			"priority=%s, want 0",
			payload["priority"],
		)
	}

	// 原始协议使用 context，而不是 task_context。
	if string(payload["context"]) !=
		`"{\"msg\":\"hello\"}"` {
		t.Fatalf(
			"context=%s",
			payload["context"],
		)
	}

	if _, ok := payload["task_context"]; ok {
		t.Fatal(
			"unexpected task_context key",
		)
	}

	var parsed CreateTaskReq
	if err := json.Unmarshal(
		data, &parsed,
	); err != nil {
		t.Fatal(err)
	}

	if parsed.TaskData.Priority == nil ||
		*parsed.TaskData.Priority != 0 ||
		parsed.TaskData.TaskStage != "sendmsg" {
		t.Fatalf(
			"unexpected decoded request: %+v",
			parsed,
		)
	}
}

func TestSetTaskUsesNestedTaskData(t *testing.T) {
	data, err := json.Marshal(
		SetTaskReq{TaskData: TaskData{
			TaskId: "id-001",
			Status: 3,
		}},
	)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(
		string(data), `"task_data":{`,
	) || !strings.Contains(
		string(data), `"task_id":"id-001"`,
	) {
		t.Fatalf(
			"wrong SetTask JSON: %s",
			data,
		)
	}
}

func TestHoldTasksResponseAndConfig(t *testing.T) {
	resp := HoldTasksResp{
		RespComm: RespComm{
			Code: 0,
			Msg:  "ok",
		},
		TaskList: []*TaskData{{
			TaskId: "x",
			Status: 2,
		}},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	var parsed HoldTasksResp
	if err := json.Unmarshal(
		data, &parsed,
	); err != nil {
		t.Fatal(err)
	}

	if parsed.Code != 0 ||
		len(parsed.TaskList) != 1 ||
		parsed.TaskList[0].Status != 2 {
		t.Fatalf(
			"unexpected decoded response: %+v",
			parsed,
		)
	}

	cfgResp := GetTaskScheduleCfgListResp{
		ScheduleCfgList: []*TaskScheduleCfg{{
			TaskType:      "lark",
			ScheduleLimit: 100,
			MaxRetryNum:   3,
		}},
	}

	cfgData, err := json.Marshal(cfgResp)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(
		string(cfgData),
		`"task_schedule_cfg_list"`,
	) {
		t.Fatalf(
			"config JSON key mismatch: %s",
			cfgData,
		)
	}
}

func TestEndpointConstants(t *testing.T) {
	if CREATE_TASK_SUFFIX != "/v1/create_task" ||
		HOLD_TASKS != "/v1/hold_tasks" ||
		SET_TASK_SUFFIX != "/v1/set_task" ||
		GET_TASK_TYPE_STATS_SUFFIX != "/v1/get_task_type_stats" {
		t.Fatal(
			"original Asyncflow endpoint constants changed",
		)
	}
}
