package groups_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
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
			resp, ok := renderer.(*common.ErrResponse)
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
		wantMsg string
	}{
		{"ErrDuplicateGroup", education.ErrDuplicateGroup, "Eine Gruppe mit diesem Namen"},
		{"ErrGroupHasStudents", education.ErrGroupHasStudents, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eduErr := &education.EducationError{Op: "DeleteGroup", Err: tt.baseErr}
			renderer := groupsAPI.ErrorRenderer(eduErr)
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
			assert.Equal(t, "error", resp.Status)
			if tt.wantMsg != "" {
				assert.Contains(t, resp.ErrorText, tt.wantMsg)
			}
			assert.NotContains(t, resp.ErrorText, "education:",
				"renderer must surface the inner sentinel, not the EducationError wrapper prefix")
		})
	}
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
			resp, ok := renderer.(*common.ErrResponse)
			assert.True(t, ok)
			assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
			assert.Equal(t, "error", resp.Status)
		})
	}
}

func TestErrorRenderer_UnknownEducationError(t *testing.T) {
	eduErr := &education.EducationError{Err: errors.New("unknown error")}
	renderer := groupsAPI.ErrorRenderer(eduErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
}

func TestErrorRenderer_NonEducationError(t *testing.T) {
	plainErr := errors.New("generic error")
	renderer := groupsAPI.ErrorRenderer(plainErr)
	resp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
}
