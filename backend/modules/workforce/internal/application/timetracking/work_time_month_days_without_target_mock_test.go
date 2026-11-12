package timetracking

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// In a month without a maintained Soll every absence day prices to 0 minutes.
// The card must still count those days, so a reader sees the Krankmeldung and
// why it credits nothing (#6889). The credit and the balance stay unchanged,
// and the credited day counters (DATEV, CSV) stay those of credited days.
func TestWTMMonthSummary_CountsAbsenceDaysWithoutTarget(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()
	f.schedules.entries = nil
	// Fronleichnam 2026-06-04 (Thursday) is a statutory holiday.
	f.svc.holidayReader = &wtmMockHolidayReader{dates: map[Date]bool{NewDate(2026, time.June, 4): true}}
	f.sessions.sessions = []*WorkSession{wtmSession(NewDate(2026, time.June, 15), 8, 120, 0)}
	f.absences.absences = []*StaffAbsence{
		// Mon 1 – Sun 7: the weekend and the holiday are no workdays → 4.
		{Model: Model{ID: 1}, StaffID: wtmStaffID, AbsenceType: AbsenceTypeSick, Status: AbsenceStatusReported,
			DateStart: NewDate(2026, time.June, 1), DateEnd: NewDate(2026, time.June, 7)},
		// Same Tuesday as the sick note: the lower ID consumed it.
		{Model: Model{ID: 2}, StaffID: wtmStaffID, AbsenceType: AbsenceTypeVacation, Status: AbsenceStatusApproved,
			DateStart: NewDate(2026, time.June, 2), DateEnd: NewDate(2026, time.June, 2)},
		// Half day vacation on Wednesday 10.
		{Model: Model{ID: 3}, StaffID: wtmStaffID, AbsenceType: AbsenceTypeVacation, Status: AbsenceStatusApproved,
			HalfDay: true, DateStart: NewDate(2026, time.June, 10), DateEnd: NewDate(2026, time.June, 10)},
		{Model: Model{ID: 4}, StaffID: wtmStaffID, AbsenceType: AbsenceTypeTraining, Status: AbsenceStatusApproved,
			DateStart: NewDate(2026, time.June, 11), DateEnd: NewDate(2026, time.June, 11)},
		{Model: Model{ID: 5}, StaffID: wtmStaffID, AbsenceType: AbsenceTypeOther, Status: AbsenceStatusApproved,
			DateStart: NewDate(2026, time.June, 12), DateEnd: NewDate(2026, time.June, 12)},
		// Freizeitausgleich has no day counter, a request has not counted yet.
		{Model: Model{ID: 6}, StaffID: wtmStaffID, AbsenceType: AbsenceTypeCompTime, Status: AbsenceStatusApproved,
			DateStart: NewDate(2026, time.June, 16), DateEnd: NewDate(2026, time.June, 16)},
		{Model: Model{ID: 7}, StaffID: wtmStaffID, AbsenceType: AbsenceTypeVacation, Status: AbsenceStatusRequested,
			DateStart: NewDate(2026, time.June, 17), DateEnd: NewDate(2026, time.June, 17)},
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)

	assert.False(t, summary.HasTarget)
	assert.InDelta(t, 4, summary.SickDaysWithoutTarget, 0.001)
	assert.InDelta(t, 0.5, summary.VacationDaysWithoutTarget, 0.001)
	assert.InDelta(t, 1, summary.TrainingDaysWithoutTarget, 0.001)
	assert.InDelta(t, 1, summary.OtherDaysWithoutTarget, 0.001)

	assert.Zero(t, summary.SickDays, "credited day counters feed DATEV and stay credited-only")
	assert.Zero(t, summary.VacationDays)
	assert.Zero(t, summary.TrainingDays)
	assert.Zero(t, summary.CreditedSickMinutes+summary.CreditedVacationMinutes+
		summary.CreditedTrainingMinutes+summary.CreditedOtherMinutes, "no new credit")
	assert.Equal(t, 120, summary.BalanceMinutes, "Saldo = Ist + Gutschriften + Buchungen − Soll")
}

