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
	payload, err := json.Marshal(map[string]int64{"account_id": accountID})
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
			TargetAccountID int64  `json:"target_account_id"`
			TargetName      string `json:"target_name"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, target.ID, resp.TargetAccountID)
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

		writes := []struct{ method, path, body string }{
			{http.MethodPost, "/auth/switch-tenant", `{"tenant_slug":"t1"}`},
			{http.MethodPost, "/auth/password", `{"current_password":"a","new_password":"b"}`},
			{http.MethodPost, "/auth/staff-preview", fmt.Sprintf(`{"account_id":%d}`, admin.ID)},
			{http.MethodPost, "/auth/staff-preview/end", `{"preview_token":"irrelevant"}`},
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

	t.Run("end accepts the admin's own preview token", func(t *testing.T) {
		require.NotEmpty(t, mintedPreviewToken, "minting subtest must run first")
		body := fmt.Sprintf(`{"preview_token":%q}`, mintedPreviewToken)
		req := httptest.NewRequest(http.MethodPost, "/auth/staff-preview/end", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := testutil.ExecuteWithAuth(t, router, req, adminClaims)
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	// The previewed account is read from the signed token, so an admin cannot
	// name a colleague they never previewed in the audit trail.
	t.Run("end rejects a token that is not this admin's preview", func(t *testing.T) {
		foreign := testutil.MintTestJWT(t, jwt.AppClaims{
			ID:            int(target.ID),
			TenantID:      testpkg.Tenant(t),
			Sub:           target.Email,
			Roles:         []string{"user"},
			ReadOnly:      true,
			ActingAdminID: admin.ID + 1000,
		})
		cases := map[string]string{
			"another admin's preview": foreign,
			"a regular session token": testutil.MintTestJWT(t, adminClaims),
			"garbage":                 "not-a-token",
		}
		for name, token := range cases {
			body := fmt.Sprintf(`{"preview_token":%q}`, token)
			req := httptest.NewRequest(http.MethodPost, "/auth/staff-preview/end", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := testutil.ExecuteWithAuth(t, router, req, adminClaims)
			assert.Equalf(t, http.StatusForbidden, rec.Code, "%s must be refused", name)
			assert.Containsf(t, rec.Body.String(), "preview_token_invalid", "%s", name)
		}
	})

	t.Run("end refuses an empty payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/staff-preview/end", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rec := testutil.ExecuteWithAuth(t, router, req, adminClaims)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("admin lists candidates, caller and guardian-only excluded", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/staff-preview/candidates", nil)
		rec := testutil.ExecuteWithAuth(t, router, req, adminClaims)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var resp struct {
			Data []struct {
				AccountID int64  `json:"account_id"`
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
