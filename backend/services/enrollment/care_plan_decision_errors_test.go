package enrollment

import (
	"fmt"
	"testing"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
)

// TestCarePlanDecisionErrorContract pins that every legacy decision error this
// package still returns matches the Care Plan owner value and its legacy name
// under errors.Is, with the owner's text (#3558). Handlers map the owner value
// only, so a legacy value that stops matching turns a 4xx into a 500.
func TestCarePlanDecisionErrorContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		returned error
		owner    error
		legacy   error
	}{
		{"care offerings disabled", ErrCareOfferingsDisabled, careplan.ErrCareOfferingsDisabled, ErrCareOfferingsDisabled},
		{"offering adjustment invalid", ErrOfferingAdjustmentInvalid, careplan.ErrOfferingAdjustmentInvalid, ErrOfferingAdjustmentInvalid},
		{"complete withdrawal confirmation", ErrCompleteWithdrawalConfirmationRequired, careplan.ErrCompleteWithdrawalConfirmationRequired, ErrCompleteWithdrawalConfirmationRequired},
		// models/enrollment cannot alias the owner; this package returns a
		// value that matches both names instead.
		{"offering change not found", errOfferingChangeNotFound, careplan.ErrOfferingChangeNotFound, enrollmentModels.ErrOfferingChangeNotFound},
		{"offering change not pending", errOfferingChangeNotPending, careplan.ErrOfferingChangeNotPending, enrollmentModels.ErrOfferingChangeNotPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.legacy == tt.returned {
				// A legacy name of this package is the owner value itself.
				assert.True(t, tt.returned == tt.owner, "legacy name must alias the owner value") //nolint:errorlint // identity is the contract
			}
			wrapped := fmt.Errorf("decide: %w", tt.returned)

			assert.ErrorIs(t, wrapped, tt.owner)
			assert.ErrorIs(t, wrapped, tt.legacy)
			assert.Equal(t, tt.owner.Error(), tt.returned.Error())
			assert.Equal(t, tt.legacy.Error(), tt.returned.Error())
		})
	}
}

// TestCarePlanDecisionErrorContract_TranslatesOwnerNotPending pins the Care
// Plan facade's not-pending outcome after this package translates it.
func TestCarePlanDecisionErrorContract_TranslatesOwnerNotPending(t *testing.T) {
	t.Parallel()

	err := (&offeringChangeCarePlanRepository{}).pendingError("decide", careplan.ErrOfferingChangeNotPending)

	assert.ErrorIs(t, err, careplan.ErrOfferingChangeNotPending)
	assert.ErrorIs(t, err, enrollmentModels.ErrOfferingChangeNotPending)
}
