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
		{"offering change disabled", ErrOfferingChangeDisabled, careplan.ErrOfferingChangeDisabled, ErrOfferingChangeDisabled},
		{"offering change invalid", ErrOfferingChangeInvalid, careplan.ErrOfferingChangeInvalid, ErrOfferingChangeInvalid},
		{"offering change no enrollment", ErrOfferingChangeNoEnrollment, careplan.ErrOfferingChangeNoEnrollment, ErrOfferingChangeNoEnrollment},
		{"offering change forbidden", ErrOfferingChangeForbidden, careplan.ErrOfferingChangeForbidden, ErrOfferingChangeForbidden},
		{"offering change capacity full", ErrOfferingChangeCapacityFull, careplan.ErrOfferingChangeCapacityFull, ErrOfferingChangeCapacityFull},
		{"offering change date out of range", ErrOfferingChangeDateOutOfRange, careplan.ErrOfferingChangeDateOutOfRange, ErrOfferingChangeDateOutOfRange},
		{"care offerings disabled", ErrCareOfferingsDisabled, careplan.ErrCareOfferingsDisabled, ErrCareOfferingsDisabled},
		{"offering adjustment invalid", ErrOfferingAdjustmentInvalid, careplan.ErrOfferingAdjustmentInvalid, ErrOfferingAdjustmentInvalid},
		{"complete withdrawal confirmation", ErrCompleteWithdrawalConfirmationRequired, careplan.ErrCompleteWithdrawalConfirmationRequired, ErrCompleteWithdrawalConfirmationRequired},
		{"course requests disabled", ErrCourseRequestsDisabled, careplan.ErrCourseRequestsDisabled, ErrCourseRequestsDisabled},
		{"course not found", ErrCourseNotFound, careplan.ErrCourseNotFound, ErrCourseNotFound},
		{"course already booked", ErrCourseAlreadyBooked, careplan.ErrCourseAlreadyBooked, ErrCourseAlreadyBooked},
		{"course request not own", ErrCourseRequestNotOwn, careplan.ErrCourseRequestNotOwn, ErrCourseRequestNotOwn},
		{"pickup reset no offering", ErrPickupResetNoOffering, careplan.ErrPickupResetNoOffering, ErrPickupResetNoOffering},
		{"pickup adjustment invalid", ErrPickupAdjustmentInvalid, careplan.ErrPickupAdjustmentInvalid, ErrPickupAdjustmentInvalid},
		{"pickup adjustment resolution required", ErrPickupAdjustmentResolutionRequired, careplan.ErrPickupAdjustmentResolutionRequired, ErrPickupAdjustmentResolutionRequired},
		{"pickup adjustment stale", ErrPickupAdjustmentStale, careplan.ErrPickupAdjustmentStale, ErrPickupAdjustmentStale},
		{"pickup adjustment future manual reset", ErrPickupAdjustmentFutureManualReset, careplan.ErrPickupAdjustmentFutureManualReset, ErrPickupAdjustmentFutureManualReset},
		{"pickup adjustment bulk confirmation", ErrPickupAdjustmentBulkConfirmation, careplan.ErrPickupAdjustmentBulkConfirmation, ErrPickupAdjustmentBulkConfirmation},
		{"pickup adjustment unauthorized", ErrPickupAdjustmentUnauthorized, careplan.ErrPickupAdjustmentUnauthorized, ErrPickupAdjustmentUnauthorized},
		{"pickup adjustment student not found", ErrPickupAdjustmentStudentNotFound, careplan.ErrPickupAdjustmentStudentNotFound, ErrPickupAdjustmentStudentNotFound},
		// models/enrollment cannot alias the owner; this package returns a
		// value that matches both names instead.
		{"offering change not found", errOfferingChangeNotFound, careplan.ErrOfferingChangeNotFound, enrollmentModels.ErrOfferingChangeNotFound},
		{"offering change not pending", errOfferingChangeNotPending, careplan.ErrOfferingChangeNotPending, enrollmentModels.ErrOfferingChangeNotPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
