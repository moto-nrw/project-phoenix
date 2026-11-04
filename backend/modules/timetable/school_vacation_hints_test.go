package timetable_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// fixedDays is a NonWorkingDayCalendar over fixed date sets.
type fixedDays struct{ holidays, closing map[string]bool }

func (d fixedDays) TenantHolidayDates(context.Context, string, string) (map[string]bool, error) {
	return d.holidays, nil
}

func (d fixedDays) ClosingDayDates(context.Context, string, string) (map[string]bool, error) {
	return d.closing, nil
}

func loadDays(t *testing.T, holidays, closing []string) timetable.NonWorkingDays {
	t.Helper()
	set := func(dates []string) map[string]bool {
		result := map[string]bool{}
		for _, date := range dates {
			result[date] = true
		}
		return result
	}
	days, err := timetable.LoadNonWorkingDays(context.Background(), fixedDays{holidays: set(holidays), closing: set(closing)}, "2026-01-01", "2027-12-31")
	require.NoError(t, err)
	return days
}

func plain(dates ...string) []timetable.SchoolVacationCandidate {
	candidates := make([]timetable.SchoolVacationCandidate, 0, len(dates))
	for _, date := range dates {
		candidates = append(candidates, timetable.SchoolVacationCandidate{Date: date})
	}
	return candidates
}

var christmas = timetable.SchoolVacationWindow{Name: "Weihnachtsferien", StartDate: "2026-12-23", EndDate: "2027-01-06"}

// The NRW Christmas vacation crosses the year: Termine on both sides of New
// Year count, the statutory holidays inside it do not.
func TestSelectSchoolVacationHints_CountsAcrossTheYearChange(t *testing.T) {
	t.Parallel()
	days := loadDays(t, []string{"2026-12-25", "2026-12-26", "2027-01-01"}, nil)
	candidates := plain("2026-12-22", "2026-12-23", "2026-12-25", "2026-12-28", "2027-01-01", "2027-01-04", "2027-01-06", "2027-01-07")

	hints := timetable.SelectSchoolVacationHints([]timetable.SchoolVacationWindow{christmas}, candidates, days, "2026-11-30", "2027-01-25")

	require.Len(t, hints, 1)
	assert.Equal(t, timetable.SchoolVacationHint{
		Name: "Weihnachtsferien", StartDate: "2026-12-23", EndDate: "2027-01-06", CancelFrom: "2026-12-23", Count: 4,
	}, hints[0], "23.12., 28.12., 04.01. and 06.01.; not the holidays, not the days around the vacation")
}

func TestSelectSchoolVacationHints_LeavesOutDeliberateCareAndClosingDays(t *testing.T) {
	t.Parallel()
	days := loadDays(t, nil, []string{"2026-12-28", "2026-12-29"})
	candidates := []timetable.SchoolVacationCandidate{
		{Date: "2026-12-23"},
		{Date: "2026-12-28"}, // closing day
		{Date: "2026-12-29", SeriesIncludesClosingDays: true}, // holiday care on a closing day
		{Date: "2026-12-30", SeriesIncludesClosingDays: true}, // holiday care series
		{Date: "2026-12-30", InHolidayPeriod: true},           // series in a holiday period
		{Date: "2026-12-31"},
	}

	hints := timetable.SelectSchoolVacationHints([]timetable.SchoolVacationWindow{christmas}, candidates, days, "2026-12-01", "2027-01-26")

	require.Len(t, hints, 1)
	assert.Equal(t, 2, hints[0].Count, "only the ordinary Termine of 23.12. and 31.12.")
}

func TestSelectSchoolVacationHints_EightWeekBoundary(t *testing.T) {
	t.Parallel()
	days := loadDays(t, nil, nil)
	candidates := plain("2026-12-23")
	windows := []timetable.SchoolVacationWindow{christmas}

	// 2026-10-28 + 56 days is 2026-12-23: the vacation starts on the last day.
	assert.Len(t, timetable.SelectSchoolVacationHints(windows, candidates, days, "2026-10-28", "2026-12-23"), 1)
	// One day earlier the vacation starts a day after the eight weeks.
	assert.Empty(t, timetable.SelectSchoolVacationHints(windows, candidates, days, "2026-10-27", "2026-12-22"))
}

func TestSelectSchoolVacationHints_RunningVacationCountsFromToday(t *testing.T) {
	t.Parallel()
	days := loadDays(t, nil, nil)
	candidates := plain("2026-12-23", "2026-12-28", "2027-01-04")

	hints := timetable.SelectSchoolVacationHints([]timetable.SchoolVacationWindow{christmas}, candidates, days, "2026-12-28", "2027-02-22")

	require.Len(t, hints, 1)
	assert.Equal(t, "2026-12-28", hints[0].CancelFrom)
	assert.Equal(t, 2, hints[0].Count, "a Termin before today is history, not a hint")
	assert.Empty(t, timetable.SelectSchoolVacationHints([]timetable.SchoolVacationWindow{christmas}, candidates, days, "2027-01-07", "2027-03-04"),
		"an ended vacation gives no hint")
}

func TestSelectSchoolVacationHints_AcknowledgedOrEmptyWindowGivesNoHint(t *testing.T) {
	t.Parallel()
	days := loadDays(t, nil, nil)
	autumn := timetable.SchoolVacationWindow{Name: "Herbstferien", StartDate: "2026-10-17", EndDate: "2026-10-31"}
	confirmed := christmas
	confirmed.Acknowledged = true
	candidates := plain("2026-10-19", "2026-12-23")

	hints := timetable.SelectSchoolVacationHints([]timetable.SchoolVacationWindow{confirmed, autumn}, candidates, days, "2026-10-01", "2027-01-30")
	require.Len(t, hints, 1, "only the window the school has not confirmed")
	assert.Equal(t, "Herbstferien", hints[0].Name)

	assert.Empty(t, timetable.SelectSchoolVacationHints([]timetable.SchoolVacationWindow{christmas}, plain("2027-02-01"), days, "2026-12-01", "2027-01-26"),
		"a vacation without a Termin gives no hint")
}
