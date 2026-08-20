package enrollment

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// --- BuildServiceRequest -------------------------------------------------

func TestBuildServiceRequest_CopiesGuardianFields(t *testing.T) {
	phone := "+49 170 1234567"
	wire := &SubmitEnrollmentRequest{
		PhaseID:           42,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     "anna@example.test",
		GuardianPhone:     &phone,
		ConsentFlags:      map[string]any{"photo": true},
		CustomData:        map[string]any{"notes": "n"},
		Children: []SubmitChildRequest{
			{FirstName: "Lara", LastName: "Beispiel", DateOfBirth: "2018-03-04"},
		},
	}
	out, err := BuildServiceRequest(wire, 7777, "203.0.113.5")
	require.NoError(t, err)
	assert.Equal(t, int64(7777), out.TenantID)
	assert.Equal(t, "203.0.113.5", out.RemoteIP, "RemoteIP threads through for rate limiting")
	assert.Equal(t, int64(42), out.PhaseID)
	assert.Equal(t, "anna@example.test", out.GuardianEmail)
	require.NotNil(t, out.GuardianPhone)
	assert.Equal(t, phone, *out.GuardianPhone)
	assert.Nil(t, out.GuardianAccountID,
		"public path leaves GuardianAccountID nil — only parent JWT path stamps it")
}

func TestBuildServiceRequest_ParsesDateOfBirth(t *testing.T) {
	wire := &SubmitEnrollmentRequest{
		PhaseID: 42,
		Children: []SubmitChildRequest{
			{FirstName: "Lara", LastName: "Beispiel", DateOfBirth: "2018-03-04"},
		},
	}
	out, err := BuildServiceRequest(wire, 7777, "203.0.113.5")
	require.NoError(t, err)
	require.Len(t, out.Children, 1)
	assert.Equal(t,
		timezone.NewDate(2018, 3, 4),
		out.Children[0].DateOfBirth)
}

func TestSubmitEnrollmentRequest_DecodesStringChildID(t *testing.T) {
	var wire SubmitEnrollmentRequest
	err := json.Unmarshal([]byte(`{
		"phase_id": 42,
		"children": [{
			"id": "640000001",
			"first_name": "Lara",
			"last_name": "Beispiel",
			"date_of_birth": "2018-03-04"
		}]
	}`), &wire)
	require.NoError(t, err)
	require.Len(t, wire.Children, 1)
	require.NotNil(t, wire.Children[0].ID)
	assert.Equal(t, int64(640000001), *wire.Children[0].ID)

	out, err := BuildServiceRequest(&wire, 7777, "")
	require.NoError(t, err)
	assert.Equal(t, int64(640000001), out.Children[0].ID)
}

func TestEditBootstrapResponse_ExposesGradeLevelMax(t *testing.T) {
	payload, err := json.Marshal(EditBootstrapResponse{GradeLevelMax: 13})
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	assert.Equal(t, float64(13), decoded["grade_level_max"])
}

func TestToEditDraftChildResponses_PreserveOnlyLockedOfferingsWhenDisabled(t *testing.T) {
	studentID := int64(42)
	responses := toEditDraftChildResponses(&enrollmentService.EditDraft{
		Children: []*enrollmentModels.RequestChild{
			{Model: base.Model{ID: 1}},
			{Model: base.Model{ID: 2}, CreatedStudentID: &studentID},
		},
		OfferingsByChild: map[int64][]*enrollmentModels.RequestChildOffering{
			1: {{CareOfferingID: 6}},
			2: {{CareOfferingID: 7}},
		},
	})

	require.Len(t, responses, 2)
	assert.Empty(t, responses[0].OfferingIDs)
	assert.True(t, responses[1].Locked)
	assert.Equal(t, []string{"7"}, responses[1].OfferingIDs)
}

func TestBuildServiceRequest_RejectsBadDateOfBirth(t *testing.T) {
	wire := &SubmitEnrollmentRequest{
		PhaseID: 42,
		Children: []SubmitChildRequest{
			{FirstName: "Lara", LastName: "Beispiel", DateOfBirth: "yesterday"},
		},
	}
	_, err := BuildServiceRequest(wire, 7777, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "date_of_birth")
	assert.Contains(t, err.Error(), "child 0", "error must report the offending child index")
}