// A day after today is planned, not taken: the card counts it once it happened,
// exactly like a credited day.
func TestWTMMonthSummary_DaysWithoutTargetStopAtToday(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()
	f.schedules.entries = nil
	// Fixture today: Wednesday 2026-07-15.
	f.absences.absences = []*StaffAbsence{
		{Model: Model{ID: 1}, StaffID: wtmStaffID, AbsenceType: AbsenceTypeSick, Status: AbsenceStatusReported,
			DateStart: NewDate(2026, time.July, 14), DateEnd: NewDate(2026, time.July, 17)},
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)

	assert.InDelta(t, 2, summary.SickDaysWithoutTarget, 0.001)
}

// With a maintained Soll a zero day is a day off per plan: a part-timer's free
// Tuesday stays uncounted, as before #6889.
func TestWTMMonthSummary_DayOffPerPlanIsNoDayWithoutTarget(t *testing.T) {
	t.Parallel()

	f := newWTMFixture() // schedule: Mondays 480
	f.absences.absences = []*StaffAbsence{
		{Model: Model{ID: 1}, StaffID: wtmStaffID, AbsenceType: AbsenceTypeVacation, Status: AbsenceStatusApproved,
			DateStart: NewDate(2026, time.June, 1), DateEnd: NewDate(2026, time.June, 2)},
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)

	assert.True(t, summary.HasTarget)
	assert.InDelta(t, 1, summary.VacationDays, 0.001, "the Monday credits")
	assert.Zero(t, summary.VacationDaysWithoutTarget, "the free Tuesday is not missing a Soll")
	assert.Equal(t, 480, summary.CreditedVacationMinutes)
}

// Whether a Soll is maintained is decided per day, not per month: one
// Sonderarbeitszeit day prices only itself, so the other absence days of an
// otherwise unmaintained month keep counting (#6853 Ferien-Vorarbeit).
func TestWTMMonthSummary_SonderarbeitszeitDayKeepsOtherDaysWithoutTarget(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()
	f.schedules.entries = nil
	f.svc.overrideReader = &wtmMockOverrideReader{days: workforce.TargetOverrideDays{
		wtmStaffID: {NewDate(2026, time.June, 1).String(): 240},
	}}
	f.absences.absences = []*StaffAbsence{
		{Model: Model{ID: 1}, StaffID: wtmStaffID, AbsenceType: AbsenceTypeSick, Status: AbsenceStatusReported,
			DateStart: NewDate(2026, time.June, 1), DateEnd: NewDate(2026, time.June, 3)},
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)

	assert.True(t, summary.HasTarget)
	assert.Equal(t, 240, summary.CreditedSickMinutes, "the Sonderarbeitszeit day credits its Soll")
	assert.InDelta(t, 1, summary.SickDays, 0.001)
	assert.InDelta(t, 2, summary.SickDaysWithoutTarget, 0.001, "Tuesday and Wednesday have no Soll")
}

// A schedule written from the 15th on leaves the days before it without Soll;
// a sick day there must not vanish because later days of the month have one.
func TestWTMMonthSummary_DaysBeforeScheduleStartCountWithoutTarget(t *testing.T) {
	t.Parallel()

	f := newWTMFixture() // schedule: Mondays 480
	f.schedules.entries[0].ValidFrom = NewDate(2026, time.June, 15)
	f.absences.absences = []*StaffAbsence{
		// Wed 10 has no schedule version yet; Tue 16 is a free day per plan.
		{Model: Model{ID: 1}, StaffID: wtmStaffID, AbsenceType: AbsenceTypeSick, Status: AbsenceStatusReported,
			DateStart: NewDate(2026, time.June, 10), DateEnd: NewDate(2026, time.June, 10)},
		{Model: Model{ID: 2}, StaffID: wtmStaffID, AbsenceType: AbsenceTypeSick, Status: AbsenceStatusReported,
			DateStart: NewDate(2026, time.June, 15), DateEnd: NewDate(2026, time.June, 16)},
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)

	assert.True(t, summary.HasTarget)
	assert.InDelta(t, 1, summary.SickDays, 0.001, "Monday 15 credits")
	assert.InDelta(t, 1, summary.SickDaysWithoutTarget, 0.001, "only Wednesday 10 lacks a Soll")
}
