package active

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validBalanceAdjustment() *StaffBalanceAdjustment {
	return &StaffBalanceAdjustment{
		StaffID:       42,
		Type:          BalanceAdjustmentTypePayout,
		MinutesDelta:  -120,
		EffectiveDate: timezone.NewDate(2026, time.March, 1),
		DecidedBy:     43,
	}
}

func TestStaffBalanceAdjustmentValidate_AcceptsEveryType(t *testing.T) {
	t.Parallel()

	for _, adjustmentType := range ValidBalanceAdjustmentTypes {
		adjustment := validBalanceAdjustment()
		adjustment.Type = adjustmentType

		assert.NoError(t, adjustment.Validate(), "type %q must be accepted", adjustmentType)
	}
	// The go-live takeover (#2132) is one of them.
	assert.Contains(t, ValidBalanceAdjustmentTypes, BalanceAdjustmentTypeOpening)
}

func TestStaffBalanceAdjustmentValidate_Rejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*StaffBalanceAdjustment)
		errMsg string
	}{
		{
			name:   "missing staff",
			mutate: func(a *StaffBalanceAdjustment) { a.StaffID = 0 },
			errMsg: "staff ID is required",
		},
		{
			name:   "unknown type",
			mutate: func(a *StaffBalanceAdjustment) { a.Type = "opening_balance" },
			errMsg: "invalid balance adjustment type",
		},
		{
			name:   "missing effective date",
			mutate: func(a *StaffBalanceAdjustment) { a.EffectiveDate = timezone.Date("") },
			errMsg: "effective_date is required",
		},
		{
			name:   "missing decided by",
			mutate: func(a *StaffBalanceAdjustment) { a.DecidedBy = 0 },
			errMsg: "decided_by is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adjustment := validBalanceAdjustment()
			tc.mutate(adjustment)

			err := adjustment.Validate()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}
