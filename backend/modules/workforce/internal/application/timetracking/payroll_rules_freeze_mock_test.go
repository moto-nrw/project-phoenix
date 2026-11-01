package timetracking

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// A closed month keeps the wage types and rules in force at its close
// (#4807): the close freezes them into the snapshot, the export books the
// closed month with them and an open month with the current ones.

type freezingSnapshotRepo struct {
	MonthSnapshots
	recorded []MonthSnapshot
}

func (r *freezingSnapshotRepo) LockStaffBalanceWrites(context.Context, int64) error { return nil }

func (r *freezingSnapshotRepo) LatestClosedMonth(context.Context, int64, int, int) (*MonthSnapshot, error) {
	return nil, nil
}

func (r *freezingSnapshotRepo) RecordClosedMonth(_ context.Context, value MonthSnapshot) (MonthSnapshot, error) {
	r.recorded = append(r.recorded, value)
	return value, nil
}

type fixedRuleSet struct{ set *workforce.PayrollRuleSet }

func (f fixedRuleSet) CurrentPayrollRuleSet(context.Context) (*workforce.PayrollRuleSet, error) {
	return f.set, nil
}

func ruleSetWithNight(rate int) *workforce.PayrollRuleSet {
	return &workforce.PayrollRuleSet{
		WageTypes: []workforce.PayrollWageType{{ID: 7, Name: "Nachtzuschlag", Number: "310", Unit: workforce.WageTypeUnitHours}},
		SurchargeRules: []workforce.PayrollSurchargeRule{{
			ID: 8, Kind: workforce.SurchargeKindNight, WindowStartMinute: 1200, WindowEndMinute: 360,
			RateType: workforce.SurchargeRatePercent, RateHundredths: rate, WageTypeID: 7,
		}},
		AbsenceTypeWageTypes: map[int64]int64{},
	}
}

func TestMonthClose_FreezesTheRulesInForce(t *testing.T) {
	t.Parallel()
	repo := &freezingSnapshotRepo{}
	events := []string{}
	service := NewStaffMonthCloseService(repo, &recordingMonthCloseMonthService{events: &events},
		&recordingMonthCloseStaffLister{events: &events}, &wtmMockSettings{accountStart: "2025-01-01"},
		slog.New(slog.DiscardHandler), WithMonthClosePayrollRules(fixedRuleSet{set: ruleSetWithNight(2500)}))

	_, err := service.CloseMonth(context.Background(), 42, 2025, 8, "Monatsabschluss")

	require.NoError(t, err)
	require.Len(t, repo.recorded, 1)
	var frozen workforce.PayrollRuleSet
	require.NoError(t, json.Unmarshal(repo.recorded[0].PayrollRules, &frozen))
	assert.Equal(t, *ruleSetWithNight(2500), frozen)
}

func TestExportPayroll_AClosedMonthBooksWithItsFrozenRules(t *testing.T) {
	t.Parallel()
	frozen, err := json.Marshal(ruleSetWithNight(4000))
	require.NoError(t, err)
	key := monthKey{Year: 2026, Month: 6}
	night := calendar.NewDate(2026, time.June, 9)
	checkIn := night.BerlinMidnight().Add(21 * time.Hour)
	checkOut := checkIn.Add(2 * time.Hour)
	prefetch := &monthPrefetch{
		sessions:  map[int64][]*WorkSession{5: {{ID: 1, StaffID: 5, CheckInTime: checkIn, CheckOutTime: &checkOut}}},
		snapshots: map[int64][]*MonthSnapshot{5: {{StaffID: 5, Year: 2026, Month: 6, PayrollRules: frozen}}},
	}
	payroll := &exportPayroll{current: ruleSetWithNight(2500), holidays: map[calendar.Date]bool{}, now: checkOut.Add(time.Hour)}

	closed := MonthExportRow{StaffID: 5}
	require.NoError(t, payroll.apply(&closed, &MonthSummary{IsClosed: true}, prefetch, key))
	open := MonthExportRow{StaffID: 5}
	require.NoError(t, payroll.apply(&open, &MonthSummary{}, prefetch, key))

	assert.Equal(t, 4000, closed.PayrollRules.SurchargeRules[0].RateHundredths, "the closed month keeps the rule of its close")
	assert.Equal(t, 2500, open.PayrollRules.SurchargeRules[0].RateHundredths, "an open month books with the current rule")
	assert.Equal(t, map[int64]int{8: 120}, closed.SurchargeMinutes)
}

func TestExportPayroll_ACloseWithoutFrozenRulesBooksWithTheCurrentOnes(t *testing.T) {
	t.Parallel()
	prefetch := &monthPrefetch{snapshots: map[int64][]*MonthSnapshot{5: {{StaffID: 5, Year: 2026, Month: 6}}}}
	payroll := &exportPayroll{current: ruleSetWithNight(2500), holidays: map[calendar.Date]bool{}}

	row := MonthExportRow{StaffID: 5}
	require.NoError(t, payroll.apply(&row, &MonthSummary{IsClosed: true}, prefetch, monthKey{Year: 2026, Month: 6}))

	assert.Equal(t, 2500, row.PayrollRules.SurchargeRules[0].RateHundredths)
}
