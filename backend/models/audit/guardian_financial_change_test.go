package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGuardianFinancialChangeValidate pins the shape rules of the payment
// trail: bank fields belong to a guardian, the payer mark belongs to one child,
// and a row that records no difference is not a change.
func TestGuardianFinancialChangeValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		change  GuardianFinancialChange
		wantErr string
	}{
		{
			name: "bank change without a student is valid",
			change: GuardianFinancialChange{
				GuardianProfileID: 7,
				ChangedBy:         3,
				FieldName:         GuardianPaymentFieldIBAN,
				NewValue:          "•••• 3000",
			},
		},
		{
			name: "payer change carries the child",
			change: GuardianFinancialChange{
				GuardianProfileID: 7,
				StudentID:         int64Ptr(11),
				ChangedBy:         3,
				FieldName:         GuardianPaymentFieldIsPayer,
				OldValue:          "false",
				NewValue:          "true",
			},
		},
		{
			name: "bank change must not carry a child",
			change: GuardianFinancialChange{
				GuardianProfileID: 7,
				StudentID:         int64Ptr(11),
				ChangedBy:         3,
				FieldName:         GuardianPaymentFieldIBAN,
				NewValue:          "•••• 3000",
			},
			wantErr: "guardian-scoped",
		},
		{
			name: "payer change without a child is rejected",
			change: GuardianFinancialChange{
				GuardianProfileID: 7,
				ChangedBy:         3,
				FieldName:         GuardianPaymentFieldIsPayer,
				NewValue:          "true",
			},
			wantErr: "student_id is required",
		},
		{
			name: "unknown field is rejected",
			change: GuardianFinancialChange{
				GuardianProfileID: 7,
				ChangedBy:         3,
				FieldName:         "account_number",
				NewValue:          "x",
			},
			wantErr: "unknown guardian payment field",
		},
		{
			name: "identical values are not a change",
			change: GuardianFinancialChange{
				GuardianProfileID: 7,
				ChangedBy:         3,
				FieldName:         GuardianPaymentFieldAccountHolder,
				OldValue:          "Sabine Schneider",
				NewValue:          "Sabine Schneider",
			},
			wantErr: "identical",
		},
		{
			name: "actor is required",
			change: GuardianFinancialChange{
				GuardianProfileID: 7,
				FieldName:         GuardianPaymentFieldIBAN,
				NewValue:          "•••• 3000",
			},
			wantErr: "changed_by is required",
		},
		{
			name: "guardian is required",
			change: GuardianFinancialChange{
				ChangedBy: 3,
				FieldName: GuardianPaymentFieldIBAN,
				NewValue:  "•••• 3000",
			},
			wantErr: "guardian_profile_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.change.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
