package feedback_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/feedback"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer_NotFoundErrors(t *testing.T) {
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
			resp, ok := renderer.(*feedback.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
			assert.NotEmpty(t, resp.StatusText)
		})
	}
}

func TestErrorRenderer_BadRequestErrors(t *testing.T) {
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
			resp, ok := renderer.(*feedback.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
			assert.NotEmpty(t, resp.StatusText)
		})
	}
}

func TestErrorRenderer_UnknownError(t *testing.T) {
	unknownErr := errors.New("unknown error")
	renderer := feedback.ErrorRenderer(unknownErr)
	resp, ok := renderer.(*feedback.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "Internal Server Error", resp.StatusText)
}

func TestErrorInvalidRequest(t *testing.T) {
	testErr := errors.New("invalid input")
	renderer := feedback.ErrorInvalidRequest(testErr)
	resp, ok := renderer.(*feedback.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	assert.Equal(t, "Invalid Request", resp.StatusText)
	assert.Equal(t, "invalid input", resp.ErrorText)
}

func TestErrorInternalServer(t *testing.T) {
	testErr := errors.New("database error")
	renderer := feedback.ErrorInternalServer(testErr)
	resp, ok := renderer.(*feedback.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "Internal Server Error", resp.StatusText)
	assert.Equal(t, "database error", resp.ErrorText)
}

func TestErrResponse_Render(t *testing.T) {
	errResp := &feedback.ErrResponse{
		Err:            errors.New("test error"),
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "Not Found",
		ErrorText:      "test error",
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := errResp.Render(w, r)
	assert.NoError(t, err)
}
