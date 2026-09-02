package auth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type accountIsolationEnv struct {
	tc              *testContext
	router          chi.Router
	tenantID        int64
	foreignTenantID int64
	claims          jwt.AppClaims
}

func newAccountIsolationEnv(t *testing.T) *accountIsolationEnv {
	t.Helper()
	tc := setupAuthRoute(t)
	router := testutil.NewTenantRouter(tc.db)
	router.Mount("/auth", tc.resource.Router())
	tenantID := testpkg.Tenant(t)
	actor := testpkg.CreateTestAccount(t, tc.db, "account-scope-actor")
	testpkg.OwnTestAccount(t, tc.db, actor.ID)
	foreignTenantID, _ := testpkg.CreateTestTenant(t, tc.db)
	return &accountIsolationEnv{
		tc:              tc,
		router:          router,
		tenantID:        tenantID,
		foreignTenantID: foreignTenantID,
		claims:          testutil.AdminTestClaimsForTenant(int(actor.ID), tenantID),
	}
}

func (e *accountIsolationEnv) accountForTenant(t *testing.T, prefix string, tenantID int64) (int64, string) {
	t.Helper()
	account := testpkg.CreateTestAccount(t, e.tc.db, prefix)
	testpkg.UnclaimTestAccount(t, e.tc.db, account.ID)
	testpkg.MapAccountToTenant(t, e.tc.db, account.ID, tenantID)
	return account.ID, account.Email
}

func (e *accountIsolationEnv) foreignAccount(t *testing.T, prefix string) (int64, string) {
	t.Helper()
	return e.accountForTenant(t, prefix, e.foreignTenantID)
}

func (e *accountIsolationEnv) missingAccountID(t *testing.T) int64 {
	t.Helper()
	account := testpkg.CreateTestAccount(t, e.tc.db, "account-scope-missing")
	_, err := e.tc.db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	require.NoError(t, err)
	return account.ID
}

func (e *accountIsolationEnv) execute(t *testing.T, claims jwt.AppClaims, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	req := testutil.NewJSONRequest(t, method, path, body)
	return testutil.ExecuteWithAuth(t, e.router, req, claims)
}

