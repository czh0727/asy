package task

import (
	"testing"
	"time"
)

func TestCalcInitialOrderTime(t *testing.T) {
	oldTimeNow := timeNow

	t.Cleanup(func() {
		timeNow = oldTimeNow
	})

	timeNow = func() time.Time {
		return time.Unix(1000, 0)
	}

	priority := 50

	cases := []struct {
		name      string
		processAt int64
		priority  *int
		want      int64
	}{
		{
			name: "immediate without priority",
			want: 1000,
		},
		{
			name:     "immediate with priority",
			priority: &priority,
			want:     950,
		},
		{
			name:      "past processAt with priority",
			processAt: 900,
			priority:  &priority,
			want:      950,
		},
		{
			name:      "future processAt ignores priority",
			processAt: 1200,
			priority:  &priority,
			want:      1200,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := calcInitialOrderTime(
				tc.processAt,
				tc.priority,
			)

			if got != tc.want {
				t.Fatalf(
					"order time=%d want=%d",
					got, tc.want,
				)
			}
		})
	}
}
