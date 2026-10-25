package records

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The cutoffs match the UTC windows of the retention functions of migration
// 1.15.566 at the database clocks its regression test freezes, whatever
// location the application clock carries.
func TestRetentionCutoffsMatchTheDatabaseWindowsInUTC(t *testing.T) {
	t.Parallel()
	instant := func(value string) time.Time {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ now, threeYearsEarlier string }{
		{"2026-03-29T12:00:00+02:00", "2023-03-29T10:00:00Z"},
		{"2026-10-25T12:00:00+01:00", "2023-10-25T11:00:00Z"},
		{"2027-03-29T12:00:00+02:00", "2024-03-29T10:00:00Z"},
		{"2028-02-29T12:00:00+01:00", "2025-02-28T11:00:00Z"},
		{"2028-03-01T00:30:00+01:00", "2025-02-28T23:30:00Z"},
	} {
		for _, location := range []*time.Location{time.UTC, berlin} {
			now := instant(tc.now).In(location)
			assert.True(t, instant(tc.threeYearsEarlier).Add(-RetentionClockMargin).Equal(RetentionYearsCutoff(now, 3)),
				"three years before %s in %s", tc.now, location)
			assert.Equal(t, 90*24*time.Hour+RetentionClockMargin, now.Sub(RetentionDaysCutoff(now, 90)),
				"90 days before %s in %s are elapsed hours", tc.now, location)
		}
	}
}