func TestBuildServiceRequest_PreservesOfferingDays(t *testing.T) {
	wire := &SubmitEnrollmentRequest{
		PhaseID: 42,
		Children: []SubmitChildRequest{
			{
				FirstName:   "Lara",
				LastName:    "Beispiel",
				DateOfBirth: "2018-03-04",
				OfferingIDs: []int64{77, 88},
				OfferingDays: []SubmitOfferingDaysRow{
					{OfferingID: 77, SelectedDays: []string{"mon", "wed"}},
				},
			},
		},
	}
	out, err := BuildServiceRequest(wire, 7777, "")
	require.NoError(t, err)
	require.Len(t, out.Children[0].OfferingDays, 1)
	assert.Equal(t, int64(77), out.Children[0].OfferingDays[0].OfferingID)
	assert.Equal(t, []string{"mon", "wed"}, out.Children[0].OfferingDays[0].SelectedDays)
}

func TestBuildServiceRequest_EmptyChildrenOK(t *testing.T) {
	// The service layer enforces ≥1 child via ErrInvalidSubmission. The
	// builder must not double-validate so callers get a consistent error
	// surface.
	out, err := BuildServiceRequest(&SubmitEnrollmentRequest{PhaseID: 42}, 7777, "")
	require.NoError(t, err)
	assert.Empty(t, out.Children)
}

func TestBuildServiceRequest_ReportsCorrectChildIndex(t *testing.T) {
	wire := &SubmitEnrollmentRequest{
		PhaseID: 42,
		Children: []SubmitChildRequest{
			{FirstName: "OK", LastName: "OK", DateOfBirth: "2018-03-04"},
			{FirstName: "Bad", LastName: "Bad", DateOfBirth: "not-a-date"},
		},
	}
	_, err := BuildServiceRequest(wire, 7777, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "child 1", "must report the failing index (1, not 0)")
}

// --- MapSubmitError ------------------------------------------------------

func TestMapSubmitError_EnrollmentDisabled403(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrEnrollmentDisabled)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestMapSubmitError_EnrollmentWindowClosed403(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrEnrollmentWindowClosed)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestMapSubmitError_LateInviteInvalid403WithCode(t *testing.T) {
	// The stable code is read by the parents-portal EnrollmentForm to
	// render the localized late-invite message. Keep this assertion strict.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrLateInviteInvalid)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentLateInviteInvalid)
}

func TestMapSubmitError_CareOfferingMissing400WithCode(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrCareOfferingMissing)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentCareOfferingMissing)
}

func TestMapSubmitError_CareOfferingUnavailable400WithStableCode(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, fmt.Errorf("child 1: %w", enrollmentService.ErrCareOfferingUnavailable))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentCareOfferingUnavailable)
}

func TestMapSubmitError_InvalidGuardianPhone400WithCode(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrInvalidGuardianPhone)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentInvalidPhone)
}

func TestMapSubmitError_InvalidGuardianEmail400WithCode(t *testing.T) {
	// ErrInvalidGuardianEmail wraps ErrInvalidSubmission, so its case must
	// win over the generic branch to attach the stable code the parent form
	// reads to localize the invalid-email message.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrInvalidGuardianEmail)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentInvalidEmail)
}

func TestMapSubmitError_PickupTimeNotAllowed400WithCode(t *testing.T) {
	// ErrPickupTimeNotAllowed wraps ErrInvalidSubmission, so its case must
	// win over the generic branch to attach the stable code the parent form
	// reads to localize the off-list pickup message.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrPickupTimeNotAllowed)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentPickupTimeNotAllowed)
}

func TestMapSubmitError_WrappedPickupTimeNotAllowed400WithCode(t *testing.T) {
	// The service never returns the bare sentinel — it wraps it with the
	// offending child index and field key. The mapper must still resolve the
	// code via errors.Is traversal, not an identity check.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	wrapped := fmt.Errorf("%w: child 0 field %q: off-list",
		enrollmentService.ErrPickupTimeNotAllowed, "schedule_pickup")
	MapSubmitError(w, r, wrapped)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentPickupTimeNotAllowed)
}

func TestMapSubmitError_SelectedDayNotAvailable400WithCode(t *testing.T) {
	// ErrSelectedDayNotAvailable wraps ErrInvalidSubmission, so its case
	// must win over the generic branch to attach the stable code the
	// enrollment form reads to localize the off-days message (#1885).
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrSelectedDayNotAvailable)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentSelectedDayNotAvailable)
}

