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
