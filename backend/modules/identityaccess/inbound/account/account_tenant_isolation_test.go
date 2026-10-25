package account_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	rootAPI "github.com/moto-nrw/project-phoenix/api"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type accountIsolationEnv struct {
	tc              *testContext
	router          chi.Router
	tenantID        int64
	foreignTenantID int64
	claims          testutil.Claims
}

func newAccountIsolationEnv(t *testing.T) *accountIsolationEnv {
	t.Helper()
	tc := setupAuthRoute(t)
	router := testutil.NewTenantRouter(tc.db)
	router.Mount("/auth", tc.resource.Router(rootAPI.TenantSessionChain))
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

func (e *accountIsolationEnv) execute(t *testing.T, claims testutil.Claims, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	req := testutil.NewJSONRequest(t, method, path, body)
	return testutil.ExecuteWithAuth(t, e.router, req, claims)
}

func (e *accountIsolationEnv) listAccounts(t *testing.T, claims testutil.Claims) []authAPI.AccountResponse {
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

	got, err := e.tc.resource.Sessions.FindManageableAccount(tenant.WithTenantID(context.Background(), e.tenantID), account.ID)
	require.NoError(t, err)
	assert.Equal(t, account.ID, got.ID)

	updatedEmail := fmt.Sprintf("account-scope-updated-%d@example.test", account.ID)
	assertAccountActionStatus(t, e, e.claims, http.MethodPut, fmt.Sprintf("/auth/accounts/%d", account.ID), map[string]string{"email": updatedEmail}, http.StatusNoContent)

	assert.Contains(t, e.listAccounts(t, e.claims), authAPI.AccountResponse{ID: account.ID, Email: updatedEmail, Active: true})
}

func assertAccountActionStatus(t *testing.T, e *accountIsolationEnv, claims testutil.Claims, method, path string, body any, status int) {
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
	_, sameOrgSchoolID := e.createSchoolInOwnOrganization(t)
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

	rr := e.execute(t, e.claims, http.MethodGet, "/auth/accounts/by-role/"+role.Name, nil)
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

func TestAccountManagementRBACForeignAccountDoesNotLeakExistence(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	foreignID, _ := e.foreignAccount(t, "account-scope-rbac-foreign")
	missingID := e.missingAccountID(t)
	role := testpkg.CreateTestRoleForTenant(t, e.tc.db, "account-scope-rbac", e.tenantID)

	for _, action := range []struct {
		method string
		path   func(int64) string
		body   any
	}{
		{http.MethodGet, func(accountID int64) string { return fmt.Sprintf("/auth/accounts/%d/roles", accountID) }, nil},
		{http.MethodPut, func(accountID int64) string { return fmt.Sprintf("/auth/accounts/%d/roles", accountID) }, map[string]string{"role_id": fmt.Sprintf("%d", role.ID)}},
	} {
		foreign := e.execute(t, e.claims, action.method, action.path(foreignID), action.body)
		missing := e.execute(t, e.claims, action.method, action.path(missingID), action.body)
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

	_, foreignErr := e.tc.resource.Sessions.FindManageableAccount(ctx, foreignID)
	_, missingErr := e.tc.resource.Sessions.FindManageableAccount(ctx, missingID)
	require.ErrorIs(t, foreignErr, identityaccess.ErrAccountNotFound)
	require.ErrorIs(t, missingErr, identityaccess.ErrAccountNotFound)
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

func assertSameNotFoundResponse(t *testing.T, e *accountIsolationEnv, foreignMethod, foreignPath, missingMethod, missingPath string, body any) {
	t.Helper()
	foreign := e.execute(t, e.claims, foreignMethod, foreignPath, body)
	missing := e.execute(t, e.claims, missingMethod, missingPath, body)
	assert.Equal(t, http.StatusNotFound, foreign.Code, foreign.Body.String())
	assert.Equal(t, missing.Code, foreign.Code)
	assert.JSONEq(t, missing.Body.String(), foreign.Body.String())
}

func TestAccountManagementInactiveMembershipDenied(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	account := testpkg.CreateTestAccount(t, e.tc.db, "account-scope-inactive")
	_, err := e.tc.db.NewUpdate().TableExpr("auth.account_tenants").Set("status = 'inactive'").Where("account_id = ?", account.ID).Where("tenant_id = ?", e.tenantID).Exec(context.Background())
	require.NoError(t, err)

	ctx := tenant.WithTenantID(context.Background(), e.tenantID)
	_, err = e.tc.resource.Sessions.FindManageableAccount(ctx, account.ID)
	require.ErrorIs(t, err, identityaccess.ErrAccountNotFound)
	assert.NotContains(t, accountIDs(e.listAccounts(t, e.claims)), account.ID)
	assertAccountActionStatus(t, e, e.claims, http.MethodPut, fmt.Sprintf("/auth/accounts/%d", account.ID), map[string]string{"email": account.Email}, http.StatusNotFound)
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

	got, err := e.tc.resource.Sessions.FindManageableAccount(ctx, foreignID)
	require.NoError(t, err)
	assert.Equal(t, foreignID, got.ID)
	require.NoError(t, e.tc.resource.Sessions.DeactivateAccount(ctx, foreignID))
	require.NoError(t, e.tc.resource.Sessions.ActivateAccount(ctx, foreignID))

	claims := e.claims
	claims.Scope = tenant.ScopePlatform
	claims.TenantID = 0
	assert.Contains(t, accountIDs(e.listAccounts(t, claims)), foreignID)
}

// Organization scope must not detach a command from an existing school unit.
func TestAccountManagementOrganizationWritesRollBackWithCaller(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	organizationID, _ := e.createSchoolInOwnOrganization(t)
	account := testpkg.CreateTestAccount(t, e.tc.db, "org-write-rollback")
	ctx := testpkg.WithTestTenantRuntime(t, tenant.WithScope(tenant.WithOrgID(testpkg.Ctx(t), organizationID), tenant.ScopeOrg))
	rollback := errors.New("later workflow command failed")
	err := testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if err := e.tc.resource.Sessions.UpdateManageableAccount(txCtx, identityaccess.AccountIdentityUpdate{
			AccountID: account.ID, Email: fmt.Sprintf("rollback-%d@example.invalid", account.ID),
		}); err != nil {
			return err
		}
		if err := e.tc.resource.Sessions.DeactivateAccount(txCtx, account.ID); err != nil {
			return err
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	got, err := e.tc.resource.Sessions.FindOwnAccount(ctx, account.ID)
	require.NoError(t, err)
	assert.Equal(t, account.Email, got.Email)
	assert.True(t, got.Active)
}

func TestOrganizationStaffOffboardingPreservesLockedCallerTransaction(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	organizationID, _ := e.createSchoolInOwnOrganization(t)
	_, account := testpkg.CreateTestCalendarStaff(t, e.tc.db, "Organization", "Offboarding")
	testpkg.CreateTestToken(t, e.tc.db, account.ID, "refresh")
	access := setupAuthModuleOn(t, e.tc.db).Auth
	ctx := testpkg.WithTestTenantRuntime(t, tenant.WithScope(tenant.WithOrgID(testpkg.Ctx(t), organizationID), tenant.ScopeOrg))
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	preview, err := access.PreviewStaffOffboarding(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, preview.DeactivateAccount)
	rollback := errors.New("later workflow command failed")
	err = testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		result, executeErr := access.ExecuteStaffOffboarding(txCtx, account.ID, preview.Revision)
		if executeErr != nil {
			return executeErr
		}
		require.True(t, result.AccountDeactivated)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	// Use a fresh context so a lock-wait timeout cannot mask the persisted state.
	got, err := e.tc.resource.Sessions.FindOwnAccount(testpkg.Ctx(t), account.ID)
	require.NoError(t, err)
	require.True(t, got.Active)
	membership, err := e.tc.resource.Sessions.VerifyAccountTenantMembership(testpkg.Ctx(t), account.ID, e.tenantID)
	require.NoError(t, err)
	require.True(t, membership)
	result, err := access.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
	require.NoError(t, err, "the rolled-back preview must still be executable")
	require.True(t, result.AccountDeactivated)
	require.EqualValues(t, 1, result.TokensRevoked)
	got, err = e.tc.resource.Sessions.FindOwnAccount(testpkg.Ctx(t), account.ID)
	require.NoError(t, err)
	require.False(t, got.Active)
}

func TestAccountManagementOrganizationWriteRetainsCallerSchoolBoundary(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	organizationID, schoolID := e.createSchoolInOwnOrganization(t)
	accountID, email := e.accountForTenant(t, "org-school-boundary", schoolID)
	ctx := testpkg.WithTestTenantRuntime(t, tenant.WithScope(tenant.WithOrgID(testpkg.Ctx(t), organizationID), tenant.ScopeOrg))
	err := testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return e.tc.resource.Sessions.UpdateManageableAccount(txCtx, identityaccess.AccountIdentityUpdate{
			AccountID: accountID, Email: fmt.Sprintf("denied-%d@example.invalid", accountID),
		})
	})
	require.ErrorIs(t, err, identityaccess.ErrAccountNotFound)
	got, err := e.tc.resource.Sessions.FindOwnAccount(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, email, got.Email)
}

// TestAccountManagementRejectsTraegerTokens: ADR 0084 removed the former org
// scope of the OGS portal. A Träger token (scope org) names no school and
// reaches no account, role or permission route, whatever rights it carries.
func TestAccountManagementRejectsTraegerTokens(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	organizationID, schoolID := e.createSchoolInOwnOrganization(t)
	accountID, _ := e.accountForTenant(t, "account-scope-traeger", schoolID)
	claims := testutil.Claims{ID: e.claims.ID, Sub: e.claims.Sub, Roles: []string{"admin"}, Permissions: []string{"admin:*"},
		IsAdmin: true, Scope: tenant.ScopeOrg, OrgID: organizationID}
	for _, action := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/auth/accounts", nil},
		{http.MethodGet, "/auth/accounts/by-role/admin", nil},
		{http.MethodGet, "/auth/account", nil},
		{http.MethodGet, "/auth/permissions", nil},
		{http.MethodGet, fmt.Sprintf("/auth/accounts/%d/roles", accountID), nil},
		{http.MethodPut, fmt.Sprintf("/auth/accounts/%d", accountID), map[string]string{"email": "traeger-write@example.test"}},
	} {
		rr := e.execute(t, claims, action.method, action.path, action.body)
		assert.Equal(t, http.StatusUnauthorized, rr.Code, "%s %s: %s", action.method, action.path, rr.Body.String())
	}
}
