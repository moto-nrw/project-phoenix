package active

import (
	"net/http"
	"testing"

	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedText   string
	}{
		{
			name:           "ErrStudentNotFound returns 404",
			err:            &activeSvc.ActiveError{Op: "test", Err: activeSvc.ErrStudentNotFound},
			expectedStatus: http.StatusNotFound,
			expectedText:   "Student Not Found",
		},
		{
			name:           "ErrStaffNotFound returns 404",
			err:            &activeSvc.ActiveError{Op: "test", Err: activeSvc.ErrStaffNotFound},
			expectedStatus: http.StatusNotFound,
			expectedText:   "Staff Not Found",
		},
		{
			name:           "ErrActiveGroupNotFound returns 404",
			err:            &activeSvc.ActiveError{Op: "test", Err: activeSvc.ErrActiveGroupNotFound},
			expectedStatus: http.StatusNotFound,
			expectedText:   "Active Group Not Found",
		},
		{
			name:           "ErrVisitNotFound returns 404",
			err:            &activeSvc.ActiveError{Op: "test", Err: activeSvc.ErrVisitNotFound},
			expectedStatus: http.StatusNotFound,
			expectedText:   "Visit Not Found",
		},
		{
			name:           "ErrInvalidData returns 400",
			err:            &activeSvc.ActiveError{Op: "test", Err: activeSvc.ErrInvalidData},
			expectedStatus: http.StatusBadRequest,
			expectedText:   "Invalid Data",
		},
		{
			name:           "ErrRoomConflict returns 409",
			err:            &activeSvc.ActiveError{Op: "test", Err: activeSvc.ErrRoomConflict},
			expectedStatus: http.StatusConflict,
			expectedText:   "Room Conflict",
		},
		{
			name:           "unknown error returns 500",
			err:            &activeSvc.ActiveError{Op: "test", Err: activeSvc.ErrDatabaseOperation},
			expectedStatus: http.StatusInternalServerError,
			expectedText:   "Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := ErrorRenderer(tt.err)
			errResp, ok := renderer.(*ErrResponse)
			assert.True(t, ok, "Expected ErrResponse type")
			assert.Equal(t, tt.expectedStatus, errResp.HTTPStatusCode)
			assert.Equal(t, tt.expectedText, errResp.StatusText)
		})
	}
}
