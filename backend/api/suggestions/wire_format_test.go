// Package suggestions_test pins the exact wire format (HTTP status code +
// raw JSON response bytes) produced by api/suggestions's hand-rolled
// ErrResponse renderer.
//
// These are B0 wire-format golden tests for issue #575 (API layer
// technical-debt / error-response consolidation). The upcoming B1 refactor
// collapses the per-package ErrResponse/ErrorRenderer duplication (active,
// feedback, suggestions, ...) into shared helpers in api/common. That
// refactor MUST NOT change a single byte of what a client currently
// receives on the wire. This file is the oracle: it renders through the
// real render.Render(...) pipeline (go-chi/render's json.NewEncoder, which
// HTML-escapes, emits compact JSON, and appends a trailing newline) and
// asserts the literal body string.
//
// Note: unlike api/active and api/feedback, suggestions.ErrResponse has no
// "code" field at all (only "status" and "error") — that shape difference
// is itself part of what B1 must reconcile or deliberately preserve.
//
// Changing an expectation in this file requires proof that the wire format
// change is intentional — not just "the refactor made the test fail."
package suggestions_test

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/suggestions"
	suggestionsSvc "github.com/moto-nrw/project-phoenix/services/suggestions"
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

func TestWireFormat_Suggestions_ErrorRenderer(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "ErrPostNotFound",
			err:        suggestionsSvc.ErrPostNotFound,
			wantStatus: 404,
			wantBody:   "{\"status\":\"Not Found\",\"error\":\"suggestion post not found\"}\n",
		},
		{
			name:       "ErrCommentNotFound",
			err:        suggestionsSvc.ErrCommentNotFound,
			wantStatus: 404,
			wantBody:   "{\"status\":\"Not Found\",\"error\":\"comment not found\"}\n",
		},
		{
			name:       "ErrForbidden",
			err:        suggestionsSvc.ErrForbidden,
			wantStatus: 403,
			wantBody:   "{\"status\":\"Forbidden\",\"error\":\"forbidden: you can only modify your own suggestions\"}\n",
		},
		{
			name:       "ErrInvalidData",
			err:        suggestionsSvc.ErrInvalidData,
			wantStatus: 400,
			wantBody:   "{\"status\":\"Bad Request\",\"error\":\"invalid suggestion data\"}\n",
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
			renderer := suggestions.ErrorRenderer(tt.err)
			gotStatus, gotBody := renderWire(t, renderer)
			assert.Equal(t, tt.wantStatus, gotStatus)
			assert.Equal(t, tt.wantBody, gotBody)
		})
	}
}

func TestWireFormat_Suggestions_ErrorInvalidRequest(t *testing.T) {
	renderer := suggestions.ErrorInvalidRequest(errors.New("bad"))
	gotStatus, gotBody := renderWire(t, renderer)
	assert.Equal(t, 400, gotStatus)
	assert.Equal(t, "{\"status\":\"Invalid Request\",\"error\":\"bad\"}\n", gotBody)
}
