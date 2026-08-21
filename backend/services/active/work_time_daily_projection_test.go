package active

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The daily table used to derive its Saldo as "Ist minus Soll" and only for
// days that had a WorkSession. A Freizeitausgleich therefore showed no Saldo at
// all, so it looked as if the day cost nothing — while the Monatskarte right
// above it had already deducted the full Tagessoll (#2443). These tests pin the
// per-day projection the table now reads against exactly the examples from the
// Rückmeldung.
//
// Fixture: Mondays are worth 480 minutes, today is 2026-07-15, so 2026-07-06
// and 2026-07-13 are past working days.

var (
	pastMonday      = timezone.NewDate(2026, time.July, 6)
	otherPastMonday = timezone.NewDate(2026, time.July, 13)
	futureMonday    = timezone.NewDate(2026, time.July, 20)
)

func wtmAbsence(id int64, absenceType string, start, end timezone.Date, status string) *activeModels.StaffAbsence {
	return &activeModels.StaffAbsence{
		Model:       base.Model{ID: id},
		StaffID:     wtmStaffID,
		AbsenceType: absenceType,
		DateStart:   start,
		DateEnd:     end,
		Status:      status,
	}
}

// projectDay returns the projection of a single day, priced over a range that
// starts and ends on it — the same call the table makes for its visible window.
func projectDay(t *testing.T, f *wtmFixture, day timezone.Date) DailyProjection {
	t.Helper()
	projection, err := f.svc.GetDailyProjection(context.Background(), wtmStaffID, day, day)
	require.NoError(t, err)
	require.Len(t, projection, 1)
	return projection[0]
}

