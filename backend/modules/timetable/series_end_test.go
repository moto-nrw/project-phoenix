package timetable_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestValidateSeriesLastDay(t *testing.T) {
	t.Parallel()

	start := calendar.Date("2026-10-19")
	periodEnd := calendar.Date("2027-07-31")

	require.NoError(t, timetable.ValidateSeriesLastDay("2026-10-23", start, &periodEnd), "holiday-care week")
	require.NoError(t, timetable.ValidateSeriesLastDay("2026-10-19", start, &periodEnd), "a single day")
	require.NoError(t, timetable.ValidateSeriesLastDay("2027-07-31", start, &periodEnd), "the last day of the period")
	require.NoError(t, timetable.ValidateSeriesLastDay("2026-10-23", "", nil), "no start and no period pin")

	for name, lastDay := range map[string]calendar.Date{
		"before the start": "2026-10-16",
		"after the period": "2027-08-01",
		"missing":          "",
	} {
		assert.ErrorIs(t, timetable.ValidateSeriesLastDay(lastDay, start, &periodEnd), timetable.ErrInvalidSeriesEnd, name)
	}
}
