package active

// Unit tests for #1420: Stundenkonto transactions (payout / comp-time grant /
// reset rows) entering the Monatskarte carry chain, and the comp_time absence
// type deliberately crediting nothing. Reuses the wtmFixture mocks
// (work_time_month_service_mock_test.go): Monday-only 480-minute schedule,
// account start 2026-06-01, today = 2026-07-15.

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wtmMockAdjustmentReader struct {
	adjustments []*activeModels.StaffBalanceAdjustment
}

func (m *wtmMockAdjustmentReader) GetByStaffAndDateRange(_ context.Context, _ int64, from, to timezone.Date) ([]*activeModels.StaffBalanceAdjustment, error) {
	var result []*activeModels.StaffBalanceAdjustment
	for _, a := range m.adjustments {
		if !a.EffectiveDate.Before(from) && !a.EffectiveDate.After(to) {
			result = append(result, a)
		}
	}
	return result, nil
}

func wtmAdjustment(id int64, adjustmentType string, minutesDelta int, effective timezone.Date) *activeModels.StaffBalanceAdjustment {
	adj := &activeModels.StaffBalanceAdjustment{
		StaffID:       wtmStaffID,
		Type:          adjustmentType,
		MinutesDelta:  minutesDelta,
		EffectiveDate: effective,
		Note:          "test",
		DecidedBy:     wtmStaffID,
		DecidedAt:     time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC),
	}
	adj.ID = id
	return adj
}

// A payout in the requested month reduces that month's own balance and shows
// up as adjustment_minutes + one Adjustments entry on the card.
func TestWTMAdjustments_CurrentMonthPayoutReducesBalance(t *testing.T) {
	f := newWTMFixture()
	baseline, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)

	f.svc.SetAdjustmentReader(&wtmMockAdjustmentReader{adjustments: []*activeModels.StaffBalanceAdjustment{
		wtmAdjustment(1, activeModels.BalanceAdjustmentTypePayout, -120, timezone.NewDate(2026, time.July, 7)),
	}})

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, -120, summary.AdjustmentMinutes)
	assert.Equal(t, baseline.BalanceMinutes-120, summary.BalanceMinutes)
	assert.Equal(t, baseline.ClosingBalanceMinutes-120, summary.ClosingBalanceMinutes)
	require.Len(t, summary.Adjustments, 1)
	assert.Equal(t, activeModels.BalanceAdjustmentTypePayout, summary.Adjustments[0].Type)
	assert.Equal(t, -120, summary.Adjustments[0].MinutesDelta)
}

// An adjustment in a PRIOR month flows into the requested month's Übertrag via
// the carry chain, without appearing in the requested month's own list.
func TestWTMAdjustments_PriorMonthAdjustmentFlowsIntoCarry(t *testing.T) {
	f := newWTMFixture()
	baseline, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)

	f.svc.SetAdjustmentReader(&wtmMockAdjustmentReader{adjustments: []*activeModels.StaffBalanceAdjustment{
		wtmAdjustment(1, activeModels.BalanceAdjustmentTypeCompTime, -60, timezone.NewDate(2026, time.June, 15)),
	}})

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, baseline.CarryInMinutes-60, summary.CarryInMinutes)
	assert.Equal(t, 0, summary.AdjustmentMinutes, "prior-month adjustment must not enter July's own sum")
	assert.Empty(t, summary.Adjustments)
	assert.Equal(t, baseline.ClosingBalanceMinutes-60, summary.ClosingBalanceMinutes)
}

// A future-dated adjustment is neutral until its effective date arrives, like
// future targets, sessions, and credits.
func TestWTMAdjustments_FutureDatedAdjustmentIgnored(t *testing.T) {
	f := newWTMFixture()
	baseline, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)

	// today = 2026-07-15; the adjustment sits five days ahead.
	f.svc.SetAdjustmentReader(&wtmMockAdjustmentReader{adjustments: []*activeModels.StaffBalanceAdjustment{
		wtmAdjustment(1, activeModels.BalanceAdjustmentTypePayout, -300, timezone.NewDate(2026, time.July, 20)),
	}})

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.AdjustmentMinutes)
	assert.Empty(t, summary.Adjustments)
	assert.Equal(t, baseline.ClosingBalanceMinutes, summary.ClosingBalanceMinutes)
}

