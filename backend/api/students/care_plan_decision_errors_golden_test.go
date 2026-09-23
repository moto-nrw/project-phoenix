package students

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decisionErrorGolden is one rendered answer of a Care Plan decision error:
// the status, the stable code and the exact text the client receives.
type decisionErrorGolden struct {
	err    error
	status int
	code   string
	text   string
}

// TestCarePlanDecisionErrorsGolden pins the status, code and text of every
// Care Plan owner error the staff handlers map (#3558). The texts are
// literals on purpose: handlers render err.Error(), so they are wire texts.
func TestCarePlanDecisionErrorsGolden(t *testing.T) {
	t.Parallel()

	t.Run("offering change decision", func(t *testing.T) {
		t.Parallel()
		for _, tt := range []decisionErrorGolden{
			{careplan.ErrOfferingChangeNotFound, http.StatusNotFound, "", "enrollment: offering change request not found"},
			{careplan.ErrOfferingChangeNotPending, http.StatusConflict, "change_request_not_pending", "enrollment: offering change request is not pending"},
			{careplan.ErrOfferingChangeForbidden, http.StatusForbidden, "", "enrollment: offering change request forbidden"},
			{careplan.ErrCareOfferingsDisabled, http.StatusForbidden, "care_offerings_disabled", "care offerings are disabled for this tenant"},
			{careplan.ErrOfferingChangeCapacityFull, http.StatusConflict, "offering_change_capacity_full", "enrollment: care offering is at capacity"},
			{careplan.ErrOfferingChangeNoEnrollment, http.StatusConflict, "offering_changes_no_enrollment", "enrollment: child has no approved enrollment"},
			{careplan.ErrOfferingChangeDateOutOfRange, http.StatusBadRequest, "offering_change_date_out_of_range", "enrollment: confirmed effective date is out of range"},
			{careplan.ErrCompleteWithdrawalConfirmationRequired, http.StatusConflict, "enrollment.complete_withdrawal_confirmation_required", "Alle Betreuungstage werden entfernt. Bitte bestätigen Sie die Komplett-Abmeldung."},
			{careplan.ErrOfferingChangeInvalid, http.StatusBadRequest, "", "enrollment: invalid offering change request"},
			{careplan.ErrOfferingAdjustmentInvalid, http.StatusBadRequest, "", "offering adjustment is invalid"},
		} {
			assertDecisionErrorGolden(t, tt, rendererStatus(t, offeringDecisionErrorRenderer(tt.err)))
		}
	})

	t.Run("pickup adjustment", func(t *testing.T) {
		t.Parallel()
		for _, tt := range []decisionErrorGolden{
			{careplan.ErrPickupAdjustmentResolutionRequired, http.StatusBadRequest, "pickup.resolution_required", "pickup adjustment: explicit resolution is required"},
			{careplan.ErrPickupAdjustmentStale, http.StatusConflict, "pickup.preview_stale", "pickup adjustment: preview is stale"},
			{careplan.ErrPickupAdjustmentFutureManualReset, http.StatusConflict, "pickup.future_manual_reset", "pickup adjustment: manual pickup times can only be reset today"},
			{careplan.ErrOfferingChangeCapacityFull, http.StatusConflict, "pickup.offering_capacity_full", "enrollment: care offering is at capacity"},
			{careplan.ErrCareOfferingsDisabled, http.StatusConflict, "pickup.offerings_disabled", "care offerings are disabled for this tenant"},
			{careplan.ErrCompleteWithdrawalConfirmationRequired, http.StatusConflict, "enrollment.complete_withdrawal_confirmation_required", "Alle Betreuungstage werden entfernt. Bitte bestätigen Sie die Komplett-Abmeldung."},
			{careplan.ErrOfferingChangeDateOutOfRange, http.StatusBadRequest, "pickup.invalid", "enrollment: confirmed effective date is out of range"},
			{careplan.ErrOfferingChangeInvalid, http.StatusBadRequest, "pickup.invalid", "enrollment: invalid offering change request"},
			{careplan.ErrPickupAdjustmentInvalid, http.StatusBadRequest, "pickup.invalid", "pickup adjustment: invalid input"},
			{careplan.ErrPickupAdjustmentUnauthorized, http.StatusForbidden, "", "pickup adjustment: student is not authorized"},
			{careplan.ErrPickupAdjustmentStudentNotFound, http.StatusNotFound, "", "pickup adjustment: student not found"},
		} {
			recorder := httptest.NewRecorder()
			renderPickupAdjustmentError(recorder, httptest.NewRequest(http.MethodPost, "/", nil), tt.err)

			var body common.ErrResponse
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			body.HTTPStatusCode = recorder.Code
			assertDecisionErrorGolden(t, tt, &body)
		}
	})

	t.Run("pickup reset", func(t *testing.T) {
		t.Parallel()
		tt := decisionErrorGolden{careplan.ErrPickupResetNoOffering, http.StatusConflict, "pickup_reset_requires_offering", "für diesen Tag gibt es keine Angebots-Gehzeit"}
		assertDecisionErrorGolden(t, tt, rendererStatus(t, pickupResetErrorRenderer(tt.err)))
	})

	t.Run("parent request conflict resolution", func(t *testing.T) {
		t.Parallel()
		for _, tt := range []decisionErrorGolden{
			{careplan.ErrOfferingChangeNotFound, http.StatusNotFound, "", "enrollment: offering change request not found"},
			{careplan.ErrOfferingChangeNotPending, http.StatusConflict, "change_request_not_pending", "enrollment: offering change request is not pending"},
			{careplan.ErrOfferingChangeForbidden, http.StatusForbidden, "", "enrollment: offering change request forbidden"},
			{careplan.ErrOfferingChangeInvalid, http.StatusBadRequest, codeStaffValueInvalid, "Der eingetragene Wert passt nicht zu diesen Anfragen. Bitte prüfen Sie ihn."},
		} {
			assertDecisionErrorGolden(t, tt, rendererStatus(t, resolveConflictErrorRenderer(tt.err)))
		}
	})
}

func assertDecisionErrorGolden(t *testing.T, want decisionErrorGolden, got *common.ErrResponse) {
	t.Helper()
	assert.Equal(t, want.status, got.HTTPStatusCode, want.text)
	assert.Equal(t, want.code, got.Code, want.text)
	assert.Equal(t, want.text, got.ErrorText)
}
