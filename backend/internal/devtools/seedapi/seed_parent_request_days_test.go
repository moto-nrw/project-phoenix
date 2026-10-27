package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Felix's open pickup change lies on the third school day: the next two are
// the days the product screenshots pick for an absence and a pickup change,
// whichever weekday the seed runs on.
func TestSeedSchoolDayAfterSkipsTheShotDays(t *testing.T) {
	t.Parallel()

	cases := []struct{ today, want string }{
		{"2026-10-20", "2026-10-23"}, // Tuesday, like the CI run: Wed and Thu stay free
		{"2026-10-22", "2026-10-27"}, // Thursday: Fri and Mon stay free
		{"2026-10-23", "2026-10-28"}, // Friday
		{"2026-10-25", "2026-10-28"}, // Sunday
	}
	for _, tc := range cases {
		today := mustParseSeedDate(t, tc.today)
		got := seedSchoolDayAfter(today, 3)
		assert.Equal(t, tc.want, got.String(), tc.today)
		assert.NotEqual(t, seedSchoolDayAfter(today, 1).String(), got.String())
		assert.NotEqual(t, seedSchoolDayAfter(today, 2).String(), got.String())
	}
}

func TestSeedParentPickupPermissionDaysAvoidPendingPickupChange(t *testing.T) {
	t.Parallel()

	cases := []struct{ today, alone string }{
		{"2026-10-19", "2026-10-23"}, // Monday
		{"2026-10-20", "2026-10-26"}, // Tuesday
		{"2026-10-21", "2026-10-27"}, // Wednesday: avoid Monday's pending request
		{"2026-10-22", "2026-10-26"}, // Thursday
		{"2026-10-23", "2026-10-27"}, // Friday
		{"2026-10-24", "2026-10-29"}, // Saturday: avoid Wednesday's pending request
		{"2026-10-25", "2026-10-29"}, // Sunday
	}
	for _, tc := range cases {
		today := mustParseSeedDate(t, tc.today)
		collector, alone := seedParentPickupPermissionDays(today)
		assert.NotContains(t, []time.Weekday{time.Saturday, time.Sunday}, collector.Weekday(), tc.today)
		assert.NotContains(t, []time.Weekday{time.Saturday, time.Sunday}, alone.Weekday(), tc.today)
		assert.NotEqual(t, seedSchoolDayAfter(today, 3).String(), alone.String(), tc.today)
		assert.Equal(t, tc.alone, alone.String(), tc.today)
	}
}
