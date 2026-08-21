package operator_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/operator"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	suggestionsSvc "github.com/moto-nrw/project-phoenix/services/suggestions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to extract ErrResponse from render.Renderer
func extractErrResponse(t *testing.T, renderer render.Renderer) (int, string, string) {
	t.Helper()
	errResp, ok := renderer.(*operator.ErrResponse)
	require.True(t, ok, "Expected *operator.ErrResponse")
	return errResp.HTTPStatusCode, errResp.StatusText, errResp.ErrorText
}

func TestErrInvalidRequest(t *testing.T) {
	t.Parallel()

	err := errors.New("invalid field")
	renderer := operator.ErrInvalidRequest(err)

	status, statusText, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "error", statusText)
	assert.Contains(t, errorText, "invalid field")
}

func TestErrInvalidCredentials(t *testing.T) {
	t.Parallel()

	renderer := operator.ErrInvalidCredentials()

	status, statusText, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "error", statusText)
	assert.Equal(t, "Invalid email or password", errorText)
}

func TestErrNotFound(t *testing.T) {
	t.Parallel()

	renderer := operator.ErrNotFound("Resource not found")

	status, statusText, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "error", statusText)
	assert.Equal(t, "Resource not found", errorText)
}

func TestErrForbidden(t *testing.T) {
	t.Parallel()

	renderer := operator.ErrForbidden("Access denied")

	status, statusText, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "error", statusText)
	assert.Equal(t, "Access denied", errorText)
}

func TestErrInternal(t *testing.T) {
	t.Parallel()

	renderer := operator.ErrInternal("Internal error")

	status, statusText, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "error", statusText)
	assert.Equal(t, "Internal error", errorText)
}

func TestErrServiceUnavailable(t *testing.T) {
	t.Parallel()

	renderer := operator.ErrServiceUnavailable("MFA status temporarily unavailable, please retry")

	status, statusText, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Equal(t, "Service Unavailable", statusText)
	assert.Equal(t, "MFA status temporarily unavailable, please retry", errorText)
}

func TestAuthErrorRenderer_InvalidCredentials(t *testing.T) {
	t.Parallel()

	err := &platformSvc.InvalidCredentialsError{}
	renderer := operator.AuthErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "Invalid email or password", errorText)
}

func TestAuthErrorRenderer_OperatorInactive(t *testing.T) {
	t.Parallel()

	err := &platformSvc.OperatorInactiveError{OperatorID: 123}
	renderer := operator.AuthErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "Operator account is inactive", errorText)
}

func TestAuthErrorRenderer_OperatorNotFound(t *testing.T) {
	t.Parallel()

	err := &platformSvc.OperatorNotFoundError{Email: "test@example.com"}
	renderer := operator.AuthErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "Invalid email or password", errorText)
}

func TestAuthErrorRenderer_GenericError(t *testing.T) {
	t.Parallel()

	err := errors.New("database error")
	renderer := operator.AuthErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "Authentication failed", errorText)
}

// TestAuthErrorRenderer_MFAStatusUnavailable (Item #3) — an operator MFA
// gate that hit a non-not-found infra error during HasEnrollment must surface
// as 503 so it fails-closed for THIS caller without locking everyone else
// out (operator MFA is mandatory; silently treating it as "not enrolled"
// would issue an enrollment-token bypass during a credentials-table outage).
func TestAuthErrorRenderer_MFAStatusUnavailable(t *testing.T) {
	t.Parallel()

	renderer := operator.AuthErrorRenderer(authService.ErrMFAStatusUnavailable)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Equal(t, "MFA status temporarily unavailable, please retry", errorText)
}

func TestAnnouncementErrorRenderer_NotFound(t *testing.T) {
	t.Parallel()

	err := &platformSvc.AnnouncementNotFoundError{AnnouncementID: 999}
	renderer := operator.AnnouncementErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "Announcement not found", errorText)
}

func TestAnnouncementErrorRenderer_InvalidData(t *testing.T) {
	t.Parallel()

	innerErr := errors.New("title required")
	err := &platformSvc.InvalidDataError{Err: innerErr}
	renderer := operator.AnnouncementErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, errorText, "title required")
}

func TestAnnouncementErrorRenderer_GenericError(t *testing.T) {
	t.Parallel()

	err := errors.New("database error")
	renderer := operator.AnnouncementErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "An error occurred", errorText)
}

func TestSuggestionsErrorRenderer_PostNotFound(t *testing.T) {
	t.Parallel()

	err := &platformSvc.PostNotFoundError{PostID: 555}
	renderer := operator.SuggestionsErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "Suggestion post not found", errorText)
}

func TestSuggestionsErrorRenderer_CommentNotFound_Platform(t *testing.T) {
	t.Parallel()

	err := &platformSvc.CommentNotFoundError{CommentID: 666}
	renderer := operator.SuggestionsErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "Comment not found", errorText)
}

func TestSuggestionsErrorRenderer_CommentNotFound_Suggestions(t *testing.T) {
	t.Parallel()

	err := &suggestionsSvc.CommentNotFoundError{CommentID: 777}
	renderer := operator.SuggestionsErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "Comment not found", errorText)
}

func TestSuggestionsErrorRenderer_Forbidden(t *testing.T) {
	t.Parallel()

	err := &suggestionsSvc.ForbiddenError{Reason: "Not your comment"}
	renderer := operator.SuggestionsErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Contains(t, errorText, "Not your comment")
}

func TestSuggestionsErrorRenderer_InvalidData(t *testing.T) {
	t.Parallel()

	innerErr := errors.New("content required")
	err := &platformSvc.InvalidDataError{Err: innerErr}
	renderer := operator.SuggestionsErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, errorText, "content required")
}

func TestSuggestionsErrorRenderer_GenericError(t *testing.T) {
	t.Parallel()

	err := errors.New("database error")
	renderer := operator.SuggestionsErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "An error occurred", errorText)
}
