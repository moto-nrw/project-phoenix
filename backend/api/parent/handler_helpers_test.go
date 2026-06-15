package parent

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// --- parseLeadingGrade ---------------------------------------------------

func TestParseLeadingGrade_DigitsThenLetter(t *testing.T) {
	assert.Equal(t, 1, parseLeadingGrade("1a"))
	assert.Equal(t, 4, parseLeadingGrade("4b"))
}

func TestParseLeadingGrade_AllDigits(t *testing.T) {
	assert.Equal(t, 10, parseLeadingGrade("10"))
}

func TestParseLeadingGrade_LeadingLetterIsZero(t *testing.T) {
	// "a1" has no leading digit run → return 0 (the caller treats 0 as
	// "couldn't determine grade", same as a malformed input).
	assert.Equal(t, 0, parseLeadingGrade("a1"))
}

func TestParseLeadingGrade_EmptyIsZero(t *testing.T) {
	assert.Equal(t, 0, parseLeadingGrade(""))
}

func TestParseLeadingGrade_PurePunctuationIsZero(t *testing.T) {
	assert.Equal(t, 0, parseLeadingGrade("---"))
}

func TestParseLeadingGrade_MultiDigitThenLetter(t *testing.T) {
	assert.Equal(t, 11, parseLeadingGrade("11x"))
}

// --- buildParentServiceRequest -------------------------------------------

func TestBuildParentServiceRequest_StampsTenantAndAccount(t *testing.T) {
	wire := &submitParentEnrollmentRequest{
		PhaseID:           42,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     "anna@example.test",
		ConsentFlags:      map[string]any{"photo": true},
		CustomData:        map[string]any{"notes": "n"},
		Children: []submitParentChildRequest{
			{FirstName: "Lara", LastName: "Beispiel", DateOfBirth: "2018-03-04"},
		},
	}
	out, err := buildParentServiceRequest(wire, 7777, 4321, "203.0.113.42")
	require.NoError(t, err)
	assert.Equal(t, int64(7777), out.TenantID)
	require.NotNil(t, out.GuardianAccountID)
	assert.Equal(t, int64(4321), *out.GuardianAccountID,
		"GuardianAccountID is the parent JWT signal that lets the decision service skip the invite step")
	assert.Equal(t, int64(42), out.PhaseID)
	assert.Equal(t, "anna@example.test", out.GuardianEmail)
	assert.True(t, out.ConsentFlags["photo"].(bool))
	assert.Equal(t, "203.0.113.42", out.RemoteIP,
		"RemoteIP is forwarded so per-IP rate limiting applies even on authenticated parent submits")
	require.Len(t, out.Children, 1)
	assert.Equal(t, timezone.NewDate(2018, 3, 4), out.Children[0].DateOfBirth)
}

func TestBuildParentServiceRequest_PreservesOfferingDays(t *testing.T) {
	wire := &submitParentEnrollmentRequest{
		PhaseID: 42,
		Children: []submitParentChildRequest{
			{
				FirstName:   "Lara",
				LastName:    "Beispiel",
				DateOfBirth: "2018-03-04",
				OfferingIDs: []int64{77, 88},
				OfferingDays: []submitParentOfferingDaysEntry{
					{OfferingID: 77, SelectedDays: []string{"mon", "wed"}},
				},
			},
		},
	}
	out, err := buildParentServiceRequest(wire, 7777, 4321, "203.0.113.42")
	require.NoError(t, err)
	require.Len(t, out.Children, 1)
	assert.Equal(t, []int64{77, 88}, out.Children[0].OfferingIDs)
	require.Len(t, out.Children[0].OfferingDays, 1)
	assert.Equal(t, int64(77), out.Children[0].OfferingDays[0].OfferingID)
	assert.Equal(t, []string{"mon", "wed"}, out.Children[0].OfferingDays[0].SelectedDays)
}

func TestBuildParentServiceRequest_RejectsBadDateOfBirth(t *testing.T) {
	wire := &submitParentEnrollmentRequest{
		PhaseID: 42,
		Children: []submitParentChildRequest{
			{FirstName: "Lara", LastName: "Beispiel", DateOfBirth: "not-a-date"},
		},
	}
	_, err := buildParentServiceRequest(wire, 7777, 4321, "203.0.113.42")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "date_of_birth")
	assert.Contains(t, err.Error(), "YYYY-MM-DD")
}

