package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMFAAdminOverrideRequest_Bind(t *testing.T) {
	t.Run("rejects empty reason", func(t *testing.T) {
		req := MFAAdminOverrideRequest{Reason: ""}
		assert.Error(t, req.Bind(nil))
	})
	t.Run("rejects too short", func(t *testing.T) {
		req := MFAAdminOverrideRequest{Reason: "ab"}
		assert.Error(t, req.Bind(nil))
	})
	t.Run("trims and accepts", func(t *testing.T) {
		req := MFAAdminOverrideRequest{Reason: "  user lost mailbox access  "}
		require.NoError(t, req.Bind(nil))
		assert.Equal(t, "user lost mailbox access", req.Reason)
	})
}

// TestMFAAdminHandlers_ServiceUnavailableWhenNotWired locks in the same
// fail-closed default the user-facing handlers have: a deployment without
// MFAService wired in must answer 503, never silently fall through.
func TestMFAAdminHandlers_ServiceUnavailableWhenNotWired(t *testing.T) {
	rs := &Resource{}

	cases := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"admin-disable", rs.mfaAdminDisable},
		{"admin-regenerate-recovery", rs.mfaAdminRegenerateRecoveryCodes},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(MFAAdminOverrideRequest{Reason: "lost mailbox"})
			require.NoError(t, err)
			req := withAdminAccountIDParam(httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)), "42")
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			tc.fn(rr, req)

			assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		})
	}
}

// withAdminAccountIDParam injects the {accountId} URL param into the chi
// route context so the handler under test can read it via chi.URLParam.
// Mirrors withAccountRouteParam from caregiver_capability_internal_test.go
// — kept locally to avoid coupling the two test files.
func withAdminAccountIDParam(r *http.Request, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
