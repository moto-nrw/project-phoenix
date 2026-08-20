package activities_test

import (
	"errors"
	"net/http"
	"testing"

	activitiesAPI "github.com/moto-nrw/project-phoenix/api/activities"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer_NotFoundErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseErr error
	}{
		{"ErrCategoryNotFound", activities.ErrCategoryNotFound},
		{"ErrGroupNotFound", activities.ErrGroupNotFound},
		{"ErrScheduleNotFound", activities.ErrScheduleNotFound},
		{"ErrSupervisorNotFound", activities.ErrSupervisorNotFound},
		{"ErrEnrollmentNotFound", activities.ErrEnrollmentNotFound},
		{"ErrStudentNotFound", activities.ErrStudentNotFound},
		{"ErrNotEnrolled", activities.ErrNotEnrolled},
		{"ErrStaffNotFound", activities.ErrStaffNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actErr := &activities.ActivityError{Err: tt.baseErr}
			renderer := activitiesAPI.ErrorRenderer(actErr)
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
			assert.Equal(t, "error", resp.Status)
		})
	}
}

func TestErrorRenderer_ConflictErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseErr error
	}{
		{"ErrGroupFull", activities.ErrGroupFull},
		{"ErrAlreadyEnrolled", activities.ErrAlreadyEnrolled},
		{"ErrTimetableTemplateProtected", activities.ErrTimetableTemplateProtected},
		{"ErrOnlySupervisorRequiresReplacement", activities.ErrOnlySupervisorRequiresReplacement},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actErr := &activities.ActivityError{Err: tt.baseErr}
			renderer := activitiesAPI.ErrorRenderer(actErr)
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
			assert.Equal(t, "error", resp.Status)
			if tt.baseErr == activities.ErrOnlySupervisorRequiresReplacement {
				assert.Equal(t, "ONLY_SUPERVISOR_REPLACEMENT_REQUIRED", resp.Code)
			}
		})
	}
}

func TestErrorRenderer_ForbiddenErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseErr error
	}{
		{"ErrGroupClosed", activities.ErrGroupClosed},
		{"ErrSystemActivityProtected", activities.ErrSystemActivityProtected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actErr := &activities.ActivityError{Err: tt.baseErr}
			renderer := activitiesAPI.ErrorRenderer(actErr)
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
			assert.Equal(t, "error", resp.Status)
		})
	}
}

func TestErrorRenderer_BadRequestErrors(t *testing.T) {
	t.Parallel()

	actErr := &activities.ActivityError{Err: activities.ErrInvalidAttendanceStatus}
	renderer := activitiesAPI.ErrorRenderer(actErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
}

func TestErrorRenderer_UnknownActivityError(t *testing.T) {
	t.Parallel()

	actErr := &activities.ActivityError{Err: errors.New("unknown error")}
	renderer := activitiesAPI.ErrorRenderer(actErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
}

func TestErrorRenderer_NonActivityError(t *testing.T) {
	t.Parallel()

	plainErr := errors.New("generic error")
	renderer := activitiesAPI.ErrorRenderer(plainErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
}