func TestWTMDailyProjection_AbsenceExamples(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		absence         *activeModels.StaffAbsence
		session         *activeModels.WorkSession
		expectedCredit  int
		expectedActual  int
		expectedBalance int
	}{
		{
			name:            "ganztägiger Freizeitausgleich ohne Arbeitszeit kostet das volle Tagessoll",
			absence:         wtmAbsence(1, activeModels.AbsenceTypeCompTime, pastMonday, pastMonday, activeModels.AbsenceStatusApproved),
			expectedBalance: -480,
		},
		{
			name:            "ganztägiger Urlaub ohne Arbeitszeit gleicht das Tagessoll aus",
			absence:         wtmAbsence(2, activeModels.AbsenceTypeVacation, pastMonday, pastMonday, activeModels.AbsenceStatusApproved),
			expectedCredit:  480,
			expectedBalance: 0,
		},
		{
			name: "halbtägiger Urlaub ohne Arbeitszeit lässt das halbe Tagessoll offen",
			absence: func() *activeModels.StaffAbsence {
				a := wtmAbsence(3, activeModels.AbsenceTypeVacation, pastMonday, pastMonday, activeModels.AbsenceStatusApproved)
				a.HalfDay = true
				return a
			}(),
			expectedCredit:  240,
			expectedBalance: -240,
		},
		{
			name: "halbtägiger Urlaub mit vier Stunden Arbeit gleicht sich aus",
			absence: func() *activeModels.StaffAbsence {
				a := wtmAbsence(4, activeModels.AbsenceTypeVacation, pastMonday, pastMonday, activeModels.AbsenceStatusApproved)
				a.HalfDay = true
				return a
			}(),
			session:         wtmSession(pastMonday, 6, 240, 0),
			expectedCredit:  240,
			expectedActual:  240,
			expectedBalance: 0,
		},
		{
			name:            "ganztägiger Freizeitausgleich mit zwei Stunden Arbeit kostet den Rest",
			absence:         wtmAbsence(5, activeModels.AbsenceTypeCompTime, pastMonday, pastMonday, activeModels.AbsenceStatusApproved),
			session:         wtmSession(pastMonday, 6, 120, 0),
			expectedActual:  120,
			expectedBalance: -360,
		},
		{
			name:            "Krankheit schreibt das Tagessoll gut",
			absence:         wtmAbsence(6, activeModels.AbsenceTypeSick, pastMonday, pastMonday, activeModels.AbsenceStatusReported),
			expectedCredit:  480,
			expectedBalance: 0,
		},
		{
			name:            "Fortbildung schreibt das Tagessoll gut",
			absence:         wtmAbsence(7, activeModels.AbsenceTypeTraining, pastMonday, pastMonday, activeModels.AbsenceStatusApproved),
			expectedCredit:  480,
			expectedBalance: 0,
		},
		{
			name:            "Sonstige schreibt das Tagessoll gut",
			absence:         wtmAbsence(8, activeModels.AbsenceTypeOther, pastMonday, pastMonday, activeModels.AbsenceStatusApproved),
			expectedCredit:  480,
			expectedBalance: 0,
		},
		{
			name:            "ein Tag ohne Abwesenheit und ohne Erfassung bleibt im Minus",
			expectedBalance: -480,
		},
		{
			name:            "eine beantragte Abwesenheit verändert den Tages-Saldo nicht",
			absence:         wtmAbsence(9, activeModels.AbsenceTypeVacation, pastMonday, pastMonday, activeModels.AbsenceStatusRequested),
			expectedBalance: -480,
		},
		{
			name:            "eine abgelehnte Abwesenheit verändert den Tages-Saldo nicht",
			absence:         wtmAbsence(10, activeModels.AbsenceTypeVacation, pastMonday, pastMonday, activeModels.AbsenceStatusDeclined),
			expectedBalance: -480,
		},
		{
			name:            "erfasste Mehrarbeit ohne Abwesenheit ergibt ein Plus",
			session:         wtmSession(pastMonday, 6, 540, 0),
			expectedActual:  540,
			expectedBalance: 60,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := newWTMFixture()
			if tc.absence != nil {
				f.absences.absences = []*activeModels.StaffAbsence{tc.absence}
			}
			if tc.session != nil {
				f.sessions.sessions = []*activeModels.WorkSession{tc.session}
			}

			day := projectDay(t, f, pastMonday)

			assert.Equal(t, 480, day.TargetMinutes, "Montag ist ein voller Arbeitstag")
			assert.Equal(t, tc.expectedCredit, day.CreditMinutes, "Gutschrift")
			assert.Equal(t, tc.expectedActual, day.ActualMinutes, "Ist")
			assert.Equal(t, tc.expectedBalance, day.BalanceMinutes, "Saldo")
		})
	}
}

// A planned day off has no Soll, so it also has no Saldo to show — only real
// work on it moves the account.
func TestWTMDailyProjection_DayWithoutTargetIsNeutral(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()
	tuesday := pastMonday.AddDays(1)

	day := projectDay(t, f, tuesday)

	assert.Zero(t, day.TargetMinutes)
	assert.Zero(t, day.CreditMinutes)
	assert.Zero(t, day.BalanceMinutes)
}

// Tomorrow's Soll is planned, not owed: the card charges Soll up to today only,
// so a future day must not report a minus the card does not carry.
func TestWTMDailyProjection_FutureDayCarriesTargetButNoBalance(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()
	f.absences.absences = []*activeModels.StaffAbsence{
		wtmAbsence(11, activeModels.AbsenceTypeVacation, futureMonday, futureMonday, activeModels.AbsenceStatusApproved),
	}

	day := projectDay(t, f, futureMonday)

	assert.Equal(t, 480, day.TargetMinutes, "der geplante Tag behält sein Soll")
	assert.Zero(t, day.CreditMinutes, "vor dem Tag gibt es nichts gutzuschreiben")
	assert.Zero(t, day.BalanceMinutes)
}

