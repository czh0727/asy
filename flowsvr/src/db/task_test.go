package db

import (
	"strings"
	"testing"
)

func TestTaskIDEncodeDecode(t *testing.T) {
	taskType := "video_generate"

	id := TaskNsp.GenTaskId(taskType, "3")

	gotType, gotPos, err := ParseTaskID(id)
	if err != nil {
		t.Fatal(err)
	}

	if gotType != taskType || gotPos != 3 {
		t.Fatalf(
			"type=%s pos=%d",
			gotType, gotPos,
		)
	}

	table, err := TaskTableName(gotType, gotPos)
	if err != nil {
		t.Fatal(err)
	}

	if table != "t_video_generate_task_3" {
		t.Fatalf("unexpected table: %s", table)
	}
}

func TestInvalidTaskType(t *testing.T) {
	cases := []string{
		"",
		"video-generate",
		"video generate",
		"video;DROP TABLE tasks",
		strings.Repeat("a", 49),
	}

	for _, taskType := range cases {
		if err := ValidateTaskType(taskType); err == nil {
			t.Errorf(
				"expected invalid task type: %q",
				taskType,
			)
		}
	}
}

func TestInvalidTaskID(t *testing.T) {
	cases := []string{
		"",
		"invalid",
		"abc_lark_1",
		"f6932999-e046-4fa9-9ed3-712243700abf_lark_0",
		"f6932999-e046-4fa9-9ed3-712243700abf_lark_bad",
	}

	for _, id := range cases {
		if _, _, err := ParseTaskID(id); err == nil {
			t.Errorf(
				"expected invalid task id: %s", id,
			)
		}
	}
}