// A historical cutoff must ignore later sessions, targets, and adjustments in
// the same month. ResetBalance uses this value to post its delta on the cutoff
// day, so including July 13/14 activity in a July 7 reset would corrupt the
// requested carryover.
func TestWTMAdjustments_ClosingBalanceAsOfHistoricalCutoff(t *testing.T) {
	f := newWTMFixture()
	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(timezone.NewDate(2026, time.July, 6), 9, 300, 0),
		wtmSession(timezone.NewDate(2026, time.July, 13), 9, 480, 0),
	}
	f.svc.SetAdjustmentReader(&wtmMockAdjustmentReader{adjustments: []*activeModels.StaffBalanceAdjustment{
		wtmAdjustment(1, activeModels.BalanceAdjustmentTypePayout, -60, timezone.NewDate(2026, time.July, 7)),
		wtmAdjustment(2, activeModels.BalanceAdjustmentTypePayout, -120, timezone.NewDate(2026, time.July, 14)),
	}})

	cutoff := timezone.NewDate(2026, time.July, 7)
	balance, err := f.svc.GetClosingBalanceAsOf(context.Background(), wtmStaffID, cutoff)
	require.NoError(t, err)

	// June contributes five unworked Mondays (-2400). Through July 7, only the
	// July 6 target/session and July 7 payout apply: -2400 - 480 + 300 - 60.
	assert.Equal(t, -2640, balance)

	todaySummary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, -2760, todaySummary.ClosingBalanceMinutes)
	assert.NotEqual(t, todaySummary.ClosingBalanceMinutes, balance)
}

func TestWTMAdjustments_BalanceAdjustmentMinutesUsesRequestedRange(t *testing.T) {
	f := newWTMFixture()
	date := timezone.NewDate(2026, time.July, 7)
	f.svc.SetAdjustmentReader(&wtmMockAdjustmentReader{adjustments: []*activeModels.StaffBalanceAdjustment{
		wtmAdjustment(1, activeModels.BalanceAdjustmentTypePayout, -60, date.AddDays(-1)),
		wtmAdjustment(2, activeModels.BalanceAdjustmentTypePayout, -120, date),
		wtmAdjustment(3, activeModels.BalanceAdjustmentTypeCompTime, -30, date),
		wtmAdjustment(4, activeModels.BalanceAdjustmentTypePayout, -240, date.AddDays(1)),
	}})

	minutes, err := f.svc.GetBalanceAdjustmentMinutes(context.Background(), wtmStaffID, date, date)

	require.NoError(t, err)
	assert.Equal(t, -150, minutes)
}

// A reset row turns the closing balance into the carry-over value: with a
// closing balance of B before the reset, delta = carryover − B yields exactly
// carryover afterwards (#1420 5c).
func TestWTMAdjustments_ResetRowZeroesClosingBalance(t *testing.T) {
	f := newWTMFixture()
	// One worked Monday (2026-07-06, 300 net minutes) against two Monday
	// targets to date (July 6 + 13 = 960): balance = 300 − 960 = −660.
	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(timezone.NewDate(2026, time.July, 6), 9, 300, 0),
	}
	baseline, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	require.NotZero(t, baseline.ClosingBalanceMinutes)

	f.svc.SetAdjustmentReader(&wtmMockAdjustmentReader{adjustments: []*activeModels.StaffBalanceAdjustment{
		wtmAdjustment(1, activeModels.BalanceAdjustmentTypeReset, -baseline.ClosingBalanceMinutes, timezone.NewDate(2026, time.July, 14)),
	}})

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.ClosingBalanceMinutes)
}

