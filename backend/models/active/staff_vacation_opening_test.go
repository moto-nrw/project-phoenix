package active

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Vacation takeover model validation (#2132). The row is the audit record of
// the go-live handover, so every guard below is what keeps an unusable
// takeover out of the ledger.

func validVacationOpening() *StaffVacationOpening {
	return &StaffVacationOpening{
		StaffID:              42,
		Year:                 2026,
		EffectiveDate:        timezone.NewDate(2026, time.March, 1),
		TakenBeforeDays:      7.5,
		EnteredRemainingDays: 22.5,
		Note:                 "Übernahme aus Altsystem",
		DecidedBy:            43,
	}
}

func TestStaffVacationOpeningValidate_Valid(t *testing.T) {
	t.Parallel()

	require.NoError(t, validVacationOpening().Validate())
}

func TestStaffVacationOpeningValidate_NegativeTakenBeforeIsAllowed(t *testing.T) {
	t.Parallel()

	// Resturlaub above entitled + carryover: extra credit carried over from
	// the old system. Legitimate, so it must not be rejected.
	opening := validVacationOpening()
	opening.TakenBeforeDays = -3.5
	opening.EnteredRemainingDays = 33.5

	assert.NoError(t, opening.Validate())
}

func TestStaffVacationOpeningValidate_Rejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*StaffVacationOpening)
		errMsg string
	}{
		{
			name:   "missing staff",
			mutate: func(o *StaffVacationOpening) { o.StaffID = 0 },
			errMsg: "staff_id is required",
		},
		{
			name:   "year below range",
			mutate: func(o *StaffVacationOpening) { o.Year = 1999 },
			errMsg: "year out of range",
		},
		{
			name:   "year above range",
			mutate: func(o *StaffVacationOpening) { o.Year = 2101 },
			errMsg: "year out of range",
		},
		{
			name:   "missing effective date",
			mutate: func(o *StaffVacationOpening) { o.EffectiveDate = timezone.Date{} },
			errMsg: "effective_date is required",
		},
		{
			name: "effective date in another year",
			mutate: func(o *StaffVacationOpening) {
				o.EffectiveDate = timezone.NewDate(2025, time.March, 1)
			},
			errMsg: "effective_date must lie in the opening year",
		},
		{
			name:   "taken before below range",
			mutate: func(o *StaffVacationOpening) { o.TakenBeforeDays = -1000 },
			errMsg: "taken_before_days out of range",
		},
		{
			name:   "taken before above range",
			mutate: func(o *StaffVacationOpening) { o.TakenBeforeDays = 1000 },
			errMsg: "taken_before_days out of range",
		},
		{
			name:   "entered remaining out of range",
			mutate: func(o *StaffVacationOpening) { o.EnteredRemainingDays = 1000 },
			errMsg: "entered_remaining_days out of range",
		},
		{
			name:   "more than one decimal place in taken before",
			mutate: func(o *StaffVacationOpening) { o.TakenBeforeDays = 7.25 },
			errMsg: "vacation opening days must have at most one decimal place",
		},
		{
			name:   "more than one decimal place in entered remaining",
			mutate: func(o *StaffVacationOpening) { o.EnteredRemainingDays = 22.25 },
			errMsg: "vacation opening days must have at most one decimal place",
		},
		{
			name:   "missing decided by",
			mutate: func(o *StaffVacationOpening) { o.DecidedBy = 0 },
			errMsg: "decided_by is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opening := validVacationOpening()
			tc.mutate(opening)

			err := opening.Validate()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestHasOneDecimalPlace(t *testing.T) {
	t.Parallel()

	assert.True(t, hasOneDecimalPlace(0))
	assert.True(t, hasOneDecimalPlace(12))
	assert.True(t, hasOneDecimalPlace(12.5))
	assert.True(t, hasOneDecimalPlace(-3.5))
	assert.False(t, hasOneDecimalPlace(12.25))
	assert.False(t, hasOneDecimalPlace(-0.05))
}

// The quota is the takeover's reference value (entitled + carryover −
// remaining = taken before), so its own precision guard matters just as much.
func TestStaffVacationQuotaValidate(t *testing.T) {
	t.Parallel()

	valid := &StaffVacationQuota{StaffID: 42, Year: 2026, EntitledDays: 30, CarryoverDays: 2.5}
	require.NoError(t, valid.Validate())

	tests := []struct {
		name   string
		mutate func(*StaffVacationQuota)
		errMsg string
	}{
		{"missing staff", func(q *StaffVacationQuota) { q.StaffID = 0 }, "staff_id is required"},
		{"year out of range", func(q *StaffVacationQuota) { q.Year = 1999 }, "year out of range"},
		{"entitled out of range", func(q *StaffVacationQuota) { q.EntitledDays = -1 }, "entitled_days out of range"},
		{"carryover out of range", func(q *StaffVacationQuota) { q.CarryoverDays = -1 }, "carryover_days out of range"},
		{"quarter day entitled", func(q *StaffVacationQuota) { q.EntitledDays = 30.25 }, "at most one decimal place"},
		{"quarter day carryover", func(q *StaffVacationQuota) { q.CarryoverDays = 2.25 }, "at most one decimal place"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			quota := &StaffVacationQuota{StaffID: 42, Year: 2026, EntitledDays: 30, CarryoverDays: 2.5}
			tc.mutate(quota)

			err := quota.Validate()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}
