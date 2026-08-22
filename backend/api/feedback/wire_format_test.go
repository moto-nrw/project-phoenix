// Package feedback_test pins the exact wire format (HTTP status code + raw
// JSON response bytes) produced by api/feedback's hand-rolled ErrResponse
// renderer.
//
// These are B0 wire-format golden tests for issue #575 (API layer
// technical-debt / error-response consolidation). The upcoming B1 refactor
// collapses the per-package ErrResponse/ErrorRenderer duplication (active,
// feedback, ...) into shared helpers in api/common. That
// refactor MUST NOT change a single byte of what a client currently
// receives on the wire. This file is the oracle: it renders through the
// real render.Render(...) pipeline (go-chi/render's json.NewEncoder, which
// HTML-escapes, emits compact JSON, and appends a trailing newline) and
// asserts the literal body string.
//
// Changing an expectation in this file requires proof that the wire format
// change is intentional — not just "the refactor made the test fail."
package feedback_test

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/feedback"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	"github.com/stretchr/testify/assert"
)

func renderWire(t *testing.T, renderer render.Renderer) (int, string) {
	t.Helper()
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	err := render.Render(w, r, renderer)
	assert.NoError(t, err)
	return w.Code, w.Body.String()
}

func TestWireFormat_Feedback_ErrorRenderer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "ErrEntryNotFound",
			err:        feedbackSvc.ErrEntryNotFound,
			wantStatus: 404,
			wantBody:   "{\"status\":\"Resource Not Found\",\"error\":\"feedback entry not found\"}\n",
		},
		{
			name:       "ErrInvalidEntryData",
			err:        feedbackSvc.ErrInvalidEntryData,
			wantStatus: 400,
			wantBody:   "{\"status\":\"Invalid Feedback Data\",\"error\":\"invalid feedback entry data\"}\n",
		},
		{
			name:       "ErrInvalidDateRange",
			err:        feedbackSvc.ErrInvalidDateRange,
			wantStatus: 400,
			wantBody:   "{\"status\":\"Invalid Date Range\",\"error\":\"invalid date range\"}\n",
		},
		{
			name:       "ErrStudentNotFound",
			err:        feedbackSvc.ErrStudentNotFound,
			wantStatus: 404,
			wantBody:   "{\"status\":\"Student Not Found\",\"error\":\"student not found\"}\n",
		},
		{
			name:       "unmapped error defaults to 500",
			err:        errors.New("boom"),
			wantStatus: 500,
			wantBody:   "{\"status\":\"Internal Server Error\",\"error\":\"boom\"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := feedback.ErrorRenderer(tt.err)
			gotStatus, gotBody := renderWire(t, renderer)
			assert.Equal(t, tt.wantStatus, gotStatus)
			assert.Equal(t, tt.wantBody, gotBody)
		})
	}
}

func TestWireFormat_Feedback_ErrorHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		renderer   render.Renderer
		wantStatus int
		wantBody   string
	}{
		{
			name:       "ErrorInvalidRequest",
			renderer:   feedback.ErrorInvalidRequest(errors.New("bad")),
			wantStatus: 400,
			wantBody:   "{\"status\":\"Invalid Request\",\"error\":\"bad\"}\n",
		},
		{
			name:       "ErrorInternalServer",
			renderer:   feedback.ErrorInternalServer(errors.New("boom2")),
			wantStatus: 500,
			wantBody:   "{\"status\":\"Internal Server Error\",\"error\":\"boom2\"}\n",
		},
		{
			name:       "ErrorForbidden",
			renderer:   feedback.ErrorForbidden(errors.New("nope")),
			wantStatus: 403,
			wantBody:   "{\"status\":\"Forbidden\",\"error\":\"nope\"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotBody := renderWire(t, tt.renderer)
			assert.Equal(t, tt.wantStatus, gotStatus)
			assert.Equal(t, tt.wantBody, gotBody)
		})
	}
}
