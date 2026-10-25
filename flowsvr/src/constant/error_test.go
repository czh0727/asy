package constant

import (
	"strings"
	"testing"
)

func TestGetErrMsg(t *testing.T) {
	if GetErrMsg(SUCCESS) != "ok" {
		t.Fatal("unexpected success message")
	}

	if GetErrMsg(ERR_CREATE_TASK) !=
		"create task failed" {
		t.Fatal("unexpected create error message")
	}

	if !strings.Contains(
		GetErrMsg(99999), "99999",
	) {
		t.Fatal(
			"unknown code must include its number",
		)
	}
}
