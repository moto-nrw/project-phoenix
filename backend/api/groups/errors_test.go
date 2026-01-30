package groups_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	groupsAPI "github.com/moto-nrw/project-phoenix/api/groups"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer_NotFoundErrors(t *testing.T) {
	tests := []struct {
		name    string
		baseErr error
	}{
		{"ErrGroupNotFound", education.ErrGroupNotFound},
		{"ErrGroupTeacherNotFound", education.ErrGroupTeacherNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eduErr := &education.EducationError{Err: tt.baseErr}
			renderer := groupsAPI.ErrorRenderer(eduErr)
			resp, ok := renderer.(*groupsAPI.ErrorResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
			assert.Equal(t, "error", resp.Status)
		})
	}
}

func TestErrorRenderer_ConflictErrors(t *testing.T) {
	eduErr := &education.EducationError{Err: education.ErrDuplicateGroup}
	renderer := groupsAPI.ErrorRenderer(eduErr)
	resp, ok := renderer.(*groupsAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
}

func TestErrorRenderer_InvalidRequestErrors(t *testing.T) {
	tests := []struct {
		name    string
		baseErr error
	}{
		{"ErrRoomNotFound", education.ErrRoomNotFound},
		{"ErrTeacherNotFound", education.ErrTeacherNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eduErr := &education.EducationError{Err: tt.baseErr}
			renderer := groupsAPI.ErrorRenderer(eduErr)
			resp, ok := renderer.(*groupsAPI.ErrorResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
			assert.Equal(t, "error", resp.Status)
		})
	}
}

func TestErrorRenderer_UnknownEducationError(t *testing.T) {
	eduErr := &education.EducationError{Err: errors.New("unknown error")}
	renderer := groupsAPI.ErrorRenderer(eduErr)
	resp, ok := renderer.(*groupsAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
}

func TestErrorRenderer_NonEducationError(t *testing.T) {
	plainErr := errors.New("generic error")
	renderer := groupsAPI.ErrorRenderer(plainErr)
	resp, ok := renderer.(*groupsAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
}

func TestErrorInvalidRequest(t *testing.T) {
	testErr := errors.New("invalid input")
	renderer := groupsAPI.ErrorInvalidRequest(testErr)
	resp, ok := renderer.(*groupsAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "invalid input", resp.ErrorText)
}

func TestErrorForbidden(t *testing.T) {
	testErr := errors.New("access denied")
	renderer := groupsAPI.ErrorForbidden(testErr)
	resp, ok := renderer.(*groupsAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "access denied", resp.ErrorText)
}

func TestErrorNotFound(t *testing.T) {
	testErr := errors.New("not found")
	renderer := groupsAPI.ErrorNotFound(testErr)
	resp, ok := renderer.(*groupsAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "not found", resp.ErrorText)
}

func TestErrorConflict(t *testing.T) {
	testErr := errors.New("conflict")
	renderer := groupsAPI.ErrorConflict(testErr)
	resp, ok := renderer.(*groupsAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "conflict", resp.ErrorText)
}

func TestErrorInternalServer(t *testing.T) {
	testErr := errors.New("internal error")
	renderer := groupsAPI.ErrorInternalServer(testErr)
	resp, ok := renderer.(*groupsAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "internal error", resp.ErrorText)
}

func TestErrorResponse_Render(t *testing.T) {
	errResp := &groupsAPI.ErrorResponse{
		Err:            errors.New("test error"),
		HTTPStatusCode: http.StatusNotFound,
		Status:         "error",
		ErrorText:      "test error",
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := errResp.Render(w, r)
	assert.NoError(t, err)
}
