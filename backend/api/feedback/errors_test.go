package feedback_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/feedback"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer_NotFoundErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"ErrEntryNotFound", feedbackSvc.ErrEntryNotFound},
		{"ErrStudentNotFound", feedbackSvc.ErrStudentNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := feedback.ErrorRenderer(tt.err)
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
			assert.NotEmpty(t, resp.Status)
		})
	}
}

func TestErrorRenderer_BadRequestErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"ErrInvalidEntryData", feedbackSvc.ErrInvalidEntryData},
		{"ErrInvalidDateRange", feedbackSvc.ErrInvalidDateRange},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := feedback.ErrorRenderer(tt.err)
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
			assert.NotEmpty(t, resp.Status)
		})
	}
}

func TestErrorRenderer_UnknownError(t *testing.T) {
	t.Parallel()

	unknownErr := errors.New("unknown error")
	renderer := feedback.ErrorRenderer(unknownErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "Internal Server Error", resp.Status)
}

func TestErrorInvalidRequest(t *testing.T) {
	t.Parallel()

	testErr := errors.New("invalid input")
	renderer := feedback.ErrorInvalidRequest(testErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	assert.Equal(t, "Invalid Request", resp.Status)
	assert.Equal(t, "invalid input", resp.ErrorText)
}

func TestErrorInternalServer(t *testing.T) {
	t.Parallel()

	testErr := errors.New("database error")
	renderer := feedback.ErrorInternalServer(testErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "Internal Server Error", resp.Status)
	assert.Equal(t, "database error", resp.ErrorText)
}

func TestErrorForbidden(t *testing.T) {
	t.Parallel()

	testErr := errors.New("feature_disabled")
	renderer := feedback.ErrorForbidden(testErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
	assert.Equal(t, "Forbidden", resp.Status)
	assert.Equal(t, "feature_disabled", resp.ErrorText)
}

func TestErrResponse_Render(t *testing.T) {
	t.Parallel()

	errResp := &common.ErrResponse{
		Err:            errors.New("test error"),
		HTTPStatusCode: http.StatusNotFound,
		Status:         "Not Found",
		ErrorText:      "test error",
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := errResp.Render(w, r)
	assert.NoError(t, err)
}