func (e *accountIsolationEnv) listAccounts(t *testing.T, claims jwt.AppClaims) []authAPI.AccountResponse {
	t.Helper()
	rr := e.execute(t, claims, http.MethodGet, "/auth/accounts", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var response struct {
		Data []authAPI.AccountResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	return response.Data
}

func accountIDs(accounts []authAPI.AccountResponse) []int64 {
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	return ids
}

func TestAccountManagementOwnTenantAllowed(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	account := testpkg.CreateTestAccount(t, e.tc.db, "account-scope-local")

	got, err := e.tc.resource.AuthService.GetAccountByID(tenant.WithTenantID(context.Background(), e.tenantID), int(account.ID))
	require.NoError(t, err)
	assert.Equal(t, account.ID, got.ID)

	updatedEmail := fmt.Sprintf("account-scope-updated-%d@example.test", account.ID)
	assertAccountActionStatus(t, e, e.claims, http.MethodPut, fmt.Sprintf("/auth/accounts/%d", account.ID), map[string]string{"email": updatedEmail}, http.StatusNoContent)
	assertAccountActionStatus(t, e, e.claims, http.MethodPut, fmt.Sprintf("/auth/accounts/%d/deactivate", account.ID), nil, http.StatusNoContent)
	assertAccountActionStatus(t, e, e.claims, http.MethodPut, fmt.Sprintf("/auth/accounts/%d/activate", account.ID), nil, http.StatusNoContent)

	assert.Contains(t, e.listAccounts(t, e.claims), authAPI.AccountResponse{ID: account.ID, Email: updatedEmail, Active: true})
}

func assertAccountActionStatus(t *testing.T, e *accountIsolationEnv, claims jwt.AppClaims, method, path string, body any, status int) {
	t.Helper()
	rr := e.execute(t, claims, method, path, body)
	require.Equal(t, status, rr.Code, rr.Body.String())
}

func TestAccountManagementForeignAccountsAbsentFromList(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	foreignID, _ := e.foreignAccount(t, "account-scope-list-foreign")
	local := testpkg.CreateTestAccount(t, e.tc.db, "account-scope-list-local")

	ids := accountIDs(e.listAccounts(t, e.claims))
	assert.Contains(t, ids, local.ID)
	assert.NotContains(t, ids, foreignID)
}

func TestAccountManagementRoleFilterExcludesForeignRoleAssignment(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	local := testpkg.CreateTestAccount(t, e.tc.db, "account-scope-role-local")
	foreignID, _ := e.foreignAccount(t, "account-scope-role-foreign")
	role := testpkg.CreateTestRoleForTenant(t, e.tc.db, "account-scope-role", e.tenantID)
	organizationID, sameOrgSchoolID := e.createSchoolInOwnOrganization(t)
	testpkg.MapAccountToTenant(t, e.tc.db, local.ID, sameOrgSchoolID)

	for _, assignment := range []struct {
		accountID int64
		tenantID  int64
	}{
		{accountID: local.ID, tenantID: e.tenantID},
		{accountID: local.ID, tenantID: sameOrgSchoolID},
		{accountID: foreignID, tenantID: e.foreignTenantID},
	} {
		_, err := e.tc.db.ExecContext(context.Background(),
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			assignment.accountID, role.ID, assignment.tenantID)
		require.NoError(t, err)
	}

	claims := e.claims
	claims.Scope = tenant.ScopeOrg
	claims.OrgID = organizationID
	rr := e.execute(t, claims, http.MethodGet, "/auth/accounts/by-role/"+role.Name, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var response struct {
		Data []authAPI.AccountResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))

	ids := accountIDs(response.Data)
	assert.Contains(t, ids, local.ID)
	assert.NotContains(t, ids, foreignID)
	assert.Equal(t, 1, len(ids))
}

func TestAccountManagementRoleFilterExcludesStaleOrganizationRoleAssignment(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	account := testpkg.CreateTestAccount(t, e.tc.db, "account-scope-role-stale")
	role := testpkg.CreateTestRoleForTenant(t, e.tc.db, "account-scope-role-stale", e.tenantID)
	organizationID, staleSchoolID := e.createSchoolInOwnOrganization(t)

	_, err := e.tc.db.ExecContext(context.Background(),
		"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
		account.ID, role.ID, staleSchoolID)
	require.NoError(t, err)

	claims := e.claims
	claims.Scope = tenant.ScopeOrg
	claims.OrgID = organizationID
	rr := e.execute(t, claims, http.MethodGet, "/auth/accounts/by-role/"+role.Name, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var response struct {
		Data []authAPI.AccountResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))

	assert.NotContains(t, accountIDs(response.Data), account.ID)
}

func TestAccountManagementOrganizationRoleFilterIncludesSameOrganizationSchool(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	organizationID, schoolID := e.createSchoolInOwnOrganization(t)
	accountID, _ := e.accountForTenant(t, "account-scope-role-organization", schoolID)
	role := testpkg.CreateTestRoleForTenant(t, e.tc.db, "account-scope-role-organization", schoolID)

	_, err := e.tc.db.ExecContext(context.Background(),
		"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
		accountID, role.ID, schoolID)
	require.NoError(t, err)

	claims := e.claims
	claims.Scope = tenant.ScopeOrg
	claims.OrgID = organizationID
	rr := e.execute(t, claims, http.MethodGet, "/auth/accounts/by-role/"+role.Name, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var response struct {
		Data []authAPI.AccountResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))

	assert.Contains(t, accountIDs(response.Data), accountID)
}

func TestAccountManagementOrganizationScopeReturnsUnmappedOwnAccount(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	testpkg.UnclaimTestAccount(t, e.tc.db, int64(e.claims.ID))
	organizationID, _ := e.createSchoolInOwnOrganization(t)

	claims := e.claims
	claims.Scope = tenant.ScopeOrg
	claims.OrgID = organizationID
	rr := e.execute(t, claims, http.MethodGet, "/auth/account", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var response struct {
		Data authAPI.AccountResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	assert.Equal(t, int64(claims.ID), response.Data.ID)
}

func TestAccountManagementDirectPermissionsStayWithinTenant(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	account := testpkg.CreateTestAccount(t, e.tc.db, "account-scope-direct-permissions")
	testpkg.MapAccountToTenant(t, e.tc.db, account.ID, e.foreignTenantID)
	localPermission := testpkg.CreateTestPermission(t, e.tc.db, "account-scope-direct-local", "account-scope-direct-local", "read")
	foreignPermission := testpkg.CreateTestPermission(t, e.tc.db, "account-scope-direct-foreign", "account-scope-direct-foreign", "read")

	for _, assignment := range []struct {
		permissionID int64
		tenantID     int64
	}{
		{permissionID: localPermission.ID, tenantID: e.tenantID},
		{permissionID: foreignPermission.ID, tenantID: e.foreignTenantID},
	} {
		_, err := e.tc.db.ExecContext(context.Background(),
			"INSERT INTO auth.account_permissions (account_id, permission_id, granted, tenant_id) VALUES (?, ?, true, ?)",
			account.ID, assignment.permissionID, assignment.tenantID)
		require.NoError(t, err)
	}

	rr := e.execute(t, e.claims, http.MethodGet, fmt.Sprintf("/auth/accounts/%d/permissions/direct", account.ID), nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var response struct {
		Data []authAPI.PermissionResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	permissionIDs := make([]int64, 0, len(response.Data))
	for _, permission := range response.Data {
		permissionIDs = append(permissionIDs, permission.ID)
	}
	assert.Contains(t, permissionIDs, localPermission.ID)
	assert.NotContains(t, permissionIDs, foreignPermission.ID)
}

func TestAccountManagementOrganizationRBACRequiresTargetSchoolMembership(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	organizationID, otherSchoolID := e.createSchoolInOwnOrganization(t)
	accountID, _ := e.accountForTenant(t, "account-scope-org-rbac", otherSchoolID)
	role := testpkg.CreateTestRoleForTenant(t, e.tc.db, "account-scope-org-rbac", e.tenantID)
	permission := testpkg.CreateTestPermission(t, e.tc.db, "account-scope-org-rbac", "account-scope-org-rbac", "read")
	claims := e.claims
	claims.Scope = tenant.ScopeOrg
	claims.OrgID = organizationID

	for _, action := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, fmt.Sprintf("/auth/accounts/%d/roles", accountID)},
		{http.MethodPost, fmt.Sprintf("/auth/accounts/%d/roles/%d", accountID, role.ID)},
		{http.MethodDelete, fmt.Sprintf("/auth/accounts/%d/roles/%d", accountID, role.ID)},
		{http.MethodGet, fmt.Sprintf("/auth/accounts/%d/permissions", accountID)},
		{http.MethodGet, fmt.Sprintf("/auth/accounts/%d/permissions/direct", accountID)},
		{http.MethodPost, fmt.Sprintf("/auth/accounts/%d/permissions/%d/grant", accountID, permission.ID)},
		{http.MethodPost, fmt.Sprintf("/auth/accounts/%d/permissions/%d/deny", accountID, permission.ID)},
		{http.MethodDelete, fmt.Sprintf("/auth/accounts/%d/permissions/%d", accountID, permission.ID)},
	} {
		rr := e.execute(t, claims, action.method, action.path, nil)
		assert.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestAccountManagementRBACForeignAccountDoesNotLeakExistence(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	foreignID, _ := e.foreignAccount(t, "account-scope-rbac-foreign")
	missingID := e.missingAccountID(t)
	role := testpkg.CreateTestRoleForTenant(t, e.tc.db, "account-scope-rbac", e.tenantID)
	permission := testpkg.CreateTestPermission(t, e.tc.db, "account-scope-rbac", "account-scope-rbac", "read")

	for _, action := range []struct {
		method string
		path   func(int64) string
	}{
		{http.MethodGet, func(accountID int64) string { return fmt.Sprintf("/auth/accounts/%d/roles", accountID) }},
		{http.MethodPost, func(accountID int64) string { return fmt.Sprintf("/auth/accounts/%d/roles/%d", accountID, role.ID) }},
		{http.MethodDelete, func(accountID int64) string { return fmt.Sprintf("/auth/accounts/%d/roles/%d", accountID, role.ID) }},
		{http.MethodGet, func(accountID int64) string { return fmt.Sprintf("/auth/accounts/%d/permissions", accountID) }},
		{http.MethodGet, func(accountID int64) string { return fmt.Sprintf("/auth/accounts/%d/permissions/direct", accountID) }},
		{http.MethodPost, func(accountID int64) string {
			return fmt.Sprintf("/auth/accounts/%d/permissions/%d/grant", accountID, permission.ID)
		}},
		{http.MethodPost, func(accountID int64) string {
			return fmt.Sprintf("/auth/accounts/%d/permissions/%d/deny", accountID, permission.ID)
		}},
		{http.MethodDelete, func(accountID int64) string {
			return fmt.Sprintf("/auth/accounts/%d/permissions/%d", accountID, permission.ID)
		}},
	} {
		foreign := e.execute(t, e.claims, action.method, action.path(foreignID), nil)
		missing := e.execute(t, e.claims, action.method, action.path(missingID), nil)
		assert.Equal(t, http.StatusNotFound, foreign.Code, foreign.Body.String())
		assert.Equal(t, missing.Code, foreign.Code)
		assert.JSONEq(t, missing.Body.String(), foreign.Body.String())
	}
}

func TestAccountManagementForeignGetDoesNotLeakExistence(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	foreignID, _ := e.foreignAccount(t, "account-scope-get-foreign")
	missingID := e.missingAccountID(t)
	ctx := tenant.WithTenantID(context.Background(), e.tenantID)

	_, foreignErr := e.tc.resource.AuthService.GetAccountByID(ctx, int(foreignID))
	_, missingErr := e.tc.resource.AuthService.GetAccountByID(ctx, int(missingID))
	require.ErrorIs(t, foreignErr, authService.ErrAccountNotFound)
	require.ErrorIs(t, missingErr, authService.ErrAccountNotFound)
	assert.Equal(t, missingErr.Error(), foreignErr.Error())
}

func TestAccountManagementForeignUpdateDoesNotLeakExistence(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	foreignID, originalEmail := e.foreignAccount(t, "account-scope-update-foreign")
	body := map[string]string{"email": fmt.Sprintf("stolen-%d@example.test", foreignID)}
	assertSameNotFoundResponse(t, e,
		http.MethodPut, fmt.Sprintf("/auth/accounts/%d", foreignID),
		http.MethodPut, fmt.Sprintf("/auth/accounts/%d", e.missingAccountID(t)), body)

	var email string
	require.NoError(t, e.tc.db.NewSelect().Column("email").TableExpr("auth.accounts").Where("id = ?", foreignID).Scan(context.Background(), &email))
	assert.Equal(t, originalEmail, email)
}

func TestAccountManagementForeignDeactivateDoesNotLeakExistence(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	foreignID, _ := e.foreignAccount(t, "account-scope-deactivate-foreign")
	assertSameNotFoundResponse(t, e,
		http.MethodPut, fmt.Sprintf("/auth/accounts/%d/deactivate", foreignID),
		http.MethodPut, fmt.Sprintf("/auth/accounts/%d/deactivate", e.missingAccountID(t)), nil)
	assert.True(t, e.accountActive(t, foreignID))
}

func TestAccountManagementForeignActivateDoesNotLeakExistence(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	foreignID, _ := e.foreignAccount(t, "account-scope-activate-foreign")
	_, err := e.tc.db.NewUpdate().TableExpr("auth.accounts").Set("active = false").Where("id = ?", foreignID).Exec(context.Background())
	require.NoError(t, err)

	assertSameNotFoundResponse(t, e,
		http.MethodPut, fmt.Sprintf("/auth/accounts/%d/activate", foreignID),
		http.MethodPut, fmt.Sprintf("/auth/accounts/%d/activate", e.missingAccountID(t)), nil)
	assert.False(t, e.accountActive(t, foreignID))
}

func assertSameNotFoundResponse(t *testing.T, e *accountIsolationEnv, foreignMethod, foreignPath, missingMethod, missingPath string, body any) {
	t.Helper()
	foreign := e.execute(t, e.claims, foreignMethod, foreignPath, body)
	missing := e.execute(t, e.claims, missingMethod, missingPath, body)
	assert.Equal(t, http.StatusNotFound, foreign.Code, foreign.Body.String())
	assert.Equal(t, missing.Code, foreign.Code)
	assert.JSONEq(t, missing.Body.String(), foreign.Body.String())
}

func (e *accountIsolationEnv) accountActive(t *testing.T, accountID int64) bool {
	t.Helper()
	var active bool
	err := e.tc.db.NewSelect().Column("active").TableExpr("auth.accounts").Where("id = ?", accountID).Scan(context.Background(), &active)
	require.NoError(t, err)
	return active
}

func TestAccountManagementInactiveMembershipDenied(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	account := testpkg.CreateTestAccount(t, e.tc.db, "account-scope-inactive")
	_, err := e.tc.db.NewUpdate().TableExpr("auth.account_tenants").Set("status = 'inactive'").Where("account_id = ?", account.ID).Where("tenant_id = ?", e.tenantID).Exec(context.Background())
	require.NoError(t, err)

	ctx := tenant.WithTenantID(context.Background(), e.tenantID)
	_, err = e.tc.resource.AuthService.GetAccountByID(ctx, int(account.ID))
	require.ErrorIs(t, err, authService.ErrAccountNotFound)
	assert.NotContains(t, accountIDs(e.listAccounts(t, e.claims)), account.ID)
	assertAccountActionStatus(t, e, e.claims, http.MethodPut, fmt.Sprintf("/auth/accounts/%d", account.ID), map[string]string{"email": account.Email}, http.StatusNotFound)
}

func TestAccountManagementOrganizationScopeStaysWithinOrganization(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	organizationID, schoolID := e.createSchoolInOwnOrganization(t)
	sameOrgID, _ := e.accountForTenant(t, "account-scope-org-local", schoolID)
	foreignID, _ := e.foreignAccount(t, "account-scope-org-foreign")
	ctx := testpkg.WithTestTenantRuntime(t, tenant.WithScope(tenant.WithOrgID(context.Background(), organizationID), tenant.ScopeOrg))

	got, err := e.tc.resource.AuthService.GetAccountByID(ctx, int(sameOrgID))
	require.NoError(t, err)
	assert.Equal(t, sameOrgID, got.ID)
	_, err = e.tc.resource.AuthService.GetAccountByID(ctx, int(foreignID))
	require.ErrorIs(t, err, authService.ErrAccountNotFound)

	claims := e.claims
	claims.Scope = tenant.ScopeOrg
	claims.OrgID = organizationID
	ids := accountIDs(e.listAccounts(t, claims))
	assert.Contains(t, ids, sameOrgID)
	assert.NotContains(t, ids, foreignID)
}

func (e *accountIsolationEnv) createSchoolInOwnOrganization(t *testing.T) (int64, int64) {
	t.Helper()
	var organizationID int64
	require.NoError(t, e.tc.db.NewSelect().Column("organization_id").TableExpr("platform.schools").Where("id = ?", e.tenantID).Scan(context.Background(), &organizationID))
	schoolID := testpkg.UniqueTestTenantID(t)
	token := fmt.Sprintf("account-scope-org-%d", schoolID)
	_, err := e.tc.db.ExecContext(context.Background(), `
		INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active)
		VALUES (?, ?, ?, ?, ?, true)`, schoolID, organizationID, token, token, token)
	require.NoError(t, err)
	return organizationID, schoolID
}

func TestAccountManagementPlatformScopeRemainsGlobal(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	foreignID, _ := e.foreignAccount(t, "account-scope-platform")
	ctx := tenant.WithScope(context.Background(), tenant.ScopePlatform)

	got, err := e.tc.resource.AuthService.GetAccountByID(ctx, int(foreignID))
	require.NoError(t, err)
	assert.Equal(t, foreignID, got.ID)
	require.NoError(t, e.tc.resource.AuthService.DeactivateAccount(ctx, int(foreignID)))
	require.NoError(t, e.tc.resource.AuthService.ActivateAccount(ctx, int(foreignID)))

	claims := e.claims
	claims.Scope = tenant.ScopePlatform
	claims.TenantID = 0
	assert.Contains(t, accountIDs(e.listAccounts(t, claims)), foreignID)
}