func TestMapSubmitError_WrappedDepartureModeLimit400WithCode(t *testing.T) {
	// Heimweg-Beschränkung (#2381): the wrapped sentinel must map to its
	// stable code so the parent form can localize the message.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	wrapped := fmt.Errorf("%w: child 0 field %q: weekday \"mon\" allows only one departure mode, got 2",
		enrollmentService.ErrDepartureModeLimitExceeded, "heimwege")
	MapSubmitError(w, r, wrapped)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentDepartureModeLimit)
}

func TestMapSubmitError_WrappedSelectedDayNotAvailable400WithCode(t *testing.T) {
	// The service wraps the sentinel with the offending day; the mapper
	// must still resolve the code via errors.Is traversal.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	wrapped := fmt.Errorf("%w: day %q is not in the offering's available_days",
		enrollmentService.ErrSelectedDayNotAvailable, "tue")
	MapSubmitError(w, r, wrapped)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentSelectedDayNotAvailable)
}

func TestMapSubmitError_DaySelectionRequired400WithCode(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrDaySelectionRequired)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentDaySelectionRequired)
}

func TestMapSubmitError_DaySelectionNotAllowed400WithCode(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrDaySelectionNotAllowed)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentDaySelectionNotAllowed)
}

func TestMapSubmitError_CareOfferingClosed400(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrCareOfferingClosed)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMapSubmitError_InvalidSubmission400(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrInvalidSubmission)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMapSubmitError_CareOfferingFull409WithCode(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrCareOfferingFull)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentCareOfferingFull)
}

func TestMapSubmitError_DuplicateEnrollment409(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrDuplicateEnrollment)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestMapSubmitError_RateLimited429WithRetryAfter(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, enrollmentService.ErrRateLimited)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "3600", w.Header().Get("Retry-After"))
}

func TestMapSubmitError_CaptchaErrorMappedAs400(t *testing.T) {
	// Captcha errors are constructed with fmt.Errorf("captcha ...") in
	// the submit handler, not as a sentinel error. The mapper sniffs
	// the message and re-routes to 400 instead of falling through to a
	// 500. This branch is the parent-portal-vs-public path divergence.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, fmt.Errorf("captcha verification failed"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMapSubmitError_UnknownError500(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	MapSubmitError(w, r, errors.New("synthetic boom"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- remoteIPFromRequest -------------------------------------------------

func TestRemoteIPFromRequest_XForwardedForRightmostHop(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.5, 198.51.100.4, 192.0.2.1")
	r.RemoteAddr = "10.0.0.1:54321"
	assert.Equal(t, "192.0.2.1", remoteIPFromRequestThroughXFFMiddleware(r))
}

func TestRemoteIPFromRequest_XForwardedForSingleEntry(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.5")
	r.RemoteAddr = "10.0.0.1:54321"
	assert.Equal(t, "203.0.113.5", remoteIPFromRequestThroughXFFMiddleware(r))
}

func TestRemoteIPFromRequest_XForwardedForTrimsWhitespace(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Forwarded-For", "  203.0.113.5  ,198.51.100.4")
	assert.Equal(t, "198.51.100.4", remoteIPFromRequestThroughXFFMiddleware(r))
}

func TestRemoteIPFromRequest_FallsBackToRemoteAddrSansPort(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.RemoteAddr = "203.0.113.5:54321"
	assert.Equal(t, "203.0.113.5", remoteIPFromRequest(r))
}

func TestRemoteIPFromRequest_RemoteAddrWithoutPortReturnedRaw(t *testing.T) {
	// httptest sets host:port by default; but in case a frontend hands
	// us an unrouted RemoteAddr (no port), don't lose it.
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.RemoteAddr = "no-port-here"
	assert.Equal(t, "no-port-here", remoteIPFromRequest(r))
}

func TestRemoteIPFromRequest_IPv6BracketsHandled(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.RemoteAddr = "[2001:db8::1]:54321"
	// net.SplitHostPort strips brackets.
	assert.Equal(t, "2001:db8::1", remoteIPFromRequest(r))
}

func remoteIPFromRequestThroughXFFMiddleware(req *http.Request) string {
	var ip string
	router := chi.NewRouter()
	router.Use(chimiddleware.ClientIPFromXFF())
	router.Post("/x", func(w http.ResponseWriter, r *http.Request) {
		ip = remoteIPFromRequest(r)
		w.WriteHeader(http.StatusNoContent)
	})
	router.ServeHTTP(httptest.NewRecorder(), req)
	return ip
}
