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
