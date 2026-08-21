package usercontext

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// ErrorRenderer Tests
// =============================================================================

// Helper to create a UserContextError wrapping a sentinel error
func wrapError(err error) error {
	return &usercontext.UserContextError{Op: "test", Err: err}
}

func TestErrorRenderer_UserNotAuthenticated(t *testing.T) {
	t.Parallel()

	err := wrapError(usercontext.ErrUserNotAuthenticated)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
}

func TestErrorRenderer_UserNotAuthorized(t *testing.T) {
	t.Parallel()

	err := wrapError(usercontext.ErrUserNotAuthorized)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusForbidden, errResp.HTTPStatusCode)
}

func TestErrorRenderer_UserNotFound(t *testing.T) {
	t.Parallel()

	err := wrapError(usercontext.ErrUserNotFound)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestErrorRenderer_UserNotLinkedToPerson(t *testing.T) {
	t.Parallel()

	err := wrapError(usercontext.ErrUserNotLinkedToPerson)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestErrorRenderer_UserNotLinkedToStaff(t *testing.T) {
	t.Parallel()

	err := wrapError(usercontext.ErrUserNotLinkedToStaff)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestErrorRenderer_UserNotLinkedToTeacher(t *testing.T) {
	t.Parallel()

	err := wrapError(usercontext.ErrUserNotLinkedToTeacher)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestErrorRenderer_GroupNotFound(t *testing.T) {
	t.Parallel()

	err := wrapError(usercontext.ErrGroupNotFound)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestErrorRenderer_NoActiveGroups(t *testing.T) {
	t.Parallel()

	err := wrapError(usercontext.ErrNoActiveGroups)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestErrorRenderer_InvalidOperation(t *testing.T) {
	t.Parallel()

	err := wrapError(usercontext.ErrInvalidOperation)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
}

func TestErrorRenderer_GenericUserContextError(t *testing.T) {
	t.Parallel()

	// Create a generic UserContextError that doesn't match specific types
	err := &usercontext.UserContextError{
		Op:  "test",
		Err: errors.New("generic error"),
	}
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
}

func TestErrorRenderer_NonUserContextError(t *testing.T) {
	t.Parallel()

	// Test with a regular error (not a UserContextError)
	err := errors.New("some random error")
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
}

// =============================================================================
// Upload helper tests moved to api/common/ (shared upload package)
// =============================================================================

// =============================================================================
// AllowedImageTypes Tests (now in common package)
// =============================================================================

func TestAllowedImageTypes(t *testing.T) {
	t.Parallel()

	allowed := []string{"image/jpeg", "image/jpg", "image/png", "image/webp"}
	notAllowed := []string{"image/gif", "image/bmp", "application/pdf", "text/html"}

	for _, ct := range allowed {
		assert.True(t, common.AllowedImageTypes[ct], "%s should be allowed", ct)
	}

	for _, ct := range notAllowed {
		assert.False(t, common.AllowedImageTypes[ct], "%s should not be allowed", ct)
	}
}
