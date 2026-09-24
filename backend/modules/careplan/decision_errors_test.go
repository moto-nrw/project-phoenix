package careplan_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
)

// TestDecisionErrorTexts pins every decision error text of the Care Plan
// owner contract. Handlers render err.Error() to staff and parents, so a
// changed text is a changed API response (#3558).
func TestDecisionErrorTexts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		want string
	}{
		{careplan.ErrOfferingChangeNotFound, "enrollment: offering change request not found"},
		{careplan.ErrOfferingChangeNotPending, "enrollment: offering change request is not pending"},
		{careplan.ErrOfferingChangeDisabled, "enrollment: post-enrollment offering changes are disabled"},
		{careplan.ErrOfferingChangeInvalid, "enrollment: invalid offering change request"},
		{careplan.ErrOfferingChangeNoEnrollment, "enrollment: child has no approved enrollment"},
		{careplan.ErrOfferingChangeForbidden, "enrollment: offering change request forbidden"},
		{careplan.ErrOfferingChangeCapacityFull, "enrollment: care offering is at capacity"},
		{careplan.ErrOfferingChangeDateOutOfRange, "enrollment: confirmed effective date is out of range"},
		{careplan.ErrOfferingChangeAlreadyPending, "enrollment: offering change request already pending"},
		{careplan.ErrCareOfferingsDisabled, "care offerings are disabled for this tenant"},
		{careplan.ErrOfferingAdjustmentInvalid, "offering adjustment is invalid"},
		{careplan.ErrCompleteWithdrawalConfirmationRequired, "Alle Betreuungstage werden entfernt. Bitte bestätigen Sie die Komplett-Abmeldung."},
		{careplan.ErrCourseRequestsDisabled, "enrollment: parent course requests are disabled"},
		{careplan.ErrCourseNotFound, "enrollment: course not found"},
		{careplan.ErrCourseAlreadyBooked, "enrollment: course is already booked"},
		{careplan.ErrCourseRequestNotOwn, "enrollment: not an own course request"},
		{careplan.ErrPickupResetNoOffering, "für diesen Tag gibt es keine Angebots-Gehzeit"},
		{careplan.ErrPickupAdjustmentInvalid, "pickup adjustment: invalid input"},
		{careplan.ErrPickupAdjustmentResolutionRequired, "pickup adjustment: explicit resolution is required"},
		{careplan.ErrPickupAdjustmentStale, "pickup adjustment: preview is stale"},
		{careplan.ErrPickupAdjustmentFutureManualReset, "pickup adjustment: manual pickup times can only be reset today"},
		{careplan.ErrPickupAdjustmentBulkConfirmation, "pickup adjustment: bulk exceptions require confirmation"},
		{careplan.ErrPickupAdjustmentUnauthorized, "pickup adjustment: student is not authorized"},
		{careplan.ErrPickupAdjustmentStudentNotFound, "pickup adjustment: student not found"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.err.Error())
	}
}
