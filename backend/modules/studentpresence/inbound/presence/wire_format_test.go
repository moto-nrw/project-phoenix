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
// Issue #2503 intentionally adds problem fields. These expectations keep the
// legacy status/error values and exact bytes pinned alongside the new fields.
package presence_test

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
	"github.com/stretchr/testify/assert"
)

func renderWire(t *testing.T, handler testutil.HandlerFunc) (int, string) {
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
			wantBody:   "{\"status\":\"Room Conflict\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-vorgang-nicht-moeglich\",\"title\":\"Conflict\",\"detail\":\"room is already occupied by another active group\",\"instance\":\"\",\"error\":\"room is already occupied by another active group\",\"code\":\"general.business_rejection\"}\n",
		},
		{
			name:       "ErrActiveGroupNotFound",
			err:        studentpresence.ErrGroupNotFound,
			wantStatus: 404,
			wantBody:   "{\"status\":\"Active Group Not Found\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen\",\"title\":\"Not Found\",\"detail\":\"active group not found\",\"instance\":\"\",\"error\":\"active group not found\",\"code\":\"general.input\"}\n",
		},
		{
			name:       "ErrStudentAlreadyActive",
			err:        studentpresence.ErrStudentAlreadyActive,
			wantStatus: 409,
			wantBody:   "{\"status\":\"Student Already Has Active Visit\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-vorgang-nicht-moeglich\",\"title\":\"Conflict\",\"detail\":\"student already has an active visit\",\"instance\":\"\",\"error\":\"student already has an active visit\",\"code\":\"general.business_rejection\"}\n",
		},
		{
			name:       "ErrStudentMoveForbidden",
			err:        studentpresence.ErrStudentMoveForbidden,
			wantStatus: 403,
			wantBody:   "{\"status\":\"Forbidden\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-zugriff-pruefen\",\"title\":\"Forbidden\",\"detail\":\"not authorized to move the selected students\",\"instance\":\"\",\"error\":\"not authorized to move the selected students\",\"code\":\"general.permission\"}\n",
		},
		{
			name:       "unmapped error defaults to 500",
			err:        errors.New("boom"),
			wantStatus: 500,
			wantBody:   "{\"status\":\"Internal Server Error\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-unerwarteter-fehler\",\"title\":\"Internal Server Error\",\"detail\":\"boom\",\"instance\":\"\",\"error\":\"boom\",\"code\":\"general.server\"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotBody := renderWire(t, func(w testutil.ResponseWriter, r *testutil.Request) {
				common.RenderError(w, r, presence.ErrorRenderer(tt.err))
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
		handler    testutil.HandlerFunc
		wantStatus int
		wantBody   string
	}{
		{
			name: "ErrorInvalidRequest",
			handler: func(w testutil.ResponseWriter, r *testutil.Request) {
				common.RenderError(w, r, presence.ErrorInvalidRequest(errors.New("bad")))
			},
			wantStatus: 400,
			wantBody:   "{\"status\":\"Invalid Request\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen\",\"title\":\"Bad Request\",\"detail\":\"bad\",\"instance\":\"\",\"error\":\"bad\",\"code\":\"general.input\"}\n",
		},
		{
			name: "ErrorForbidden",
			handler: func(w testutil.ResponseWriter, r *testutil.Request) {
				common.RenderError(w, r, presence.ErrorForbidden(errors.New("nope")))
			},
			wantStatus: 403,
			wantBody:   "{\"status\":\"Forbidden\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-zugriff-pruefen\",\"title\":\"Forbidden\",\"detail\":\"nope\",\"instance\":\"\",\"error\":\"nope\",\"code\":\"general.permission\"}\n",
		},
		{
			name: "ErrorUnauthorized",
			handler: func(w testutil.ResponseWriter, r *testutil.Request) {
				common.RenderError(w, r, presence.ErrorUnauthorized(errors.New("nope")))
			},
			wantStatus: 401,
			wantBody:   "{\"status\":\"Unauthorized\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-zugriff-pruefen\",\"title\":\"Unauthorized\",\"detail\":\"nope\",\"instance\":\"\",\"error\":\"nope\",\"code\":\"general.permission\"}\n",
		},
		{
			name: "ErrorInternalServer",
			handler: func(w testutil.ResponseWriter, r *testutil.Request) {
				common.RenderError(w, r, presence.ErrorInternalServer(errors.New("boom2")))
			},
			wantStatus: 500,
			wantBody:   "{\"status\":\"Internal Server Error\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-unerwarteter-fehler\",\"title\":\"Internal Server Error\",\"detail\":\"boom2\",\"instance\":\"\",\"error\":\"boom2\",\"code\":\"general.server\"}\n",
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
