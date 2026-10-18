package httpadapter_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	activitiesAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http/activities"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer_NotFoundErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseErr error
	}{
		{"ErrCategoryNotFound", activityContract.ErrCategoryNotFound},
		{"ErrGroupNotFound", activityContract.ErrGroupNotFound},
		{"ErrScheduleNotFound", activityContract.ErrScheduleNotFound},
		{"ErrSupervisorNotFound", activityContract.ErrSupervisorNotFound},
		{"ErrEnrollmentNotFound", activityContract.ErrEnrollmentNotFound},
		{"ErrStudentNotFound", activityContract.ErrStudentNotFound},
		{"ErrNotEnrolled", activityContract.ErrNotEnrolled},
		{"ErrStaffNotFound", activityContract.ErrStaffNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actErr := &activityContract.ActivityError{Err: tt.baseErr}
			renderer := activitiesAPI.ErrorRenderer(actErr)
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
			assertEnvelope(t, resp, "general.input")
		})
	}
}

func TestErrorRenderer_ConflictErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseErr error
		code    string
	}{
		{"ErrGroupFull", activityContract.ErrGroupFull, "general.business_rejection"},
		{"ErrAlreadyEnrolled", activityContract.ErrAlreadyEnrolled, "general.business_rejection"},
		{"ErrTimetableTemplateProtected", activityContract.ErrTimetableTemplateProtected, "timetable.activity_template_protected"},
		{"ErrOnlySupervisorRequiresReplacement", activityContract.ErrOnlySupervisorRequiresReplacement, "timetable.only_supervisor_replacement_required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actErr := &activityContract.ActivityError{Err: tt.baseErr}
			renderer := activitiesAPI.ErrorRenderer(actErr)
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
			assertEnvelope(t, resp, tt.code)
			if tt.baseErr == activityContract.ErrOnlySupervisorRequiresReplacement {
				assert.Equal(t, "timetable.only_supervisor_replacement_required", resp.Code)
			}
		})
	}
}

func TestErrorRenderer_ForbiddenErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseErr error
		code    string
	}{
		{"ErrGroupClosed", activityContract.ErrGroupClosed, "general.permission"},
		{"ErrSystemActivityProtected", activityContract.ErrSystemActivityProtected, "timetable.activity_system_protected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actErr := &activityContract.ActivityError{Err: tt.baseErr}
			renderer := activitiesAPI.ErrorRenderer(actErr)
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
			assertEnvelope(t, resp, tt.code)
		})
	}
}

func TestErrorRenderer_BadRequestErrors(t *testing.T) {
	t.Parallel()

	actErr := &activityContract.ActivityError{Err: activityContract.ErrInvalidAttendanceStatus}
	renderer := activitiesAPI.ErrorRenderer(actErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	assertEnvelope(t, resp, "general.input")
}

func TestErrorRenderer_UnknownActivityError(t *testing.T) {
	t.Parallel()

	actErr := &activityContract.ActivityError{Err: errors.New("unknown error")}
	renderer := activitiesAPI.ErrorRenderer(actErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assertEnvelope(t, resp, "general.server")
}

func TestErrorRenderer_NonActivityError(t *testing.T) {
	t.Parallel()

	plainErr := errors.New("generic error")
	renderer := activitiesAPI.ErrorRenderer(plainErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assertEnvelope(t, resp, "general.server")
}

// assertEnvelope renders resp and checks the wire envelope: the status
// member is the HTTP status number and the code is the expected one.
func assertEnvelope(t *testing.T, resp *common.ErrResponse, wantCode string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	require.NoError(t, render.Render(recorder, httptest.NewRequest(http.MethodGet, "/", nil), resp))
	var body struct {
		Status int    `json:"status"`
		Code   string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, resp.HTTPStatusCode, body.Status)
	assert.Equal(t, wantCode, body.Code)
}
