package operator_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/operator"
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
	err := errors.New("invalid field")
	renderer := operator.ErrInvalidRequest(err)

	status, statusText, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "error", statusText)
	assert.Contains(t, errorText, "invalid field")
}

func TestErrInvalidCredentials(t *testing.T) {
	renderer := operator.ErrInvalidCredentials()

	status, statusText, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "error", statusText)
	assert.Equal(t, "Invalid email or password", errorText)
}

func TestErrNotFound(t *testing.T) {
	renderer := operator.ErrNotFound("Resource not found")

	status, statusText, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "error", statusText)
	assert.Equal(t, "Resource not found", errorText)
}

func TestErrForbidden(t *testing.T) {
	renderer := operator.ErrForbidden("Access denied")

	status, statusText, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "error", statusText)
	assert.Equal(t, "Access denied", errorText)
}

func TestErrInternal(t *testing.T) {
	renderer := operator.ErrInternal("Internal error")

	status, statusText, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "error", statusText)
	assert.Equal(t, "Internal error", errorText)
}

func TestAuthErrorRenderer_InvalidCredentials(t *testing.T) {
	err := &platformSvc.InvalidCredentialsError{}
	renderer := operator.AuthErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "Invalid email or password", errorText)
}

func TestAuthErrorRenderer_OperatorInactive(t *testing.T) {
	err := &platformSvc.OperatorInactiveError{OperatorID: 123}
	renderer := operator.AuthErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "Operator account is inactive", errorText)
}

func TestAuthErrorRenderer_OperatorNotFound(t *testing.T) {
	err := &platformSvc.OperatorNotFoundError{Email: "test@example.com"}
	renderer := operator.AuthErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "Invalid email or password", errorText)
}

func TestAuthErrorRenderer_GenericError(t *testing.T) {
	err := errors.New("database error")
	renderer := operator.AuthErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "Authentication failed", errorText)
}

func TestAnnouncementErrorRenderer_NotFound(t *testing.T) {
	err := &platformSvc.AnnouncementNotFoundError{AnnouncementID: 999}
	renderer := operator.AnnouncementErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "Announcement not found", errorText)
}

func TestAnnouncementErrorRenderer_InvalidData(t *testing.T) {
	innerErr := errors.New("title required")
	err := &platformSvc.InvalidDataError{Err: innerErr}
	renderer := operator.AnnouncementErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, errorText, "title required")
}

func TestAnnouncementErrorRenderer_GenericError(t *testing.T) {
	err := errors.New("database error")
	renderer := operator.AnnouncementErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "An error occurred", errorText)
}

func TestSuggestionsErrorRenderer_PostNotFound(t *testing.T) {
	err := &platformSvc.PostNotFoundError{PostID: 555}
	renderer := operator.SuggestionsErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "Suggestion post not found", errorText)
}

func TestSuggestionsErrorRenderer_CommentNotFound_Platform(t *testing.T) {
	err := &platformSvc.CommentNotFoundError{CommentID: 666}
	renderer := operator.SuggestionsErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "Comment not found", errorText)
}

func TestSuggestionsErrorRenderer_CommentNotFound_Suggestions(t *testing.T) {
	err := &suggestionsSvc.CommentNotFoundError{CommentID: 777}
	renderer := operator.SuggestionsErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "Comment not found", errorText)
}

func TestSuggestionsErrorRenderer_Forbidden(t *testing.T) {
	err := &suggestionsSvc.ForbiddenError{Reason: "Not your comment"}
	renderer := operator.SuggestionsErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Contains(t, errorText, "Not your comment")
}

func TestSuggestionsErrorRenderer_InvalidData(t *testing.T) {
	innerErr := errors.New("content required")
	err := &platformSvc.InvalidDataError{Err: innerErr}
	renderer := operator.SuggestionsErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, errorText, "content required")
}

func TestSuggestionsErrorRenderer_GenericError(t *testing.T) {
	err := errors.New("database error")
	renderer := operator.SuggestionsErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "An error occurred", errorText)
}
