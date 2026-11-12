package planning

// The sick-day index decides which occurrences a series materializes
// cancelled (#6302). The day rule itself is timetable.SickReport's; the
// integration suite in workflows/shiftplansync/compose runs it through the
// real absence writes and the cascade.

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sickAbsence(id int64, start, end calendar.Date, mutate func(*workforce.StaffAbsence)) workforce.StaffAbsence {
	absence := workforce.StaffAbsence{
		ID: id, StaffID: 7, AbsenceType: workforce.AbsenceTypeSick,
		DateStart: start.String(), DateEnd: end.String(), Status: workforce.AbsenceStatusReported,
	}
	if mutate != nil {
		mutate(&absence)
	}
	return absence
}

func TestSickDayIndexPicksTheLowerReportIDWithinTheWindow(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.August, 31)
	higher := sickAbsence(9, monday.AddDays(-3), monday.AddDays(1), nil)
	lower := sickAbsence(4, monday.AddDays(1), monday.AddDays(9), nil)
	for name, absences := range map[string][]workforce.StaffAbsence{
		"higher first": {higher, lower},
		"lower first":  {lower, higher},
	} {
		days, err := sickDayIndex(absences, monday, monday.AddDays(2))
		require.NoError(t, err)

		assert.Equal(t, map[calendar.Date]int64{
			monday:            9,
			monday.AddDays(1): 4,
			monday.AddDays(2): 4,
		}, days, "%s: only window days, the lower ID on the shared day", name)
	}
}

func TestSickDayIndexSkipsHalfDaysAndOtherTypes(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.August, 31)
	days, err := sickDayIndex([]workforce.StaffAbsence{
		sickAbsence(1, monday, monday, func(a *workforce.StaffAbsence) { a.HalfDay = true }),
		sickAbsence(2, monday.AddDays(1), monday.AddDays(1), func(a *workforce.StaffAbsence) { a.AbsenceType = workforce.AbsenceTypeVacation }),
		sickAbsence(3, monday.AddDays(2), monday.AddDays(4), func(a *workforce.StaffAbsence) { a.StartHalfDay, a.EndHalfDay = true, true }),
	}, monday, monday.AddDays(6))
	require.NoError(t, err)

	assert.Equal(t, map[calendar.Date]int64{monday.AddDays(3): 3}, days)
}

func TestSickDayIndexRejectsAnInvalidReportDate(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.August, 31)
	_, err := sickDayIndex([]workforce.StaffAbsence{{ID: 5, AbsenceType: workforce.AbsenceTypeSick, DateStart: "31.08.2026", DateEnd: monday.String()}}, monday, monday)
	require.Error(t, err)
}
