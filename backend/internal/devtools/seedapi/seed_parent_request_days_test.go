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
