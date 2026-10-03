package schoolstructurehttp_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	schoolstructurehttp "github.com/moto-nrw/project-phoenix/modules/schoolstructure/http"
)

// renderedError is the error body the shared renderer writes.
type renderedError struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// renderError renders err the way the routes do and returns the status code
// and the decoded body.
func renderError(t *testing.T, err error) (int, renderedError) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/groups", nil)
	require.NoError(t, render.Render(recorder, request, schoolstructurehttp.ErrorRenderer(err)))
	var body renderedError
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return recorder.Code, body
}

func TestErrorRenderer_NotFoundErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseErr error
	}{
		{"ErrGroupNotFound", testutil.EducationSuiteErrGroupNotFound},
		{"ErrGroupTeacherNotFound", testutil.EducationSuiteErrGroupTeacherNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := renderError(t, &testutil.EducationSuiteError{Err: tt.baseErr})
			assert.Equal(t, http.StatusNotFound, status)
			assert.Equal(t, "error", body.Status)
		})
	}
}

func TestErrorRenderer_ConflictErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseErr error
		wantMsg string
	}{
		{"ErrDuplicateGroup", testutil.EducationSuiteErrDuplicateGroup, "Eine Gruppe mit diesem Namen"},
		{"ErrGroupHasStudents", testutil.EducationSuiteErrGroupHasStudents, ""},
		{"ErrGroupHasHandover", testutil.EducationSuiteErrGroupHasHandover, "Übergabe muss zuerst beendet werden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := renderError(t, &testutil.EducationSuiteError{Op: "DeleteGroup", Err: tt.baseErr})
			assert.Equal(t, http.StatusConflict, status)
			assert.Equal(t, "error", body.Status)
			if tt.wantMsg != "" {
				assert.Contains(t, body.Error, tt.wantMsg)
			}
			assert.NotContains(t, body.Error, "education:",
				"renderer must surface the inner sentinel, not the EducationError wrapper prefix")
		})
	}
}

func TestErrorRenderer_InvalidRequestErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseErr error
	}{
		{"ErrRoomNotFound", testutil.EducationSuiteErrRoomNotFound},
		{"ErrTeacherNotFound", testutil.EducationSuiteErrTeacherNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := renderError(t, &testutil.EducationSuiteError{Err: tt.baseErr})
			assert.Equal(t, http.StatusBadRequest, status)
			assert.Equal(t, "error", body.Status)
		})
	}
}

func TestErrorRenderer_UnknownEducationError(t *testing.T) {
	t.Parallel()

	status, body := renderError(t, &testutil.EducationSuiteError{Err: errors.New("unknown error")})
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "error", body.Status)
}

func TestErrorRenderer_NonEducationError(t *testing.T) {
	t.Parallel()

	status, body := renderError(t, errors.New("generic error"))
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "error", body.Status)
}
