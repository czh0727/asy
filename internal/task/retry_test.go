package task

import (
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	cases := []struct {
		count int
		want  time.Duration
	}{
		{0, 2 * time.Second},
		{1, 4 * time.Second},
		{2, 8 * time.Second},
		{3, 16 * time.Second},
		{4, 32 * time.Second},
		{5, time.Minute},
		{1000, time.Minute},
	}

	for _, tc := range cases {
		got := RetryDelay(tc.count)
		if got != tc.want {
			t.Errorf(
				"RetryDelay(%d)=%v, want %v",
				tc.count, got, tc.want,
			)
		}
	}
}
