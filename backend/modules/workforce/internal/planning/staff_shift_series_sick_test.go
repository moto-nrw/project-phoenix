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

		sick := workforce.AbsenceTypeSick
		assert.Equal(t, map[calendar.Date]absenceStamp{
			monday:            {absenceID: 9, absenceType: sick},
			monday.AddDays(1): {absenceID: 4, absenceType: sick},
			monday.AddDays(2): {absenceID: 4, absenceType: sick},
		}, days, "%s: only window days, the lower ID on the shared day", name)
	}
}

// Every absence in force takes plan effect since #6847, a sick report as well
// as an approved vacation. A pending request and a single half day do not,
// and the half boundary days of a range stay.
func TestSickDayIndexSkipsHalfDaysAndAbsencesNotInForce(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.August, 31)
	vacation := func(status string) func(*workforce.StaffAbsence) {
		return func(a *workforce.StaffAbsence) { a.AbsenceType, a.Status = workforce.AbsenceTypeVacation, status }
	}
	days, err := sickDayIndex([]workforce.StaffAbsence{
		sickAbsence(1, monday, monday, func(a *workforce.StaffAbsence) { a.HalfDay = true }),
		sickAbsence(2, monday.AddDays(1), monday.AddDays(1), vacation(workforce.AbsenceStatusApproved)),
		sickAbsence(3, monday.AddDays(2), monday.AddDays(4), func(a *workforce.StaffAbsence) { a.StartHalfDay, a.EndHalfDay = true, true }),
		sickAbsence(4, monday.AddDays(5), monday.AddDays(5), vacation(workforce.AbsenceStatusRequested)),
		sickAbsence(5, monday.AddDays(6), monday.AddDays(6), func(a *workforce.StaffAbsence) {
			a.AbsenceType, a.HalfDay = workforce.AbsenceTypeTraining, true
		}),
	}, monday, monday.AddDays(6))
	require.NoError(t, err)

	assert.Equal(t, map[calendar.Date]absenceStamp{
		monday.AddDays(1): {absenceID: 2, absenceType: workforce.AbsenceTypeVacation},
		monday.AddDays(3): {absenceID: 3, absenceType: workforce.AbsenceTypeSick},
	}, days)
}

// A shift a vacation cancels names no art: the plan is team-visible (#6847).
func TestApplySickReportLabelsTheShiftByAbsenceType(t *testing.T) {
	t.Parallel()

	for absenceType, want := range map[string]string{
		workforce.AbsenceTypeSick:     workforce.SickShiftChangeReason,
		workforce.AbsenceTypeVacation: workforce.AbsenceShiftChangeReason,
		workforce.AbsenceTypeOther:    workforce.AbsenceShiftChangeReason,
	} {
		shift := &workforce.ShiftRecord{}
		applySickReport(shift, absenceStamp{absenceID: 3, absenceType: absenceType})
		assert.True(t, shift.Cancelled)
		require.NotNil(t, shift.ChangeReason)
		assert.Equal(t, want, *shift.ChangeReason, absenceType)
		releaseSickState(shift)
		assert.Nil(t, shift.ChangeReason, "reactivation drops the %s label", absenceType)
	}
}

func TestSickDayIndexRejectsAnInvalidReportDate(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.August, 31)
	_, err := sickDayIndex([]workforce.StaffAbsence{{ID: 5, AbsenceType: workforce.AbsenceTypeSick, Status: workforce.AbsenceStatusReported, DateStart: "31.08.2026", DateEnd: monday.String()}}, monday, monday)
	require.Error(t, err)
}
