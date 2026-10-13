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
			wantBody:   `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen","title":"Bad Request","status":400,"detail":"bad","instance":"","code":"general.input"}` + "\n",
		},
		{
			name:       "ErrInvalidCredentials",
			renderer:   common.OperatorInvalidCredentials(),
			wantStatus: 401,
			// #2519: the failed login names its reason by code; the type
			// follows the code's input class.
			wantBody: `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen","title":"Unauthorized","status":401,"detail":"Invalid email or password","instance":"","code":"identity.invalid_credentials"}` + "\n",
		},
		{
			name:       "ErrRejectionWithDetails",
			renderer:   common.OperatorRejectionWithDetails(409, common.CodeProvisioningOrganizationHasSchools, "has schools", map[string]any{"school_count": 2}),
			wantStatus: 409,
			wantBody:   `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-vorgang-nicht-moeglich","title":"Conflict","status":409,"detail":"has schools","instance":"","code":"provisioning.organization_has_schools","details":{"school_count":2}}` + "\n",
		},
		{
			name:       "ErrInvalidField",
			renderer:   common.OperatorInvalidField(common.CodeIdentityPasswordTooWeak, "password", "weak"),
			wantStatus: 400,
			wantBody:   `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen","title":"Bad Request","status":400,"detail":"weak","instance":"","code":"identity.password_too_weak","errors":[{"field":"password","reason":"weak"}]}` + "\n",
		},
		{
			name:       "ErrUnauthorized",
			renderer:   common.OperatorUnauthorized(),
			wantStatus: 401,
			wantBody:   `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-zugriff-pruefen","title":"Unauthorized","status":401,"detail":"Unauthorized","instance":"","code":"general.permission"}` + "\n",
		},
		{
			name:       "ErrNotFound",
			renderer:   common.OperatorNotFound("thing missing"),
			wantStatus: 404,
			wantBody:   `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen","title":"Not Found","status":404,"detail":"thing missing","instance":"","code":"general.input"}` + "\n",
		},
		{
			name:       "ErrConflict",
			renderer:   common.OperatorConflict("duplicate"),
			wantStatus: 409,
			wantBody:   `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-vorgang-nicht-moeglich","title":"Conflict","status":409,"detail":"duplicate","instance":"","code":"general.business_rejection"}` + "\n",
		},
		{
			name:       "ErrForbidden",
			renderer:   common.OperatorForbidden("no access"),
			wantStatus: 403,
			wantBody:   `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-zugriff-pruefen","title":"Forbidden","status":403,"detail":"no access","instance":"","code":"general.permission"}` + "\n",
		},
		{
			name:       "ErrInternal",
			renderer:   common.OperatorInternal("oops"),
			wantStatus: 500,
			wantBody:   `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-unerwarteter-fehler","title":"Internal Server Error","status":500,"detail":"oops","instance":"","code":"general.server"}` + "\n",
		},
		{
			name:       "ErrServiceUnavailable",
			renderer:   common.OperatorServiceUnavailable("try later"),
			wantStatus: 503,
			wantBody:   `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-gerade-nicht-erreichbar","title":"Service Unavailable","status":503,"detail":"try later","instance":"","code":"general.unavailable"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotBody := renderWireOperator(t, tt.renderer)
			assert.Equal(t, tt.wantStatus, gotStatus)
			assert.Equal(t, tt.wantBody, gotBody)
			assert.Empty(t, routetest.ProblemEnvelopeViolations([]byte(gotBody), gotStatus, false))
		})
	}
}
