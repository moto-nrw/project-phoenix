package timetracking

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifiersPreserveWrappedIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		classify func(error) render.Renderer
		kind     error
		status   int
	}{
		{"stamp", classifyServiceError, workforce.ErrAlreadyCheckedIn, http.StatusConflict},
		{"absence", classifyAbsenceError, workforce.ErrAbsenceNotFound, http.StatusNotFound},
		{"admin", classifyAdminEditError, workforce.ErrAdminSessionNotOwned, http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			failure := fmt.Errorf("outer: %w", &workforce.TimeTrackingError{Kind: tc.kind, Cause: errors.New("a different diagnostic")})
			w := httptest.NewRecorder()
			require.NoError(t, render.Render(w, httptest.NewRequest(http.MethodGet, "/", nil), tc.classify(failure)))
			assert.Equal(t, tc.status, w.Code)
			assert.Contains(t, w.Body.String(), `"detail":"outer: a different diagnostic"`)
		})
	}
}

func TestClassifiersDoNotMatchDiagnosticLookalikes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text string
		classify   func(error) render.Renderer
	}{
		{"stamp", "already checked in", classifyServiceError},
		{"absence", "absence not found", classifyAbsenceError},
		{"admin", "notes required for admin edits", classifyAdminEditError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			require.NoError(t, render.Render(w, httptest.NewRequest(http.MethodGet, "/", nil), tc.classify(errors.New(tc.text))))
			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})
	}
}

// TestSessionDayRejectionsAreInputErrors pins the #6288 save checks: both are
// the caller's input, never a 500, and a check-in in the future carries its
// own code so the person reads what to correct.
func TestSessionDayRejectionsAreInputErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		classify func(error) render.Renderer
		kind     error
		code     string
	}{
		{"combined breaks", classifyServiceError, workforce.ErrTimeTrackingInvalid, `"status":400`},
		{"admin combined breaks", classifyAdminEditError, workforce.ErrTimeTrackingInvalid, `"status":400`},
		{"admin negative break", classifyAdminEditError, workforce.ErrBreakMinutesNegative, `"status":400`},
		{"admin negative duration", classifyAdminEditError, workforce.ErrBreakDurationNegative, `"status":400`},
		{"admin active duration", classifyAdminEditError, workforce.ErrActiveBreakDurationEdit, `"status":400`},
		{"future check-in", classifyServiceError, workforce.ErrSessionCheckInInFuture, `"code":"workforce.session_check_in_in_future"`},
		{"check-in after day", classifyServiceError, workforce.ErrSessionCheckInAfterDate, `"status":400`},
		{"admin check-in after day", classifyAdminEditError, workforce.ErrSessionCheckInAfterDate, `"status":400`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			failure := &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingInvalid, Cause: fmt.Errorf("invalid: %w", tc.kind)}
			w := httptest.NewRecorder()
			require.NoError(t, render.Render(w, httptest.NewRequest(http.MethodPut, "/", nil), tc.classify(failure)))
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), tc.code)
		})
	}
}

func TestAdminBreakOwnershipPrecedesGenericValidation(t *testing.T) {
	t.Parallel()
	failure := &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingInvalid, Cause: workforce.ErrBreakNotOwned}
	w := httptest.NewRecorder()
	require.NoError(t, render.Render(w, httptest.NewRequest(http.MethodPut, "/", nil), classifyAdminEditError(failure)))
	assert.Equal(t, http.StatusNotFound, w.Code)
}
