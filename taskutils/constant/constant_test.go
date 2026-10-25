package constant

import "testing"

func TestOriginalTaskStatusValues(t *testing.T) {
	cases := []struct {
		status TaskEnum
		want   int
	}{
		{TASK_STATUS_PENDING, 1},
		{TASK_STATUS_PROCESSING, 2},
		{TASK_STATUS_SUCC, 3},
		{TASK_STATUS_FAILED, 4},
	}

	for _, tc := range cases {
		if int(tc.status) != tc.want {
			t.Errorf(
				"status=%d, want=%d",
				tc.status, tc.want,
			)
		}
	}
}
