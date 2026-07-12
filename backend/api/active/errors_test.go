package active_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/api/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer_NotFoundErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrActiveGroupNotFound", activeSvc.ErrActiveGroupNotFound},
		{"ErrVisitNotFound", activeSvc.ErrVisitNotFound},
		{"ErrGroupSupervisorNotFound", activeSvc.ErrGroupSupervisorNotFound},
		{"ErrCombinedGroupNotFound", activeSvc.ErrCombinedGroupNotFound},
		{"ErrGroupMappingNotFound", activeSvc.ErrGroupMappingNotFound},
		{"ErrStudentNotFound", activeSvc.ErrStudentNotFound},
		{"ErrStaffNotFound", activeSvc.ErrStaffNotFound},
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
	tests := []struct {
		name string
		err  error
	}{
		{"ErrInvalidData", activeSvc.ErrInvalidData},
		{"ErrActiveGroupAlreadyEnded", activeSvc.ErrActiveGroupAlreadyEnded},
		{"ErrVisitAlreadyEnded", activeSvc.ErrVisitAlreadyEnded},
		{"ErrSupervisionAlreadyEnded", activeSvc.ErrSupervisionAlreadyEnded},
		{"ErrCombinedGroupAlreadyEnded", activeSvc.ErrCombinedGroupAlreadyEnded},
		{"ErrGroupAlreadyInCombination", activeSvc.ErrGroupAlreadyInCombination},
		{"ErrStudentAlreadyInGroup", activeSvc.ErrStudentAlreadyInGroup},
		// ErrStudentAlreadyActive moved out of the 400 list — see
		// TestErrorRenderer_ConflictError. It now maps to 409 Conflict
		// because Issue #844 routes the DB-level duplicate-active-visit
		// translation through this renderer for non-IoT callers.
		{"ErrStaffAlreadySupervising", activeSvc.ErrStaffAlreadySupervising},
		{"ErrCannotDeleteActiveGroup", activeSvc.ErrCannotDeleteActiveGroup},
		{"ErrInvalidTimeRange", activeSvc.ErrInvalidTimeRange},
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
	renderer := active.ErrorRenderer(activeSvc.ErrRoomConflict)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, "Room Conflict", resp.Status)
}

// TestErrorRenderer_StudentAlreadyActiveConflict guards the Issue #844
// review fix: ErrStudentAlreadyActive must map to 409 Conflict, not
// 400 Bad Request. The active service's CreateVisit now translates
// the DB unique-index violation to this sentinel for every caller —
// including admin POST /active/visits — so a 400 here would
// contradict the IoT path's 409 response on the same conflict.
func TestErrorRenderer_StudentAlreadyActiveConflict(t *testing.T) {
	renderer := active.ErrorRenderer(activeSvc.ErrStudentAlreadyActive)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, "Student Already Has Active Visit", resp.Status)
}

func TestErrorRenderer_StudentsNotPresentConflict(t *testing.T) {
	renderer := active.ErrorRenderer(activeSvc.ErrStudentsNotPresent)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, "Students Not Present", resp.Status)
}

func TestErrorRenderer_StudentMoveForbidden(t *testing.T) {
	renderer := active.ErrorRenderer(activeSvc.ErrStudentMoveForbidden)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
	assert.Equal(t, "Forbidden", resp.Status)
}

func TestErrorRenderer_UnknownError(t *testing.T) {
	unknownErr := errors.New("unknown error")
	renderer := active.ErrorRenderer(unknownErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "Internal Server Error", resp.Status)
}
