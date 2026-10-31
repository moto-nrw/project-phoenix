package presence_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer_NotFoundErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"ErrActiveGroupNotFound", studentpresence.ErrGroupNotFound},
		{"ErrVisitNotFound", studentpresence.ErrVisitNotFound},
		{"ErrGroupSupervisorNotFound", studentpresence.ErrGroupSupervisorNotFound},
		{"ErrCombinedGroupNotFound", studentpresence.ErrCombinedGroupNotFound},
		{"ErrGroupMappingNotFound", studentpresence.ErrGroupMappingNotFound},
		{"ErrStudentNotFound", studentpresence.ErrStudentNotFound},
		{"ErrStaffNotFound", studentpresence.ErrStaffNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := presence.ErrorRenderer(tt.err)
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, testutil.StatusNotFound, resp.HTTPStatusCode)
			assertEnvelope(t, resp, "general.input")
		})
	}
}

func TestErrorRenderer_BadRequestErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"ErrInvalidData", studentpresence.ErrInvalidData},
		{"ErrActiveGroupAlreadyEnded", studentpresence.ErrGroupAlreadyEnded},
		{"ErrVisitAlreadyEnded", studentpresence.ErrVisitAlreadyEnded},
		{"ErrSupervisionAlreadyEnded", studentpresence.ErrSupervisionAlreadyEnded},
		{"ErrCombinedGroupAlreadyEnded", studentpresence.ErrCombinedGroupAlreadyEnded},
		{"ErrGroupAlreadyInCombination", studentpresence.ErrGroupAlreadyInCombination},
		{"ErrStudentAlreadyInGroup", studentpresence.ErrStudentAlreadyInGroup},
		// ErrStudentAlreadyActive moved out of the 400 list — see
		// TestErrorRenderer_ConflictError. It now maps to 409 Conflict
		// because Issue #844 routes the DB-level duplicate-active-visit
		// translation through this renderer for non-IoT callers.
		{"ErrStaffAlreadySupervising", studentpresence.ErrStaffAlreadySupervising},
		{"ErrCannotDeleteActiveGroup", studentpresence.ErrCannotDeleteActiveGroup},
		{"ErrInvalidTimeRange", studentpresence.ErrInvalidTimeRange},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := presence.ErrorRenderer(tt.err)
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, testutil.StatusBadRequest, resp.HTTPStatusCode)
			assertEnvelope(t, resp, "general.input")
		})
	}
}

func TestErrorRenderer_ConflictError(t *testing.T) {
	t.Parallel()

	renderer := presence.ErrorRenderer(studentpresence.ErrRoomConflict)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, testutil.StatusConflict, resp.HTTPStatusCode)
	assertEnvelope(t, resp, "general.business_rejection")
}

func TestErrorRenderer_RoomCapacityConflict(t *testing.T) {
	t.Parallel()

	renderer := presence.ErrorRenderer(&studentpresence.RoomCapacityError{
		RoomID:           12,
		RoomName:         "Mensa",
		CurrentOccupancy: 43,
		MaxCapacity:      43,
	})
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, testutil.StatusConflict, resp.HTTPStatusCode)
	assertEnvelope(t, resp, "presence.room_capacity_exceeded")
}

// TestErrorRenderer_StudentAlreadyActiveConflict guards the Issue #844
// review fix: ErrStudentAlreadyActive must map to 409 Conflict, not
// 400 Bad Request. The active service's CreateVisit now translates
// the DB unique-index violation to this sentinel for every caller —
// including admin POST /active/visits — so a 400 here would
// contradict the IoT path's 409 response on the same conflict.
func TestErrorRenderer_StudentAlreadyActiveConflict(t *testing.T) {
	t.Parallel()

	renderer := presence.ErrorRenderer(studentpresence.ErrStudentAlreadyActive)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, testutil.StatusConflict, resp.HTTPStatusCode)
	assertEnvelope(t, resp, "general.business_rejection")
}

func TestErrorRenderer_StudentsNotPresentConflict(t *testing.T) {
	t.Parallel()

	renderer := presence.ErrorRenderer(studentpresence.ErrStudentsNotPresent)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, testutil.StatusConflict, resp.HTTPStatusCode)
	assertEnvelope(t, resp, "general.business_rejection")
}

func TestErrorRenderer_StudentMoveForbidden(t *testing.T) {
	t.Parallel()

	renderer := presence.ErrorRenderer(studentpresence.ErrStudentMoveForbidden)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, testutil.StatusForbidden, resp.HTTPStatusCode)
	assertEnvelope(t, resp, "general.permission")
}

func TestErrorRenderer_UnknownError(t *testing.T) {
	t.Parallel()

	unknownErr := errors.New("unknown error")
	renderer := presence.ErrorRenderer(unknownErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, testutil.StatusInternalServerError, resp.HTTPStatusCode)
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

func TestErrorRenderer_ChangedVisitCohortIsAConflict(t *testing.T) {
	t.Parallel()
	renderer := presence.ErrorRenderer(studentpresence.ErrVisitWriteConflict)
	response, ok := renderer.(*common.ErrResponse)
	require.True(t, ok)
	require.Equal(t, http.StatusConflict, response.HTTPStatusCode)
	assertEnvelope(t, response, "general.business_rejection")
}
