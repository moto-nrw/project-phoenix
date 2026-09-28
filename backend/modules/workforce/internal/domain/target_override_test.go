package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandTargetOverridesSkipsWeekendsAndStatutoryHolidays(t *testing.T) {
	t.Parallel()

	overrides := []StaffTargetOverride{
		{StaffID: 7, StartDate: "2026-12-21", EndDate: "2027-01-03", DailyMinutes: 300},
		{StaffID: 8, StartDate: "2026-12-28", EndDate: "2026-12-28", DailyMinutes: 0},
	}
	holidays := map[string]bool{"2026-12-25": true, "2027-01-01": true}

	days := ExpandTargetOverrides(overrides, "2026-12-01", "2026-12-31", holidays)

	assert.Equal(t, 300, days[7]["2026-12-21"])
	assert.NotContains(t, days[7], "2026-12-25", "statutory holiday keeps Soll 0")
	assert.NotContains(t, days[7], "2026-12-26", "Saturday is not an override day")
	assert.NotContains(t, days[7], "2027-01-02", "days outside the window are clamped away")
	assert.Len(t, days[7], 8, "21.–31.12. has 9 weekdays, minus the 25th")
	require.Contains(t, days[8], "2026-12-28")
	assert.Zero(t, days[8]["2026-12-28"], "a zero-minute override is still an override day")
}

func TestValidateStaffTargetOverrideFields(t *testing.T) {
	t.Parallel()

	valid := StaffTargetOverrideFields{StartDate: "2026-10-19", EndDate: "2026-10-23", DailyMinutes: 510}
	require.NoError(t, ValidateStaffTargetOverrideFields(valid))
	require.NoError(t, ValidateStaffTargetOverrideFields(StaffTargetOverrideFields{StartDate: "2026-10-19", EndDate: "2026-10-19", DailyMinutes: 0}))

	cases := map[string]StaffTargetOverrideFields{
		"inverted range": {StartDate: "2026-10-23", EndDate: "2026-10-19", DailyMinutes: 60},
		"bad date":       {StartDate: "2026-13-01", EndDate: "2026-12-01", DailyMinutes: 60},
		"negative":       {StartDate: "2026-10-19", EndDate: "2026-10-23", DailyMinutes: -1},
		"over 12h":       {StartDate: "2026-10-19", EndDate: "2026-10-23", DailyMinutes: MaxDailyMinutes + 1},
		"too long":       {StartDate: "2026-01-01", EndDate: "2027-01-02", DailyMinutes: 60},
	}
	for name, fields := range cases {
		err := ValidateStaffTargetOverrideFields(fields)
		assert.ErrorIs(t, err, ErrInvalidStaffTargetOverride, name)
	}
}

func TestRejectTargetOverrideOverlap(t *testing.T) {
	t.Parallel()

	existing := []StaffTargetOverride{{ID: 3, StartDate: "2026-10-19", EndDate: "2026-10-23"}}
	touching := StaffTargetOverrideFields{StartDate: "2026-10-23", EndDate: "2026-10-30"}

	err := RejectTargetOverrideOverlap(existing, touching)
	require.ErrorIs(t, err, ErrStaffTargetOverrideRejected)
	var typed *TargetOverrideError
	require.True(t, errors.As(err, &typed))
	assert.Contains(t, typed.Reason, "19.10.2026 bis 23.10.2026")

	assert.NoError(t, RejectTargetOverrideOverlap(existing, StaffTargetOverrideFields{StartDate: "2026-10-24", EndDate: "2026-10-30"}))
}

func TestRejectClosedTargetOverrideMonths(t *testing.T) {
	t.Parallel()

	reopenedAt := time.Date(2026, time.November, 2, 9, 0, 0, 0, time.UTC)
	closed := []*StaffMonthBalanceSnapshot{
		{Year: 2026, Month: 9},
		{Year: 2026, Month: 10, ReopenedAt: &reopenedAt},
	}

	err := RejectClosedTargetOverrideMonths(closed, StaffTargetOverrideFields{StartDate: "2026-09-28", EndDate: "2026-10-02"})
	require.ErrorIs(t, err, ErrStaffTargetOverrideRejected)
	assert.Contains(t, err.Error(), "September 2026 ist abgeschlossen")

	assert.NoError(t, RejectClosedTargetOverrideMonths(closed, StaffTargetOverrideFields{StartDate: "2026-10-19", EndDate: "2026-10-23"}),
		"a reopened month accepts changes again")
}
