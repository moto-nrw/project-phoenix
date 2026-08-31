package auth_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func previewStartBody(t *testing.T, accountID int64) *bytes.Reader {
	t.Helper()
	// account_id is a JSON string on the wire (int64-safe for JS clients).
	payload, err := json.Marshal(map[string]string{"account_id": fmt.Sprint(accountID)})
	require.NoError(t, err)
	return bytes.NewReader(payload)
}

// TestStaffPreviewEndpoints drives the production /auth router: permission
// gate, token minting, and the read-only write block for preview tokens.
func TestStaffPreviewEndpoints(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	admin := testpkg.CreateTestAccount(t, tc.db, "preview-api-admin")
	adminClaims := jwt.AppClaims{
		ID:          int(admin.ID),
		TenantID:    testpkg.Tenant(t),
		Sub:         admin.Email,
		Roles:       []string{"admin"},
		Permissions: []string{"admin:*"},
	}

	_, target := testpkg.CreateTestCalendarStaff(t, tc.db, "Ziel", "Person")

	// Filled by the minting subtest and consumed by the end subtests below —
	// the end call proves with the real token which preview it closes.
	var mintedPreviewToken string

	t.Run("non-admin cannot start a preview", func(t *testing.T) {
		claims := jwt.AppClaims{
			ID:          int(target.ID),
			TenantID:    testpkg.Tenant(t),
			Sub:         target.Email,
			Roles:       []string{"user"},
			Permissions: []string{"users:read"},
		}
		req := httptest.NewRequest(http.MethodPost, "/auth/staff-preview", previewStartBody(t, admin.ID))
		req.Header.Set("Content-Type", "application/json")
		rec := testutil.ExecuteWithAuth(t, router, req, claims)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("admin mints a read-only preview token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/staff-preview", previewStartBody(t, target.ID))
		req.Header.Set("Content-Type", "application/json")
		rec := testutil.ExecuteWithAuth(t, router, req, adminClaims)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var resp struct {
			AccessToken     string `json:"access_token"`
			ExpiresIn       int64  `json:"expires_in"`
			TargetAccountID int64  `json:"target_account_id,string"`
			TargetName      string `json:"target_name"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, target.ID, resp.TargetAccountID)
		// The ID must reach the JS client as a string, never as a number.
		assert.Contains(t, rec.Body.String(), fmt.Sprintf(`"target_account_id":"%d"`, target.ID))
		assert.Equal(t, "Ziel Person", resp.TargetName)
		assert.Positive(t, resp.ExpiresIn)
		mintedPreviewToken = resp.AccessToken

		parts := strings.Split(resp.AccessToken, ".")
		require.Len(t, parts, 3)
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		require.NoError(t, err)
		var claims map[string]any
		require.NoError(t, json.Unmarshal(payload, &claims))
		assert.Equal(t, true, claims["read_only"])
		assert.EqualValues(t, admin.ID, claims["acting_admin_id"])
		assert.EqualValues(t, target.ID, claims["id"])
	})

	t.Run("preview token is blocked on every write under /auth", func(t *testing.T) {
		previewClaims := jwt.AppClaims{
			ID:            int(target.ID),
			TenantID:      testpkg.Tenant(t),
			Sub:           target.Email,
			Roles:         []string{"user"},
			Permissions:   []string{"users:read"},
			ReadOnly:      true,
			ActingAdminID: admin.ID,
		}

		// /auth/staff-preview/end is deliberately absent: it is a PUBLIC
		// token-proved route (the signed preview token in the body is the
		// credential), so a preview session may end itself through it.
		writes := []struct{ method, path, body string }{
			{http.MethodPost, "/auth/switch-tenant", `{"tenant_slug":"t1"}`},
			{http.MethodPost, "/auth/password", `{"current_password":"a","new_password":"b"}`},
			{http.MethodPost, "/auth/staff-preview", fmt.Sprintf(`{"account_id":"%d"}`, admin.ID)},
		}
		for _, w := range writes {
			req := httptest.NewRequest(w.method, w.path, strings.NewReader(w.body))
			req.Header.Set("Content-Type", "application/json")
			rec := testutil.ExecuteWithAuth(t, router, req, previewClaims)
			assert.Equalf(t, http.StatusForbidden, rec.Code, "%s %s must be blocked", w.method, w.path)
			assert.Containsf(t, rec.Body.String(), "read_only_preview", "%s %s", w.method, w.path)
		}

		// Reads keep working — the preview must see what the target sees.
		req := httptest.NewRequest(http.MethodGet, "/auth/account/tenants", nil)
		rec := testutil.ExecuteWithAuth(t, router, req, previewClaims)
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	// The end route is public and token-proved: the signed preview token in
	// the body is the credential, so the end stays recordable after the
	// admin's own tokens have expired (laptop closed for a week mid-preview).
	t.Run("end works without any session — the signed token is the proof", func(t *testing.T) {
		require.NotEmpty(t, mintedPreviewToken, "minting subtest must run first")
		body := fmt.Sprintf(`{"preview_token":%q}`, mintedPreviewToken)
		req := httptest.NewRequest(http.MethodPost, "/auth/staff-preview/end", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	// Only a server-minted preview token can write an end row — nobody can
	// stamp the audit trail with a preview that never happened.
	t.Run("end rejects anything that is not a preview token", func(t *testing.T) {
		cases := map[string]string{
			"a regular session token":            testutil.MintTestJWT(t, adminClaims),
			"a preview token without preview id": testutil.MintTestJWT(t, jwt.AppClaims{ID: int(target.ID), TenantID: testpkg.Tenant(t), Sub: target.Email, ReadOnly: true, ActingAdminID: admin.ID}),
			"garbage":                            "not-a-token",
		}
		for name, token := range cases {
			body := fmt.Sprintf(`{"preview_token":%q}`, token)
			req := httptest.NewRequest(http.MethodPost, "/auth/staff-preview/end", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equalf(t, http.StatusForbidden, rec.Code, "%s must be refused", name)
			assert.Containsf(t, rec.Body.String(), "preview_token_invalid", "%s", name)
		}
	})

	t.Run("end refuses an empty payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/staff-preview/end", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("admin lists candidates, caller and guardian-only excluded", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/staff-preview/candidates", nil)
		rec := testutil.ExecuteWithAuth(t, router, req, adminClaims)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var resp struct {
			Data []struct {
				AccountID int64  `json:"account_id,string"`
				FirstName string `json:"first_name"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

		ids := make(map[int64]bool, len(resp.Data))
		for _, c := range resp.Data {
			ids[c.AccountID] = true
		}
		assert.True(t, ids[target.ID], "staff target must be listed")
		assert.False(t, ids[admin.ID], "caller must not be listed")
	})

	t.Run("non-admin cannot list candidates", func(t *testing.T) {
		claims := jwt.AppClaims{
			ID:          int(target.ID),
			TenantID:    testpkg.Tenant(t),
			Sub:         target.Email,
			Roles:       []string{"user"},
			Permissions: []string{"users:read"},
		}
		req := httptest.NewRequest(http.MethodGet, "/auth/staff-preview/candidates", nil)
		rec := testutil.ExecuteWithAuth(t, router, req, claims)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}
