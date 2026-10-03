// This file pins the exact wire format (HTTP status code + raw JSON response
// bytes) of the Operator* constructors. It renders through the real
// render.Render(...) pipeline (go-chi/render's json.NewEncoder, which
// HTML-escapes, emits compact JSON, and appends a trailing newline) and
// asserts the literal body string. The subtest names keep the old api/operator
// helper names (#3231).
//
// Until #2507 the operator surface had its own body: the text in "message"
// instead of "error", and the status texts "Too Many Requests" and
// "Service Unavailable". ADR 0006 moved it deliberately to the shared error
// envelope that every other surface answers with.
//
// Changing an expectation in this file requires proof that the wire format
// change is intentional, not just "the refactor made the test fail."
package common_test

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/stretchr/testify/assert"
)

func renderWireOperator(t *testing.T, renderer render.Renderer) (int, string) {
	t.Helper()
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	err := render.Render(w, r, renderer)
	assert.NoError(t, err)
	return w.Code, w.Body.String()
}

func TestWireFormat_Operator_ErrHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		renderer   render.Renderer
		wantStatus int
		wantBody   string
	}{
		{
			name:       "ErrInvalidRequest",
			renderer:   common.OperatorInvalidRequest(errors.New("bad")),
			wantStatus: 400,
			wantBody:   `{"status":"error","type":"https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen","title":"Bad Request","detail":"bad","instance":"","error":"bad","code":"general.input"}` + "\n",
		},
		{
			name:       "ErrInvalidCredentials",
			renderer:   common.OperatorInvalidCredentials(),
			wantStatus: 401,
			wantBody:   `{"status":"error","type":"https://moto-app.de/help/fehlermeldungen#anleitung-zugriff-pruefen","title":"Unauthorized","detail":"Invalid email or password","instance":"","error":"Invalid email or password","code":"general.permission"}` + "\n",
		},
		{
			name:       "ErrUnauthorized",
			renderer:   common.OperatorUnauthorized(),
			wantStatus: 401,
			wantBody:   `{"status":"error","type":"https://moto-app.de/help/fehlermeldungen#anleitung-zugriff-pruefen","title":"Unauthorized","detail":"Unauthorized","instance":"","error":"Unauthorized","code":"general.permission"}` + "\n",
		},
		{
			name:       "ErrNotFound",
			renderer:   common.OperatorNotFound("thing missing"),
			wantStatus: 404,
			wantBody:   `{"status":"error","type":"https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen","title":"Not Found","detail":"thing missing","instance":"","error":"thing missing","code":"general.input"}` + "\n",
		},
		{
			name:       "ErrConflict",
			renderer:   common.OperatorConflict("duplicate"),
			wantStatus: 409,
			wantBody:   `{"status":"error","type":"https://moto-app.de/help/fehlermeldungen#anleitung-vorgang-nicht-moeglich","title":"Conflict","detail":"duplicate","instance":"","error":"duplicate","code":"general.business_rejection"}` + "\n",
		},
		{
			name:       "ErrForbidden",
			renderer:   common.OperatorForbidden("no access"),
			wantStatus: 403,
			wantBody:   `{"status":"error","type":"https://moto-app.de/help/fehlermeldungen#anleitung-zugriff-pruefen","title":"Forbidden","detail":"no access","instance":"","error":"no access","code":"general.permission"}` + "\n",
		},
		{
			name:       "ErrTooManyRequests",
			renderer:   common.OperatorTooManyRequests("slow down"),
			wantStatus: 429,
			wantBody:   `{"status":"error","type":"https://moto-app.de/help/fehlermeldungen#anleitung-gerade-nicht-erreichbar","title":"Too Many Requests","detail":"slow down","instance":"","error":"slow down","code":"general.unavailable"}` + "\n",
		},
		{
			name:       "ErrInternal",
			renderer:   common.OperatorInternal("oops"),
			wantStatus: 500,
			wantBody:   `{"status":"error","type":"https://moto-app.de/help/fehlermeldungen#anleitung-unerwarteter-fehler","title":"Internal Server Error","detail":"oops","instance":"","error":"oops","code":"general.server"}` + "\n",
		},
		{
			name:       "ErrServiceUnavailable",
			renderer:   common.OperatorServiceUnavailable("try later"),
			wantStatus: 503,
			wantBody:   `{"status":"error","type":"https://moto-app.de/help/fehlermeldungen#anleitung-gerade-nicht-erreichbar","title":"Service Unavailable","detail":"try later","instance":"","error":"try later","code":"general.unavailable"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotBody := renderWireOperator(t, tt.renderer)
			assert.Equal(t, tt.wantStatus, gotStatus)
			assert.Equal(t, tt.wantBody, gotBody)
			assert.Empty(t, routetest.ProblemEnvelopeViolations([]byte(gotBody), false))
		})
	}
}