func TestBuildParentServiceRequest_EmptyChildrenOK(t *testing.T) {
	// buildParentServiceRequest itself doesn't enforce ≥1 child — that
	// belongs to the service layer's validateSubmission. The handler
	// should hand off cleanly even with zero children so the service
	// can produce the expected ErrInvalidSubmission.
	wire := &submitParentEnrollmentRequest{PhaseID: 42}
	out, err := buildParentServiceRequest(wire, 7777, 4321, "203.0.113.42")
	require.NoError(t, err)
	assert.Empty(t, out.Children)
}

func TestBuildParentServiceRequest_TargetGradeLevelPassesThrough(t *testing.T) {
	g := int16(2)
	wire := &submitParentEnrollmentRequest{
		PhaseID: 42,
		Children: []submitParentChildRequest{
			{FirstName: "Lara", LastName: "Beispiel", DateOfBirth: "2018-03-04", TargetGradeLevel: &g},
		},
	}
	out, err := buildParentServiceRequest(wire, 7777, 4321, "203.0.113.42")
	require.NoError(t, err)
	require.NotNil(t, out.Children[0].TargetGradeLevel)
	assert.Equal(t, int16(2), *out.Children[0].TargetGradeLevel)
}

// --- mapParentSubmitError ------------------------------------------------

func TestMapParentSubmitError_EnrollmentDisabled403(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	mapParentSubmitError(w, r, enrollmentService.ErrEnrollmentDisabled)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestMapParentSubmitError_EnrollmentWindowClosed403(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	mapParentSubmitError(w, r, enrollmentService.ErrEnrollmentWindowClosed)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestMapParentSubmitError_CareOfferingClosed400(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	mapParentSubmitError(w, r, enrollmentService.ErrCareOfferingClosed)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMapParentSubmitError_InvalidSubmission400(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	mapParentSubmitError(w, r, enrollmentService.ErrInvalidSubmission)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// An invalid optional co-guardian phone must map to 400 with the stable
// code, not 500. ErrInvalidGuardianPhone does NOT wrap ErrInvalidSubmission,
// so without an explicit case it would fall through to the default 500.
func TestMapParentSubmitError_InvalidGuardianPhone400WithCode(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	mapParentSubmitError(w, r, enrollmentService.ErrInvalidGuardianPhone)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "enrollment.invalid_phone")
}

func TestMapParentSubmitError_InvalidGuardianEmail400WithCode(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	mapParentSubmitError(w, r, enrollmentService.ErrInvalidGuardianEmail)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "enrollment.invalid_email")
}

func TestMapParentSubmitError_CareOfferingFull409WithCode(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	mapParentSubmitError(w, r, enrollmentService.ErrCareOfferingFull)
	assert.Equal(t, http.StatusConflict, w.Code)
	// Stable code is read by the parents-portal form to render the
	// friendly German message. Keep this assertion strict.
	assert.Contains(t, w.Body.String(), "enrollment.care_offering_full")
}

func TestMapParentSubmitError_DuplicateEnrollment409(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	mapParentSubmitError(w, r, enrollmentService.ErrDuplicateEnrollment)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestMapParentSubmitError_RateLimited429WithRetryAfter(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	mapParentSubmitError(w, r, enrollmentService.ErrRateLimited)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "3600", w.Header().Get("Retry-After"),
		"Retry-After must be set so the client can back off")
}

func TestMapParentSubmitError_UnknownError500(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	mapParentSubmitError(w, r, errors.New("synthetic boom"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- toChildResponse -----------------------------------------------------

func TestToChildResponse_StringifiesIDs(t *testing.T) {
	now := timezone.NewDate(2026, 9, 1)
	in := &parentModels.ChildSummary{
		StudentID:    12345,
		TenantID:     6789,
		FirstName:    "Lara",
		LastName:     "Beispiel",
		SchoolClass:  "1a",
		Status:       "active",
		EnrolledFrom: &now,
		SchoolName:   "OGS Sonnenschule",
		SchoolSlug:   "sonnenschule",
	}
	out := toChildResponse(in)
	assert.Equal(t, "12345", out.StudentID, "int64 IDs are stringified per CLAUDE.md rule 4")
	assert.Equal(t, "6789", out.TenantID)
	assert.Equal(t, "Lara", out.FirstName)
	assert.Equal(t, "1a", out.SchoolClass)
	assert.Equal(t, "active", out.Status)
	require.NotNil(t, out.EnrolledFrom)
	assert.Equal(t, now, *out.EnrolledFrom)
	assert.Nil(t, out.EnrolledUntil, "nil pointers pass through unchanged")
}