// A reset is an audited ledger transaction, not a mutable checkpoint. A
// correction entered later for work before the reset must remain visible in
// the current balance; rewriting the stored reset delta would erase it.
func TestWTMAdjustments_LateHistoricalCorrectionRemainsVisibleAfterReset(t *testing.T) {
	f := newWTMFixture()
	cutoff := timezone.NewDate(2026, time.July, 7)
	balanceAtDecision, err := f.svc.GetClosingBalanceAsOf(context.Background(), wtmStaffID, cutoff)
	require.NoError(t, err)
	f.svc.SetAdjustmentReader(&wtmMockAdjustmentReader{adjustments: []*activeModels.StaffBalanceAdjustment{
		wtmAdjustment(1, activeModels.BalanceAdjustmentTypeReset, -balanceAtDecision, cutoff),
	}})

	beforeCorrection, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)

	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(timezone.NewDate(2026, time.July, 6), 9, 300, 0),
	}
	afterCorrection, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)

	assert.Equal(t, beforeCorrection.ClosingBalanceMinutes+300, afterCorrection.ClosingBalanceMinutes)
}

// comp_time absences (#1420 5b) credit NOTHING: the day keeps its Soll, so
// the balance drops by the day's target — unlike a vacation day, which
// credits it. The day still counts as consumed, so an overlapping vacation
// cannot re-credit it.
func TestWTMAbsences_CompTimeNotCredited(t *testing.T) {
	monday := timezone.NewDate(2026, time.July, 6)

	// Vacation on the Monday: the 480-minute Soll is credited back.
	f := newWTMFixture()
	f.absences.absences = []*activeModels.StaffAbsence{{
		StaffID: wtmStaffID, AbsenceType: activeModels.AbsenceTypeVacation,
		DateStart: monday, DateEnd: monday, Status: activeModels.AbsenceStatusApproved,
	}}
	vacationSummary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, 480, vacationSummary.CreditedVacationMinutes)

	// comp_time on the same Monday: no credit anywhere, balance 480 lower.
	f = newWTMFixture()
	f.absences.absences = []*activeModels.StaffAbsence{{
		StaffID: wtmStaffID, AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart: monday, DateEnd: monday, Status: activeModels.AbsenceStatusApproved,
	}}
	compTimeSummary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Zero(t, compTimeSummary.CreditedVacationMinutes)
	assert.Zero(t, compTimeSummary.CreditedOtherMinutes, "comp_time must not fall into the default other-credit branch")
	assert.Equal(t, vacationSummary.BalanceMinutes-480, compTimeSummary.BalanceMinutes)

	// Overlapping approved vacation created later (higher ID) must not
	// re-credit the consumed day.
	f.absences.absences = append(f.absences.absences, &activeModels.StaffAbsence{
		StaffID: wtmStaffID, AbsenceType: activeModels.AbsenceTypeVacation,
		DateStart: monday, DateEnd: monday, Status: activeModels.AbsenceStatusApproved,
	})
	f.absences.absences[1].ID = 99
	overlapSummary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Zero(t, overlapSummary.CreditedVacationMinutes, "comp_time consumed the day; vacation must not re-credit it")
	assert.Equal(t, compTimeSummary.BalanceMinutes, overlapSummary.BalanceMinutes)
}

// A half-day comp_time absence still credits nothing. The intended half-day
// effect comes from recording the worked half: four worked hours against an
// eight-hour target leave a four-hour deduction. Crediting another four hours
// here would incorrectly make the Freizeitausgleich free.
func TestWTMAbsences_HalfDayCompTimeRequiresWorkedHalf(t *testing.T) {
	monday := timezone.NewDate(2026, time.July, 6)

	f := newWTMFixture()
	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(monday, 9, 240, 0),
	}
	f.absences.absences = []*activeModels.StaffAbsence{{
		StaffID: wtmStaffID, AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart: monday, DateEnd: monday, HalfDay: true,
		Status: activeModels.AbsenceStatusApproved,
	}}

	compTimeSummary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Zero(t, compTimeSummary.CreditedVacationMinutes)
	assert.Zero(t, compTimeSummary.CreditedOtherMinutes)

	f.absences.absences[0].AbsenceType = activeModels.AbsenceTypeVacation
	vacationSummary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, 240, vacationSummary.CreditedVacationMinutes)
	assert.Equal(t, vacationSummary.BalanceMinutes-240, compTimeSummary.BalanceMinutes)
}
