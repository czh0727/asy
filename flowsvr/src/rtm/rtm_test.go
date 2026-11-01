package rtm

import (
	"testing"

	"github.com/czh0727/asy/flowsvr/src/db"
)

func TestGetMaxProcessTime(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *db.TaskScheduleCfg
		fallback int64
		want     int64
	}{
		{
			name: "use task config",
			cfg: &db.TaskScheduleCfg{
				MaxProcessingTime: 30,
			},
			fallback: 10,
			want:     30,
		},
		{
			name:     "use global fallback",
			cfg:      &db.TaskScheduleCfg{},
			fallback: 10,
			want:     10,
		},
		{
			name:     "nil task config",
			cfg:      nil,
			fallback: 10,
			want:     10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMaxProcessTime(
				tt.cfg,
				tt.fallback,
			)

			if got != tt.want {
				t.Fatalf(
					"got %d, want %d",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestShouldFailTimeoutTask(t *testing.T) {
	tests := []struct {
		name    string
		current int
		max     int
		want    bool
	}{
		{
			name:    "retry available",
			current: 0,
			max:     3,
			want:    false,
		},
		{
			name:    "last retry available",
			current: 2,
			max:     3,
			want:    false,
		},
		{
			name:    "retry exhausted",
			current: 3,
			max:     3,
			want:    true,
		},
		{
			name:    "retry exceeded",
			current: 4,
			max:     3,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldFailTimeoutTask(
				tt.current,
				tt.max,
			)

			if got != tt.want {
				t.Fatalf(
					"got %v, want %v",
					got,
					tt.want,
				)
			}
		})
	}
}
