package api

import (
	"testing"

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

func TestSeedWalkingHomeAloneDateDoesNotOverlapOtherRequests(t *testing.T) {
	t.Parallel()
	cases := []struct{ today, want string }{
		{"2026-10-26", "2026-10-30"},
		{"2026-10-27", "2026-11-02"},
		{"2026-10-28", "2026-11-03"}, // Wednesday: both old requests landed on Monday.
		{"2026-10-29", "2026-11-04"},
		{"2026-10-30", "2026-11-05"},
		{"2026-10-31", "2026-11-05"},
		{"2026-11-01", "2026-11-05"},
	}
	for _, tc := range cases {
		today := mustParseSeedDate(t, tc.today)
		got := seedWalkingHomeAloneDate(today)
		assert.Equal(t, tc.want, got.String(), tc.today)
		for reserved := 1; reserved <= 3; reserved++ {
			assert.NotEqual(t, seedSchoolDayAfter(today, reserved).String(), got.String(),
				"%s must leave the first three school days free", tc.today)
		}
	}
}
