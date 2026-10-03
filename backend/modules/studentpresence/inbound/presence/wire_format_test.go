// Package presence_test pins the exact wire format (HTTP status code + raw
// JSON response bytes) of the presence error rules, rendered through the
// production common.RenderError pipeline (go-chi/render's json.NewEncoder,
// which HTML-escapes, emits compact JSON, and appends a trailing newline).
//
// Until #2507 these bodies carried human-readable status texts such as
// "Room Conflict". ADR 0006 moved them deliberately to the shared error
// envelope with status "error"; the error text and code stay as they were.
// Changing an expectation here still requires proof that the wire change is
// intentional.
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
			wantBody:   "{\"status\":\"error\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-vorgang-nicht-moeglich\",\"title\":\"Conflict\",\"detail\":\"room is already occupied by another active group\",\"instance\":\"\",\"error\":\"room is already occupied by another active group\",\"code\":\"general.business_rejection\"}\n",
		},
		{
			name:       "ErrActiveGroupNotFound",
			err:        studentpresence.ErrGroupNotFound,
			wantStatus: 404,
			wantBody:   "{\"status\":\"error\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen\",\"title\":\"Not Found\",\"detail\":\"active group not found\",\"instance\":\"\",\"error\":\"active group not found\",\"code\":\"general.input\"}\n",
		},
		{
			name:       "ErrStudentAlreadyActive",
			err:        studentpresence.ErrStudentAlreadyActive,
			wantStatus: 409,
			wantBody:   "{\"status\":\"error\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-vorgang-nicht-moeglich\",\"title\":\"Conflict\",\"detail\":\"student already has an active visit\",\"instance\":\"\",\"error\":\"student already has an active visit\",\"code\":\"general.business_rejection\"}\n",
		},
		{
			name:       "ErrStudentMoveForbidden",
			err:        studentpresence.ErrStudentMoveForbidden,
			wantStatus: 403,
			wantBody:   "{\"status\":\"error\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-zugriff-pruefen\",\"title\":\"Forbidden\",\"detail\":\"not authorized to move the selected students\",\"instance\":\"\",\"error\":\"not authorized to move the selected students\",\"code\":\"general.permission\"}\n",
		},
		{
			name:       "unmapped error defaults to 500",
			err:        errors.New("boom"),
			wantStatus: 500,
			wantBody:   "{\"status\":\"error\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-unerwarteter-fehler\",\"title\":\"Internal Server Error\",\"detail\":\"boom\",\"instance\":\"\",\"error\":\"boom\",\"code\":\"general.server\"}\n",
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
