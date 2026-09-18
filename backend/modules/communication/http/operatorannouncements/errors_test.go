package operatorannouncements_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/http/operatorannouncements"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to extract the operator error body from render.Renderer
func extractErrResponse(t *testing.T, renderer render.Renderer) (int, string, string) {
	t.Helper()
	errResp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok, "Expected *common.OperatorErrResponse")
	return errResp.HTTPStatusCode, errResp.StatusText, errResp.ErrorText
}

func TestAnnouncementErrorRenderer_NotFound(t *testing.T) {
	t.Parallel()

	err := &communication.AnnouncementNotFoundError{AnnouncementID: 999}
	renderer := operatorannouncements.AnnouncementErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "Announcement not found", errorText)
}

func TestAnnouncementErrorRenderer_InvalidData(t *testing.T) {
	t.Parallel()

	innerErr := errors.New("title required")
	err := &communication.InvalidDataError{Err: innerErr}
	renderer := operatorannouncements.AnnouncementErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, errorText, "title required")
}

func TestAnnouncementErrorRenderer_GenericError(t *testing.T) {
	t.Parallel()

	err := errors.New("database error")
	renderer := operatorannouncements.AnnouncementErrorRenderer(err)

	status, _, errorText := extractErrResponse(t, renderer)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "An error occurred", errorText)
}
