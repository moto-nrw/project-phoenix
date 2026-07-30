package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithinWindow(t *testing.T) {
	tests := []struct {
		name            string
		now, start, end int
		want            bool
	}{
		{name: "normal before", now: 5 * 60, start: 6 * 60, end: 18 * 60, want: false},
		{name: "normal start inclusive", now: 6 * 60, start: 6 * 60, end: 18 * 60, want: true},
		{name: "normal end exclusive", now: 18 * 60, start: 6 * 60, end: 18 * 60, want: false},
		{name: "overnight evening", now: 22 * 60, start: 18 * 60, end: 6 * 60, want: true},
		{name: "overnight morning", now: 2 * 60, start: 18 * 60, end: 6 * 60, want: true},
		{name: "overnight daytime", now: 12 * 60, start: 18 * 60, end: 6 * 60, want: false},
		{name: "equal bounds mean all day", now: 12 * 60, start: 6 * 60, end: 6 * 60, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, withinWindow(tt.now, tt.start, tt.end))
		})
	}
}
