package activities_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	activitiesAPI "github.com/moto-nrw/project-phoenix/api/activities"
	"github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer_NotFoundErrors(t *testing.T) {
	tests := []struct {
		name    string
		baseErr error
	}{
		{"ErrCategoryNotFound", activities.ErrCategoryNotFound},
		{"ErrGroupNotFound", activities.ErrGroupNotFound},
		{"ErrScheduleNotFound", activities.ErrScheduleNotFound},
		{"ErrSupervisorNotFound", activities.ErrSupervisorNotFound},
		{"ErrEnrollmentNotFound", activities.ErrEnrollmentNotFound},
		{"ErrNotEnrolled", activities.ErrNotEnrolled},
		{"ErrStaffNotFound", activities.ErrStaffNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actErr := &activities.ActivityError{Err: tt.baseErr}
			renderer := activitiesAPI.ErrorRenderer(actErr)
			resp, ok := renderer.(*activitiesAPI.ErrorResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
			assert.Equal(t, "error", resp.Status)
		})
	}
}

func TestErrorRenderer_ConflictErrors(t *testing.T) {
	tests := []struct {
		name    string
		baseErr error
	}{
		{"ErrGroupFull", activities.ErrGroupFull},
		{"ErrAlreadyEnrolled", activities.ErrAlreadyEnrolled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actErr := &activities.ActivityError{Err: tt.baseErr}
			renderer := activitiesAPI.ErrorRenderer(actErr)
			resp, ok := renderer.(*activitiesAPI.ErrorResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
			assert.Equal(t, "error", resp.Status)
		})
	}
}

func TestErrorRenderer_ForbiddenErrors(t *testing.T) {
	actErr := &activities.ActivityError{Err: activities.ErrGroupClosed}
	renderer := activitiesAPI.ErrorRenderer(actErr)
	resp, ok := renderer.(*activitiesAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
}

func TestErrorRenderer_BadRequestErrors(t *testing.T) {
	actErr := &activities.ActivityError{Err: activities.ErrInvalidAttendanceStatus}
	renderer := activitiesAPI.ErrorRenderer(actErr)
	resp, ok := renderer.(*activitiesAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
}

func TestErrorRenderer_UnknownActivityError(t *testing.T) {
	actErr := &activities.ActivityError{Err: errors.New("unknown error")}
	renderer := activitiesAPI.ErrorRenderer(actErr)
	resp, ok := renderer.(*activitiesAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
}

func TestErrorRenderer_NonActivityError(t *testing.T) {
	plainErr := errors.New("generic error")
	renderer := activitiesAPI.ErrorRenderer(plainErr)
	resp, ok := renderer.(*activitiesAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
}

func TestErrorInvalidRequest(t *testing.T) {
	testErr := errors.New("invalid input")
	renderer := activitiesAPI.ErrorInvalidRequest(testErr)
	resp, ok := renderer.(*activitiesAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "invalid input", resp.ErrorText)
}

func TestErrorForbidden(t *testing.T) {
	testErr := errors.New("access denied")
	renderer := activitiesAPI.ErrorForbidden(testErr)
	resp, ok := renderer.(*activitiesAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "access denied", resp.ErrorText)
}

func TestErrorNotFound(t *testing.T) {
	testErr := errors.New("not found")
	renderer := activitiesAPI.ErrorNotFound(testErr)
	resp, ok := renderer.(*activitiesAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "not found", resp.ErrorText)
}

func TestErrorConflict(t *testing.T) {
	testErr := errors.New("conflict")
	renderer := activitiesAPI.ErrorConflict(testErr)
	resp, ok := renderer.(*activitiesAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "conflict", resp.ErrorText)
}

func TestErrorInternalServer(t *testing.T) {
	testErr := errors.New("internal error")
	renderer := activitiesAPI.ErrorInternalServer(testErr)
	resp, ok := renderer.(*activitiesAPI.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "internal error", resp.ErrorText)
}

func TestErrorResponse_Render(t *testing.T) {
	errResp := &activitiesAPI.ErrorResponse{
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
