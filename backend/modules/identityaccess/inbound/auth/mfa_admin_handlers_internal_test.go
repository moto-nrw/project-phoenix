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
	t.Parallel()

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
	t.Parallel()

	rs := &Resource{}

	cases := []struct {
		name   string
		method string
		body   []byte
		fn     func(http.ResponseWriter, *http.Request)
	}{
		{"admin-disable", http.MethodDelete, mustJSON(MFAAdminOverrideRequest{Reason: "lost mailbox"}), rs.mfaAdminDisable},
		{"admin-get-state", http.MethodGet, nil, rs.mfaAdminGetState},
		{"admin-set-override", http.MethodPut, mustJSON(MFAAdminOverrideSetRequest{Override: "none", Reason: "test"}), rs.mfaAdminSetOverride},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader *bytes.Reader
			if tc.body != nil {
				bodyReader = bytes.NewReader(tc.body)
			} else {
				bodyReader = bytes.NewReader(nil)
			}
			req := withAdminAccountIDParam(httptest.NewRequest(tc.method, "/", bodyReader), "42")
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			tc.fn(rr, req)

			assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		})
	}
}

// TestMFAAdminOverrideSetRequest_Bind validates the new tri-state override
// request. The DB has a CHECK constraint, but rejecting at the API layer
// gives the user a clear error instead of a generic 500.
func TestMFAAdminOverrideSetRequest_Bind(t *testing.T) {
	t.Parallel()

	t.Run("rejects unknown override value", func(t *testing.T) {
		req := MFAAdminOverrideSetRequest{Override: "yes-please", Reason: "test"}
		assert.Error(t, req.Bind(nil))
	})
	t.Run("rejects empty reason", func(t *testing.T) {
		req := MFAAdminOverrideSetRequest{Override: "force_off", Reason: ""}
		assert.Error(t, req.Bind(nil))
	})
	t.Run("rejects short reason", func(t *testing.T) {
		req := MFAAdminOverrideSetRequest{Override: "force_off", Reason: "ab"}
		assert.Error(t, req.Bind(nil))
	})
	t.Run("accepts force_off + trimmed reason", func(t *testing.T) {
		req := MFAAdminOverrideSetRequest{Override: " force_off ", Reason: "  user lost mailbox  "}
		require.NoError(t, req.Bind(nil))
		assert.Equal(t, "force_off", req.Override)
		assert.Equal(t, "user lost mailbox", req.Reason)
	})
	t.Run("accepts force_on", func(t *testing.T) {
		req := MFAAdminOverrideSetRequest{Override: "force_on", Reason: "must always 2FA"}
		require.NoError(t, req.Bind(nil))
	})
	t.Run("accepts none (clear override)", func(t *testing.T) {
		req := MFAAdminOverrideSetRequest{Override: "none", Reason: "user got mailbox back"}
		require.NoError(t, req.Bind(nil))
	})
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
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
