package enrollmenthttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Every refusal of a submission keeps its status and code, also when the
// owner wraps it, and only the rate limit carries a retry hint (#2734 moved
// the mapping into a rule table).
func TestMapSubmitError_EveryRefusalKeepsStatusAndCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err    error
		status int
		code   string
	}{
		{capability.ErrEnrollmentDisabled, http.StatusForbidden, "enrollment.disabled"},
		{capability.ErrEnrollmentWindowClosed, http.StatusForbidden, "enrollment.window_closed"},
		{capability.ErrLateInviteInvalid, http.StatusForbidden, "enrollment.late_invite_invalid"},
		{capability.ErrPhaseNotEligible, http.StatusForbidden, "enrollment.phase_not_eligible"},
		{capability.ErrChildClassNotEligible, http.StatusBadRequest, "enrollment.class_not_eligible"},
		{capability.ErrChildGradeNotEligible, http.StatusBadRequest, "enrollment.grade_not_eligible"},
		{capability.ErrChildAlreadyEnrolled, http.StatusBadRequest, "enrollment.child_already_enrolled"},
		{capability.ErrChildNotEnrolled, http.StatusBadRequest, "enrollment.child_not_enrolled"},
		{capability.ErrChildEnrollmentAmbiguous, http.StatusBadRequest, "enrollment.child_ambiguous"},
		{capability.ErrChildEnrollmentNotPermitted, http.StatusForbidden, "enrollment.child_not_permitted"},
		{capability.ErrCareOfferingUnavailable, http.StatusBadRequest, "enrollment.care_offering_unavailable"},
		{capability.ErrCareOfferingMissing, http.StatusBadRequest, "enrollment.care_offering_missing"},
		{capability.ErrCareOfferingExactlyOneRequired, http.StatusBadRequest, "enrollment.care_offering_exactly_one"},
		{capability.ErrRequiredCareOfferingMissing, http.StatusBadRequest, "enrollment.required_care_offering_missing"},
		{capability.ErrCareOfferingsDisabled, http.StatusBadRequest, "enrollment.care_offerings_disabled"},
		{capability.ErrInvalidGuardianPhone, http.StatusBadRequest, "enrollment.invalid_phone"},
		{capability.ErrInvalidGuardianEmail, http.StatusBadRequest, "enrollment.invalid_email"},
		{capability.ErrPickupTimeNotAllowed, http.StatusBadRequest, "enrollment.pickup_time_not_allowed"},
		{capability.ErrDepartureModeLimitExceeded, http.StatusBadRequest, "enrollment.departure_mode_limit"},
		{capability.ErrSelectedDayNotAvailable, http.StatusBadRequest, "enrollment.selected_day_not_available"},
		{capability.ErrDaySelectionRequired, http.StatusBadRequest, "enrollment.day_selection_required"},
		{capability.ErrDaySelectionNotAllowed, http.StatusBadRequest, "enrollment.day_selection_not_allowed"},
		{capability.ErrCareOfferingClosed, http.StatusBadRequest, "enrollment.care_offering_closed"},
		{capability.ErrInvalidSubmission, http.StatusBadRequest, ""},
		{capability.ErrCareOfferingFull, http.StatusConflict, "enrollment.care_offering_full"},
		{capability.ErrDuplicateEnrollment, http.StatusConflict, "enrollment.request_duplicate"},
		{capability.ErrExistingStudentAlreadyRequested, http.StatusConflict, "enrollment.child_already_requested"},
		{capability.ErrRateLimited, http.StatusTooManyRequests, "enrollment.submission_rate_limited"},
		{capability.ErrCaptchaRequired, http.StatusBadRequest, "enrollment.captcha_required"},
		{capability.ErrCaptchaFailed, http.StatusBadRequest, "enrollment.captcha_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.err.Error(), func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			mapSubmitError(w, httptest.NewRequest(http.MethodPost, "/x", nil), fmt.Errorf("submit: %w", tc.err))
			assert.Equal(t, tc.status, w.Code)
			if tc.code != "" {
				var body struct {
					Code string `json:"code"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
				assert.Equal(t, tc.code, body.Code)
			}
			if errors.Is(tc.err, capability.ErrRateLimited) {
				assert.Equal(t, "3600", w.Header().Get("Retry-After"))
			} else {
				assert.Empty(t, w.Header().Get("Retry-After"))
			}
		})
	}
}

// An error the mapping does not know is a server error.
func TestMapSubmitError_UnknownErrorIsServerError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	mapSubmitError(w, httptest.NewRequest(http.MethodPost, "/x", nil), errors.New("captcha provider down"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
