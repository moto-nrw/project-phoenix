package active_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/active"
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
			resp, ok := renderer.(*active.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
			assert.NotEmpty(t, resp.StatusText)
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
		{"ErrStudentAlreadyActive", activeSvc.ErrStudentAlreadyActive},
		{"ErrStaffAlreadySupervising", activeSvc.ErrStaffAlreadySupervising},
		{"ErrCannotDeleteActiveGroup", activeSvc.ErrCannotDeleteActiveGroup},
		{"ErrInvalidTimeRange", activeSvc.ErrInvalidTimeRange},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := active.ErrorRenderer(tt.err)
			resp, ok := renderer.(*active.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
			assert.NotEmpty(t, resp.StatusText)
			assert.NotEmpty(t, resp.ErrorText)
		})
	}
}

func TestErrorRenderer_ConflictError(t *testing.T) {
	renderer := active.ErrorRenderer(activeSvc.ErrRoomConflict)
	resp, ok := renderer.(*active.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, "Room Conflict", resp.StatusText)
}

func TestErrorRenderer_UnknownError(t *testing.T) {
	unknownErr := errors.New("unknown error")
	renderer := active.ErrorRenderer(unknownErr)
	resp, ok := renderer.(*active.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "Internal Server Error", resp.StatusText)
}

func TestErrorInvalidRequest(t *testing.T) {
	testErr := errors.New("invalid input")
	renderer := active.ErrorInvalidRequest(testErr)
	resp, ok := renderer.(*active.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	assert.Equal(t, "Invalid Request", resp.StatusText)
	assert.Equal(t, "invalid input", resp.ErrorText)
}

func TestErrorInternalServer(t *testing.T) {
	testErr := errors.New("database error")
	renderer := active.ErrorInternalServer(testErr)
	resp, ok := renderer.(*active.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "Internal Server Error", resp.StatusText)
	assert.Equal(t, "database error", resp.ErrorText)
}

func TestErrorForbidden(t *testing.T) {
	testErr := errors.New("access denied")
	renderer := active.ErrorForbidden(testErr)
	resp, ok := renderer.(*active.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
	assert.Equal(t, "Forbidden", resp.StatusText)
	assert.Equal(t, "access denied", resp.ErrorText)
}

func TestErrorUnauthorized(t *testing.T) {
	testErr := errors.New("not authenticated")
	renderer := active.ErrorUnauthorized(testErr)
	resp, ok := renderer.(*active.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, resp.HTTPStatusCode)
	assert.Equal(t, "Unauthorized", resp.StatusText)
	assert.Equal(t, "not authenticated", resp.ErrorText)
}

func TestErrResponse_Render(t *testing.T) {
	errResp := &active.ErrResponse{
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
