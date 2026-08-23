// Package operator_test pins the exact wire format (HTTP status code + raw
// JSON response bytes) produced by api/operator's hand-rolled ErrResponse
// renderer and its Err* constructor helpers.
//
// These are B0 wire-format golden tests for issue #575 (API layer
// technical-debt / error-response consolidation). The upcoming B1 refactor
// collapses the per-package ErrResponse/ErrorRenderer duplication (active,
// feedback, operator, ...) into shared helpers in api/common.
// That refactor MUST NOT change a single byte of what a client currently
// receives on the wire. This file is the oracle: it renders through the
// real render.Render(...) pipeline (go-chi/render's json.NewEncoder, which
// HTML-escapes, emits compact JSON, and appends a trailing newline) and
// asserts the literal body string.
//
// Note: operator.ErrResponse diverges from every other package's
// ErrResponse in two ways that B1 must reconcile or deliberately preserve:
//  1. The error-text field is tagged json:"message" here, not json:"error"
//     like api/active, api/feedback, api/iot/common.
//  2. Most helpers hardcode StatusText to the literal string "error"
//     (lowercase, not human text like "Forbidden"), except
//     ErrTooManyRequests ("Too Many Requests") and ErrServiceUnavailable
//     ("Service Unavailable"), which use human text.
//
// Changing an expectation in this file requires proof that the wire format
// change is intentional — not just "the refactor made the test fail."
package operator_test

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/operator"
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
			renderer:   operator.ErrInvalidRequest(errors.New("bad")),
			wantStatus: 400,
			wantBody:   "{\"status\":\"error\",\"message\":\"bad\"}\n",
		},
		{
			name:       "ErrInvalidCredentials",
			renderer:   operator.ErrInvalidCredentials(),
			wantStatus: 401,
			wantBody:   "{\"status\":\"error\",\"message\":\"Invalid email or password\"}\n",
		},
		{
			name:       "ErrUnauthorized",
			renderer:   operator.ErrUnauthorized(),
			wantStatus: 401,
			wantBody:   "{\"status\":\"error\",\"message\":\"Unauthorized\"}\n",
		},
		{
			name:       "ErrNotFound",
			renderer:   operator.ErrNotFound("thing missing"),
			wantStatus: 404,
			wantBody:   "{\"status\":\"error\",\"message\":\"thing missing\"}\n",
		},
		{
			name:       "ErrConflict",
			renderer:   operator.ErrConflict("duplicate"),
			wantStatus: 409,
			wantBody:   "{\"status\":\"error\",\"message\":\"duplicate\"}\n",
		},
		{
			name:       "ErrForbidden",
			renderer:   operator.ErrForbidden("no access"),
			wantStatus: 403,
			wantBody:   "{\"status\":\"error\",\"message\":\"no access\"}\n",
		},
		{
			name:       "ErrTooManyRequests",
			renderer:   operator.ErrTooManyRequests("slow down"),
			wantStatus: 429,
			wantBody:   "{\"status\":\"Too Many Requests\",\"message\":\"slow down\"}\n",
		},
		{
			name:       "ErrInternal",
			renderer:   operator.ErrInternal("oops"),
			wantStatus: 500,
			wantBody:   "{\"status\":\"error\",\"message\":\"oops\"}\n",
		},
		{
			name:       "ErrServiceUnavailable",
			renderer:   operator.ErrServiceUnavailable("try later"),
			wantStatus: 503,
			wantBody:   "{\"status\":\"Service Unavailable\",\"message\":\"try later\"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotBody := renderWireOperator(t, tt.renderer)
			assert.Equal(t, tt.wantStatus, gotStatus)
			assert.Equal(t, tt.wantBody, gotBody)
		})
	}
}
