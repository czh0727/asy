package ctrlmodel

import (
	"strings"
	"testing"

	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/czh0727/asy/taskutils/rpc/model"
)

func TestFillTaskModelAndResp(t *testing.T) {
	priority := 10

	source := &model.TaskData{
		UserId:           "user-1",
		TaskType:         "lark",
		TaskStage:        "sendmsg",
		Priority:         &priority,
		TaskContext:      `{"msg":"hello"}`,
		MaxRetryNum:      3,
		MaxRetryInterval: 5,
		OrderTime:        1000,
	}

	var stored db.Task

	err := FillTaskModel(source, &stored, "1")
	if err != nil {
		t.Fatal(err)
	}

	if stored.Status != int(db.TASK_STATUS_PENDING) {
		t.Fatalf("status=%d", stored.Status)
	}

	if !strings.HasSuffix(stored.TaskId, "_lark_1") {
		t.Fatalf("task ID=%s", stored.TaskId)
	}

	if stored.TaskStage != "sendmsg" ||
		stored.Priority != 10 ||
		stored.TaskContext != `{"msg":"hello"}` {
		t.Fatalf("unexpected task: %+v", stored)
	}

	var output model.TaskData
	FillTaskResp(&stored, &output)

	if output.TaskId != stored.TaskId ||
		output.TaskStage != source.TaskStage ||
		output.Priority == nil ||
		*output.Priority != 10 {
		t.Fatalf("wrong response: %+v", output)
	}
}
