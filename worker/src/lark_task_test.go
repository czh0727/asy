package main

import (
	"encoding/json"
	"testing"

	"github.com/czh0727/asy/taskutils/constant"
)

func TestLarkTaskContextLoad(t *testing.T) {
	contextData := LarkTaskContext{
		ReqBody: &LarkReq{
			Msg:      "hello",
			FromAddr: "fish",
			ToAddr:   "cat",
		},
		UserId: "student",
	}

	data, err := json.Marshal(contextData)
	if err != nil {
		t.Fatal(err)
	}

	job := &LarkTask{}
	job.TaskContext = string(data)

	if err := job.ContextLoad(); err != nil {
		t.Fatal(err)
	}

	if job.ContextData == nil ||
		job.ContextData.ReqBody == nil ||
		job.ContextData.ReqBody.Msg != "hello" {
		t.Fatalf(
			"context=%+v",
			job.ContextData,
		)
	}
}

func TestLarkTaskHandleProcess(t *testing.T) {
	job := &LarkTask{}
	job.TaskStage = "sendmsg"

	job.TaskContext = `{
		"ReqBody":{
			"Msg":"hello",
			"FromAddr":"fish",
			"ToAddr":"cat"
		},
		"UserId":"student"
	}`

	if err := job.ContextLoad(); err != nil {
		t.Fatal(err)
	}

	if err := job.HandleProcess(); err != nil {
		t.Fatal(err)
	}

	if job.TaskStage != "record" {
		t.Fatalf("stage=%s", job.TaskStage)
	}

	if job.Status != int(constant.TASK_STATUS_SUCC) {
		t.Fatalf("status=%d", job.Status)
	}

	if job.ContextIntf == nil {
		t.Fatal("SetContextLocal was not called")
	}
}

func TestLarkTaskInvalidStage(t *testing.T) {
	job := &LarkTask{}
	job.TaskStage = "unknown"
	job.TaskContext = "{}"

	if err := job.ContextLoad(); err != nil {
		t.Fatal(err)
	}

	if err := job.HandleProcess(); err != nil {
		t.Fatal(err)
	}

	if job.Status != int(constant.TASK_STATUS_FAILED) {
		t.Fatalf("status=%d", job.Status)
	}
}

func TestLarkTaskInvalidContext(t *testing.T) {
	job := &LarkTask{}
	job.TaskContext = "{invalid json"

	if err := job.ContextLoad(); err == nil {
		t.Fatal("expected context parse error")
	}
}
