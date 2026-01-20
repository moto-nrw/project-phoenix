package activities

import (
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name:           "ErrStaffNotFound returns 404",
			err:            &activities.ActivityError{Op: "test", Err: activities.ErrStaffNotFound},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "ErrCategoryNotFound returns 404",
			err:            &activities.ActivityError{Op: "test", Err: activities.ErrCategoryNotFound},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "ErrGroupNotFound returns 404",
			err:            &activities.ActivityError{Op: "test", Err: activities.ErrGroupNotFound},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "ErrSupervisorNotFound returns 404",
			err:            &activities.ActivityError{Op: "test", Err: activities.ErrSupervisorNotFound},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "ErrGroupFull returns 409",
			err:            &activities.ActivityError{Op: "test", Err: activities.ErrGroupFull},
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "ErrAlreadyEnrolled returns 409",
			err:            &activities.ActivityError{Op: "test", Err: activities.ErrAlreadyEnrolled},
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "ErrGroupClosed returns 403",
			err:            &activities.ActivityError{Op: "test", Err: activities.ErrGroupClosed},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "ErrInvalidAttendanceStatus returns 400",
			err:            &activities.ActivityError{Op: "test", Err: activities.ErrInvalidAttendanceStatus},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unknown ActivityError returns 500",
			err:            &activities.ActivityError{Op: "test", Err: nil},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "non-ActivityError returns 500",
			err:            assert.AnError,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := ErrorRenderer(tt.err)
			errResp, ok := renderer.(*ErrorResponse)
			assert.True(t, ok, "Expected ErrorResponse type")
			assert.Equal(t, tt.expectedStatus, errResp.HTTPStatusCode)
		})
	}
}
