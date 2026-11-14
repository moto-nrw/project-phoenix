package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// TestSchoolPortalListsAndRevokesOnlyItsOwnDevices: moto schule shows the
// devices it remembered at the session's school and removes them; a device
// the tenant portal remembered at the same school is neither listed nor
// removable there (#6955).
func TestSchoolPortalListsAndRevokesOnlyItsOwnDevices(t *testing.T) {
	t.Parallel()
	db, resource, tenantID, _ := setupSchoolRoute(t)
	email, accountID := registerLehrkraft(t, db, resource, tenantID, "school-devices")
	router := newSchoolChiRouter(resource)
	device := func(portal, label string) int64 {
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO auth.mfa_trusted_devices (account_id, portal, tenant_id, token_hash, user_agent, expires_at)
			VALUES (?, ?, ?, ?, ?, NOW() + INTERVAL '30 days') RETURNING id`, accountID, portal, tenantID, fmt.Sprintf("%s-%d", label, accountID), label).
			Scan(context.Background(), &id))
		return id
	}
	schoolDevice := device("school", "Tablet in moto schule")
	tenantDevice := device("tenant", "Laptop im Schul-Portal")
	claims := sessiontoken.AppClaims{ID: int(accountID), Sub: email, Roles: []string{"lehrkraft"}, TenantID: tenantID, Scope: tenant.ScopeSchool}
	send := func(method, path string) *httptest.ResponseRecorder {
		return testutil.ExecuteWithAuthPermissions(t, router, httptest.NewRequest(method, path, nil), claims, nil)
	}

	rec := send(http.MethodGet, "/auth/mfa/trusted-devices")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var listed []struct {
		ID        int64  `json:"id"`
		UserAgent string `json:"user_agent"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listed))
	require.Len(t, listed, 1, "moto schule lists its own device only")
	assert.Equal(t, schoolDevice, listed[0].ID)

	rec = send(http.MethodDelete, fmt.Sprintf("/auth/mfa/trusted-devices/%d", tenantDevice))
	assert.Equal(t, http.StatusForbidden, rec.Code, "a tenant-portal device is not moto schule's to remove")
	rec = send(http.MethodDelete, fmt.Sprintf("/auth/mfa/trusted-devices/%d", schoolDevice))
	assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	var active int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.mfa_trusted_devices WHERE account_id = ? AND revoked_at IS NULL`, accountID).
		Scan(context.Background(), &active))
	assert.Equal(t, 1, active, "only the school device was removed")

	tenantClaims := claims
	tenantClaims.Scope = tenant.ScopeTenant
	rec = testutil.ExecuteWithAuthPermissions(t, router, httptest.NewRequest(http.MethodGet, "/auth/mfa/trusted-devices", nil), tenantClaims, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "a tenant token does not open moto schule's device list")
}
