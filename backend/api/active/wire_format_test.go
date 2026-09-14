// Package active_test pins the exact wire format (HTTP status code + raw
// JSON response bytes) produced by api/active's hand-rolled ErrResponse
// renderer.
//
// These are B0 wire-format golden tests for issue #575 (API layer
// technical-debt / error-response consolidation). The upcoming B1 refactor
// collapses the per-package ErrResponse/ErrorRenderer duplication (active,
// feedback, ...) into shared helpers in api/common. That
// refactor MUST NOT change a single byte of what a client currently
// receives on the wire. This file is the oracle: it renders through the
// production common.RenderError pipeline (go-chi/render's json.NewEncoder, which
// HTML-escapes, emits compact JSON, and appends a trailing newline) and
// asserts the literal body string.
//
// Changing an expectation in this file requires proof that the wire format
// change is intentional (e.g. a linked B1 commit updating PyrePortal's
// error-string mapping in lockstep) — not just "the refactor made the test
// fail."
package active_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
)

func renderWire(t *testing.T, handler http.HandlerFunc) (int, string) {
	t.Helper()
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w.Code, w.Body.String()
}

func TestWireFormat_Active_ErrorRenderer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "ErrRoomConflict",
			err:        studentpresence.ErrRoomConflict,
			wantStatus: 409,
			wantBody:   "{\"status\":\"Room Conflict\",\"error\":\"room is already occupied by another active group\"}\n",
		},
		{
			name:       "ErrActiveGroupNotFound",
			err:        studentpresence.ErrGroupNotFound,
			wantStatus: 404,
			wantBody:   "{\"status\":\"Active Group Not Found\",\"error\":\"active group not found\"}\n",
		},
		{
			name:       "ErrStudentAlreadyActive",
			err:        studentpresence.ErrStudentAlreadyActive,
			wantStatus: 409,
			wantBody:   "{\"status\":\"Student Already Has Active Visit\",\"error\":\"student already has an active visit\"}\n",
		},
		{
			name:       "ErrStudentMoveForbidden",
			err:        studentpresence.ErrStudentMoveForbidden,
			wantStatus: 403,
			wantBody:   "{\"status\":\"Forbidden\",\"error\":\"not authorized to move the selected students\"}\n",
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
			gotStatus, gotBody := renderWire(t, func(w http.ResponseWriter, r *http.Request) {
				common.RenderError(w, r, active.ErrorRenderer(tt.err))
			})
			assert.Equal(t, tt.wantStatus, gotStatus)
			assert.Equal(t, tt.wantBody, gotBody)
		})
	}
}

func TestWireFormat_Active_ErrorHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
		wantBody   string
	}{
		{
			name: "ErrorInvalidRequest",
			handler: func(w http.ResponseWriter, r *http.Request) {
				common.RenderError(w, r, active.ErrorInvalidRequest(errors.New("bad")))
			},
			wantStatus: 400,
			wantBody:   "{\"status\":\"Invalid Request\",\"error\":\"bad\"}\n",
		},
		{
			name: "ErrorForbidden",
			handler: func(w http.ResponseWriter, r *http.Request) {
				common.RenderError(w, r, active.ErrorForbidden(errors.New("nope")))
			},
			wantStatus: 403,
			wantBody:   "{\"status\":\"Forbidden\",\"error\":\"nope\"}\n",
		},
		{
			name: "ErrorUnauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				common.RenderError(w, r, active.ErrorUnauthorized(errors.New("nope")))
			},
			wantStatus: 401,
			wantBody:   "{\"status\":\"Unauthorized\",\"error\":\"nope\"}\n",
		},
		{
			name: "ErrorInternalServer",
			handler: func(w http.ResponseWriter, r *http.Request) {
				common.RenderError(w, r, active.ErrorInternalServer(errors.New("boom2")))
			},
			wantStatus: 500,
			wantBody:   "{\"status\":\"Internal Server Error\",\"error\":\"boom2\"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotBody := renderWire(t, tt.handler)
			assert.Equal(t, tt.wantStatus, gotStatus)
			assert.Equal(t, tt.wantBody, gotBody)
		})
	}
}
