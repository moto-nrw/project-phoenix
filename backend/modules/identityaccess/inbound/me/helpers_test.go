package me

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// ErrorRenderer Tests
// =============================================================================

// Helper to create a CallerError wrapping a sentinel error
func wrapError(err error) error {
	return &identityaccess.CallerError{Op: "test", Err: err}
}

// renderedError renders an error response the way the routes do and returns
// its HTTP status and body.
func renderedError(t *testing.T, renderer render.Renderer) (int, map[string]any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, render.Render(recorder, request, renderer))
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return recorder.Code, body
}

func assertRenderedStatus(t *testing.T, err error, want int) {
	t.Helper()
	status, body := renderedError(t, ErrorRenderer(err))
	assert.Equal(t, want, status)
	assert.Equal(t, "error", body["status"])
	assert.Equal(t, err.Error(), body["error"])
}

func TestErrorRenderer_UserNotAuthenticated(t *testing.T) {
	t.Parallel()
	assertRenderedStatus(t, wrapError(identityaccess.ErrCallerNotAuthenticated), http.StatusUnauthorized)
}

func TestErrorRenderer_UserNotAuthorized(t *testing.T) {
	t.Parallel()
	assertRenderedStatus(t, wrapError(identityaccess.ErrCallerNotAuthorized), http.StatusForbidden)
}

func TestErrorRenderer_UserNotFound(t *testing.T) {
	t.Parallel()
	assertRenderedStatus(t, wrapError(identityaccess.ErrCallerNotFound), http.StatusNotFound)
}

func TestErrorRenderer_UserNotLinkedToPerson(t *testing.T) {
	t.Parallel()
	assertRenderedStatus(t, wrapError(identityaccess.ErrCallerNotLinkedToPerson), http.StatusNotFound)
}

func TestErrorRenderer_UserNotLinkedToStaff(t *testing.T) {
	t.Parallel()
	assertRenderedStatus(t, wrapError(identityaccess.ErrCallerNotLinkedToStaff), http.StatusNotFound)
}

func TestErrorRenderer_UserNotLinkedToTeacher(t *testing.T) {
	t.Parallel()
	assertRenderedStatus(t, wrapError(identityaccess.ErrCallerNotLinkedToTeacher), http.StatusNotFound)
}

func TestErrorRenderer_GroupNotFound(t *testing.T) {
	t.Parallel()
	assertRenderedStatus(t, wrapError(identityaccess.ErrCallerGroupNotFound), http.StatusNotFound)
}

// The old TestErrorRenderer_NoActiveGroups (404) and
// TestErrorRenderer_InvalidOperation (400) covered the sentinels
// ErrNoActiveGroups and ErrInvalidOperation, which no production path ever
// returned; the caller context has no counterpart, so the rows are gone.
// The old TestAllowedImageTypes pinned api/common's upload allowlist, which
// api/common's own upload tests cover.

func TestErrorRenderer_GenericUserContextError(t *testing.T) {
	t.Parallel()
	// A CallerError that wraps no known outcome is a server error.
	assertRenderedStatus(t, &identityaccess.CallerError{Op: "test", Err: errors.New("generic error")}, http.StatusInternalServerError)
}

func TestErrorRenderer_NonUserContextError(t *testing.T) {
	t.Parallel()
	// A plain error (not a CallerError) is a server error.
	assertRenderedStatus(t, errors.New("some random error"), http.StatusInternalServerError)
}
