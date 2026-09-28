package presence

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetermineActivityStatusUnlimitedStaysActive(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "active", determineActivityStatus(1000, 0))
}

// TestDetermineActivityStatusAgainstLimit pins the dashboard states around a
// limit of 45: the existing meanings for occupancy up to the limit, and
// "overbooked" once the session holds more children than the limit (#3634).
func TestDetermineActivityStatusAgainstLimit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		participants int
		want         string
	}{
		{name: "empty", participants: 0, want: "active"},
		{name: "at 80 percent", participants: 36, want: "active"},
		{name: "above 80 percent", participants: 37, want: "ending_soon"},
		{name: "one below the limit", participants: 44, want: "ending_soon"},
		{name: "at the limit", participants: 45, want: "full"},
		{name: "one above the limit", participants: 46, want: "overbooked"},
		{name: "production case", participants: 66, want: "overbooked"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, determineActivityStatus(tc.participants, 45))
		})
	}
}
