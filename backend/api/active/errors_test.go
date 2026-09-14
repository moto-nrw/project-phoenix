package active_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
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
			renderer := active.ErrorRenderer(tt.err)
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
			assert.NotEmpty(t, resp.Status)
			assert.NotEmpty(t, resp.ErrorText)
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
			renderer := active.ErrorRenderer(tt.err)
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
			assert.NotEmpty(t, resp.Status)
			assert.NotEmpty(t, resp.ErrorText)
		})
	}
}

func TestErrorRenderer_ConflictError(t *testing.T) {
	t.Parallel()

	renderer := active.ErrorRenderer(studentpresence.ErrRoomConflict)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, "Room Conflict", resp.Status)
}

func TestErrorRenderer_RoomCapacityConflict(t *testing.T) {
	t.Parallel()

	renderer := active.ErrorRenderer(&studentpresence.RoomCapacityError{
		RoomID:           12,
		RoomName:         "Mensa",
		CurrentOccupancy: 43,
		MaxCapacity:      43,
	})
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, "Room Capacity Exceeded", resp.Status)
}

// TestErrorRenderer_StudentAlreadyActiveConflict guards the Issue #844
// review fix: ErrStudentAlreadyActive must map to 409 Conflict, not
// 400 Bad Request. The active service's CreateVisit now translates
// the DB unique-index violation to this sentinel for every caller —
// including admin POST /active/visits — so a 400 here would
// contradict the IoT path's 409 response on the same conflict.
func TestErrorRenderer_StudentAlreadyActiveConflict(t *testing.T) {
	t.Parallel()

	renderer := active.ErrorRenderer(studentpresence.ErrStudentAlreadyActive)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, "Student Already Has Active Visit", resp.Status)
}

func TestErrorRenderer_StudentsNotPresentConflict(t *testing.T) {
	t.Parallel()

	renderer := active.ErrorRenderer(studentpresence.ErrStudentsNotPresent)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, "Students Not Present", resp.Status)
}

func TestErrorRenderer_StudentMoveForbidden(t *testing.T) {
	t.Parallel()

	renderer := active.ErrorRenderer(studentpresence.ErrStudentMoveForbidden)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
	assert.Equal(t, "Forbidden", resp.Status)
}

func TestErrorRenderer_UnknownError(t *testing.T) {
	t.Parallel()

	unknownErr := errors.New("unknown error")
	renderer := active.ErrorRenderer(unknownErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "Internal Server Error", resp.Status)
}
