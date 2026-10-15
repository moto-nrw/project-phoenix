package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The account cutoff is 1 January of last year (Berlin), the cutoff of the
// operator ledger. Right after New Year it stays a year older for an hour,
// so the ledger has always lost an account's entries first (#4047).
func TestOperatorDeletionCutoffFollowsTheLedgerWindow(t *testing.T) {
	t.Parallel()
	berlin := func(year int, month time.Month, day, hour, minute int) time.Time {
		return time.Date(year, month, day, hour, minute, 0, 0, calendar.Berlin)
	}
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{name: "during the year", now: berlin(2026, time.October, 15, 12, 0), want: berlin(2025, time.January, 1, 0, 0)},
		{name: "last minute of the year", now: berlin(2026, time.December, 31, 23, 59), want: berlin(2025, time.January, 1, 0, 0)},
		{name: "first hour of the new year", now: berlin(2027, time.January, 1, 0, 30), want: berlin(2025, time.January, 1, 0, 0)},
		{name: "after the first hour", now: berlin(2027, time.January, 1, 1, 30), want: berlin(2026, time.January, 1, 0, 0)},
		{name: "UTC instant in the first Berlin hour", now: time.Date(2026, time.December, 31, 23, 30, 0, 0, time.UTC), want: berlin(2025, time.January, 1, 0, 0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.True(t, tc.want.Equal(operatorDeletionCutoff(tc.now)), "got %s", operatorDeletionCutoff(tc.now))
		})
	}
}
