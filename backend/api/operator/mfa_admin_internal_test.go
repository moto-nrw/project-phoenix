package operator

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

// Operator MFA admin endpoints reuse the auth-side service via
// TenantMFAService + a school-membership check via AccountTenantRepository.
// These internal tests cover Bind-validation + fail-closed behavior when
// either dependency is missing — the same shape the tenant-side admin
// tests have.

func TestOperatorMFAAdminResetRequest_Bind(t *testing.T) {
	t.Parallel()

	t.Run("rejects empty reason", func(t *testing.T) {
		req := MFAAdminResetRequest{Reason: ""}
		assert.Error(t, req.Bind(nil))
	})
	t.Run("rejects too short", func(t *testing.T) {
		req := MFAAdminResetRequest{Reason: "ab"}
		assert.Error(t, req.Bind(nil))
	})
	t.Run("trims + accepts", func(t *testing.T) {
		req := MFAAdminResetRequest{Reason: "  user lost mailbox  "}
		require.NoError(t, req.Bind(nil))
		assert.Equal(t, "user lost mailbox", req.Reason)
	})
}

func TestOperatorMFAAdminOverrideSetRequest_Bind(t *testing.T) {
	t.Parallel()

	t.Run("rejects unknown override value", func(t *testing.T) {
		req := MFAAdminOverrideSetRequest{Override: "maybe", Reason: "test"}
		assert.Error(t, req.Bind(nil))
	})
	t.Run("rejects empty reason", func(t *testing.T) {
		req := MFAAdminOverrideSetRequest{Override: "force_on", Reason: ""}
		assert.Error(t, req.Bind(nil))
	})
	t.Run("accepts each known override + trimmed reason", func(t *testing.T) {
		for _, override := range []string{"none", "force_off", "force_on"} {
			req := MFAAdminOverrideSetRequest{Override: " " + override + " ", Reason: "  reasonable  "}
			require.NoError(t, req.Bind(nil), "override %q must be accepted", override)
			assert.Equal(t, override, req.Override)
			assert.Equal(t, "reasonable", req.Reason)
		}
	})
}

// TestOperatorMFAAdmin_FailsClosedWithoutDependencies locks in the
// requireMFAAdminDeps guard: any missing dependency (TenantMFAService or
// AccountTenantRepository) must answer 500 ("not configured") rather than
// silently doing nothing.
func TestOperatorMFAAdmin_FailsClosedWithoutDependencies(t *testing.T) {
	t.Parallel()

	rs := &ProvisioningResource{} // both deps nil

	cases := []struct {
		name   string
		method string
		body   []byte
		fn     func(http.ResponseWriter, *http.Request)
	}{
		{"get-state", http.MethodGet, nil, rs.GetSchoolAccountMFAState},
		{"reset", http.MethodDelete, mustOperatorJSON(MFAAdminResetRequest{Reason: "lost mailbox"}), rs.ResetSchoolAccountMFA},
		{"set-override", http.MethodPut, mustOperatorJSON(MFAAdminOverrideSetRequest{Override: "none", Reason: "back to default"}), rs.SetSchoolAccountMFAOverride},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := withOperatorAdminParams(httptest.NewRequest(tc.method, "/", bytes.NewReader(tc.body)), "7", "42")
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			tc.fn(rr, req)

			assert.Equal(t, http.StatusInternalServerError, rr.Code,
				"expected 500 when MFA admin deps are nil — got %d body=%s", rr.Code, rr.Body.String())
		})
	}
}

func withOperatorAdminParams(r *http.Request, schoolID, accountID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", schoolID)
	rctx.URLParams.Add("accountId", accountID)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func mustOperatorJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