// Days before a mid-month account start are invisible to the card, so the table
// must not price them either — not even the work recorded on them.
func TestWTMDailyProjection_ZeroesDaysBeforeMidMonthAccountStart(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()
	f.settings.accountStart = "2026-07-08"
	f.sessions.sessions = []*activeModels.WorkSession{wtmSession(pastMonday, 6, 240, 0)}
	f.absences.absences = []*activeModels.StaffAbsence{
		wtmAbsence(12, activeModels.AbsenceTypeVacation, pastMonday, pastMonday, activeModels.AbsenceStatusApproved),
	}

	day := projectDay(t, f, pastMonday)

	assert.Zero(t, day.TargetMinutes)
	assert.Zero(t, day.CreditMinutes)
	assert.Zero(t, day.ActualMinutes)
	assert.Zero(t, day.BalanceMinutes)
}

// A public holiday zeroes the Soll for everyone (§2 EntgFG), so there is
// nothing to credit and nothing to miss.
func TestWTMDailyProjection_HolidayHasNoBalance(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()
	f.svc.SetHolidayReader(&wtmMockHolidayReader{dates: map[timezone.Date]bool{pastMonday: true}})
	f.absences.absences = []*activeModels.StaffAbsence{
		wtmAbsence(13, activeModels.AbsenceTypeCompTime, pastMonday, pastMonday, activeModels.AbsenceStatusApproved),
	}

	day := projectDay(t, f, pastMonday)

	assert.Zero(t, day.TargetMinutes)
	assert.Zero(t, day.CreditMinutes)
	assert.Zero(t, day.BalanceMinutes)
}

// Overlapping absences pay a day only once — the lowest ID owns it. Without
// that rule the table would credit a day twice and drift from the card.
func TestWTMDailyProjection_OverlappingAbsencesCreditOnce(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()
	f.absences.absences = []*activeModels.StaffAbsence{
		wtmAbsence(21, activeModels.AbsenceTypeCompTime, pastMonday, pastMonday, activeModels.AbsenceStatusApproved),
		wtmAbsence(22, activeModels.AbsenceTypeVacation, pastMonday, pastMonday, activeModels.AbsenceStatusApproved),
	}

	day := projectDay(t, f, pastMonday)

	assert.Zero(t, day.CreditMinutes, "der Freizeitausgleich mit der kleineren ID verbraucht den Tag")
	assert.Equal(t, -480, day.BalanceMinutes)
}

// The guarantee the whole issue rests on: the visible day rows must add up to
// the Monatskarte below them.
func TestWTMDailyProjection_SumMatchesMonthCardBalance(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()
	ctx := context.Background()
	f.sessions.sessions = []*activeModels.WorkSession{wtmSession(pastMonday, 6, 300, 0)}
	f.absences.absences = []*activeModels.StaffAbsence{
		wtmAbsence(31, activeModels.AbsenceTypeCompTime, otherPastMonday, otherPastMonday, activeModels.AbsenceStatusApproved),
		wtmAbsence(32, activeModels.AbsenceTypeVacation, futureMonday, futureMonday, activeModels.AbsenceStatusApproved),
	}

	projection, err := f.svc.GetDailyProjection(ctx, wtmStaffID,
		timezone.NewDate(2026, time.July, 1), timezone.NewDate(2026, time.July, 31))
	require.NoError(t, err)

	sumBalance, sumActual, sumCredit := 0, 0, 0
	for _, day := range projection {
		sumBalance += day.BalanceMinutes
		sumActual += day.ActualMinutes
		sumCredit += day.CreditMinutes
	}

	summary, err := f.svc.GetMonthSummary(ctx, wtmStaffID, 2026, 7)
	require.NoError(t, err)

	assert.Equal(t, summary.BalanceMinutes, sumBalance, "Summe der Tageszeilen = Monatssaldo")
	assert.Equal(t, summary.ActualMinutes, sumActual, "Summe Ist")
	assert.Equal(t, summary.CreditedSickMinutes+summary.CreditedVacationMinutes+
		summary.CreditedTrainingMinutes+summary.CreditedOtherMinutes, sumCredit, "Summe Gutschriften")
	// 6th: 300 worked against 480 Soll, 13th: Freizeitausgleich, full 480 owed.
	assert.Equal(t, -180-480, sumBalance)
}
