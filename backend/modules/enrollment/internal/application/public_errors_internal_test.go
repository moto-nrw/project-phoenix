package application

import (
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/assert"
)

// TestCarePlanDecisionErrorContract pins that every Care Plan refusal the
// enrollment flows return matches the Care Plan owner value and Enrollment's
// public value under errors.Is, with the owner's text (#3558, #3565).
// Handlers map the Enrollment value only, so a returned error that stops
// matching it turns a 4xx into a 500.
func TestCarePlanDecisionErrorContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		owner  error
		public error
	}{
		{"care offerings disabled", careplan.ErrCareOfferingsDisabled, enrollment.ErrCareOfferingsDisabled},
		{"offering adjustment invalid", careplan.ErrOfferingAdjustmentInvalid, enrollment.ErrOfferingAdjustmentInvalid},
		{"complete withdrawal confirmation", careplan.ErrCompleteWithdrawalConfirmationRequired, enrollment.ErrCompleteWithdrawalConfirmationRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			returned := publicError(tt.owner)
			wrapped := fmt.Errorf("decide: %w", returned)

			assert.ErrorIs(t, wrapped, tt.owner)
			assert.ErrorIs(t, wrapped, tt.public)
			assert.Equal(t, tt.owner.Error(), returned.Error())
			assert.Equal(t, tt.public.Error(), returned.Error())
		})
	}
}

// publicError leaves nil and errors without an originating value alone.
func TestPublicErrorPassesUnrelatedErrorsThrough(t *testing.T) {
	t.Parallel()

	assert.NoError(t, publicError(nil))
	other := errors.New("database unavailable")
	assert.Same(t, other, publicError(other), "an error without an originating value stays unchanged")
	assert.Equal(t, publicError(careplan.ErrCareOfferingsDisabled).Error(), PublicError(careplan.ErrCareOfferingsDisabled).Error())
}
