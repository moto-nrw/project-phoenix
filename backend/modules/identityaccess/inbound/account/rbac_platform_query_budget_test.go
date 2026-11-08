//go:build testhooks

package account_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// This is the actual operator issuer, not a tenant JWT with elevated flags.
// Its permission-less platform claims currently fail the shared catalogue
// guards. These denial budgets do not cover a successful AcrossSchools read.
func TestRBACPlatformCatalogueDenialsQueryBudget(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixture(t)
	operator := testpkg.CreateTestOperator(t, f.db)
	token, refresh, err := f.module.Auth.IssueTokensForAuthenticatedOperator(context.Background(), operator.ID, "", "")
	require.NoError(t, err)
	family := rbacPlatformClaims(t, token, operator.ID)
	stored, err := f.module.Auth.FindOperator(context.Background(), operator.ID)
	require.NoError(t, err)
	require.True(t, stored.Active)
	require.NotNil(t, stored.LastLogin, "the real issuer must stamp the persisted platform account")
	session, err := f.module.Auth.(ports.Records).LatestOperatorSessionInFamily(context.Background(), family)
	require.NoError(t, err)
	require.Equal(t, operator.ID, session.OperatorID)
	require.Zero(t, session.Generation)
	require.Nil(t, session.RotatedAt)

	var observations []api.TenantRequestEventForTests
	f.router = api.NewTenantRequestObservedRouterForTests(f.router, func(event api.TenantRequestEventForTests) {
		observations = append(observations, event)
	})

	system := testpkg.CreateTestSystemRole(t, f.db, "rbac-platform-system")
	foreign := testpkg.CreateTestRoleForTenant(t, f.db, "rbac-platform-foreign", tenant.FromContext(f.foreign))
	baseline := rbacPlatformState(t, f, operator.ID, family)
	require.NotEmpty(t, baseline.Catalogue.Permissions)
	systemNames := []string{}
	for _, role := range baseline.GlobalRoles {
		if role.IsSystem {
			systemNames = append(systemNames, role.Name)
		}
	}
	require.Subset(t, systemNames, []string{"admin", "user", "guardian", "lehrkraft", "guest", "ag-leitung", "schulverwaltung", system.Name})
	resource := fmt.Sprintf("rbac-platform-%d", f.actor)
	var addedRoles []identityaccess.Role
	var addedPermissions []identityaccess.Permission
	var roleMeasurements, permissionMeasurements [][]string
	for _, n := range []int{0, 3, 8} {
		for len(addedRoles) < n {
			ctx := testpkg.Ctx(t)
			schoolID := testpkg.Tenant(t)
			if len(addedRoles)%2 != 0 {
				ctx, schoolID = f.foreign, tenant.FromContext(f.foreign)
			}
			role := testpkg.CreateTestRoleForTenant(t, f.db, fmt.Sprintf("rbac-platform-growth-%d", len(addedRoles)), schoolID)
			saved, err := f.module.Auth.GetRole(ctx, role.ID)
			require.NoError(t, err)
			addedRoles = append(addedRoles, saved)
		}
		for len(addedPermissions) < n {
			action := fmt.Sprintf("read-%d", len(addedPermissions))
			addedPermissions = append(addedPermissions, f.permission(t, resource+":"+action, "Platform denial growth control", resource, action))
		}
		state := rbacPlatformState(t, f, operator.ID, family)
		require.Len(t, state.GlobalRoles, len(baseline.GlobalRoles)+n)
		require.Len(t, state.Catalogue.Permissions, len(baseline.Catalogue.Permissions)+n)
		require.Subset(t, state.GlobalRoles, baseline.GlobalRoles)
		require.Subset(t, state.GlobalRoles, addedRoles)
		require.Subset(t, state.Catalogue.Permissions, baseline.Catalogue.Permissions)
		require.Subset(t, state.Catalogue.Permissions, addedPermissions)
		t.Logf("additional_roles=%d additional_global_permissions=%d baseline_roles=%d baseline_permissions=%d", n, n, len(baseline.GlobalRoles), len(baseline.Catalogue.Permissions))
		for _, path := range []string{"/auth/roles/", "/auth/permissions/"} {
			rr, queries := rbacPlatformGet(t, f, path, token, operator.ID, family)
			rbacPlatformDenied(t, f, rr, observations)
			observations = nil
			if path == "/auth/roles/" {
				roleMeasurements = append(roleMeasurements, queries)
			} else {
				permissionMeasurements = append(permissionMeasurements, queries)
			}
		}
	}
	// Filters cannot turn an authenticated, permission-less operator into
	// a reader, including filters naming a foreign school's role.
	for _, path := range []string{
		"/auth/roles/?name=" + url.QueryEscape(strings.ToUpper(foreign.Name)),
		"/auth/roles/?name=" + url.QueryEscape(strings.ToUpper(system.Name)),
		"/auth/permissions/?resource=" + strings.ToUpper(resource),
		"/auth/permissions/?action=READ-0",
		"/auth/permissions/?resource=" + resource + "&action=READ-0",
	} {
		rr, _ := rbacPlatformGet(t, f, path, token, operator.ID, family)
		rbacPlatformDenied(t, f, rr, observations)
		observations = nil
	}
	// Production renewal re-mints the same permission-less operator claims.
	signing := f.module.SessionSigning
	codec, err := api.NewConfiguredSessionTokens(signing.Secret, signing.AccessExpiry, signing.RefreshExpiry)
	require.NoError(t, err)
	refreshClaims, err := codec.ParseRefreshToken(refresh)
	require.NoError(t, err)
	require.Equal(t, tenant.ScopePlatform, refreshClaims.Scope)
	require.Zero(t, refreshClaims.TenantID)
	renewed, nextRefresh, err := f.module.Auth.RefreshOperatorToken(context.Background(), refresh, "")
	require.NoError(t, err)
	require.NotEmpty(t, nextRefresh)
	require.Equal(t, family, rbacPlatformClaims(t, renewed, operator.ID))
	session, err = f.module.Auth.(ports.Records).LatestOperatorSessionInFamily(context.Background(), family)
	require.NoError(t, err)
	require.Equal(t, operator.ID, session.OperatorID)
	require.Equal(t, 1, session.Generation)
	for _, path := range []string{"/auth/roles/", "/auth/permissions/"} {
		rr, _ := rbacPlatformGet(t, f, path, renewed, operator.ID, family)
		rbacPlatformDenied(t, f, rr, observations)
		observations = nil
		for _, absent := range []string{"", "not-a-jwt"} {
			rr, queries := rbacPlatformGet(t, f, path, absent, operator.ID, family)
			rbacBudgetUnauthorized(t, rr)
			require.Empty(t, queries, "missing/invalid tokens must not open even the admin transaction")
			require.Empty(t, observations)
		}
	}
	// A real permitted tenant still reads the shared global catalogue, but
	// never the foreign custom role visible to an administrative store scope.
	rr, _ := f.get(t, "/auth/roles/?name="+url.QueryEscape(foreign.Name), f.token)
	rbacBudgetCollection(t, rr, "Roles retrieved successfully", []map[string]any{})
	f.successFrame(t, "auth.roles")
	rr, _ = f.get(t, "/auth/roles/?name="+url.QueryEscape(strings.ToUpper(addedRoles[0].Name)), f.token)
	rbacBudgetCollection(t, rr, "Roles retrieved successfully", []map[string]any{rbacBudgetRoleDTO(addedRoles[0])})
	f.successFrame(t, "auth.roles")
	permissions := make([]map[string]any, 0, len(addedPermissions))
	for _, permission := range addedPermissions {
		permissions = append(permissions, rbacBudgetPermissionDTO(permission.ID, permission.Name, permission.Description, permission.Resource, permission.Action, permission.CreatedAt, permission.UpdatedAt))
	}
	rr, _ = f.get(t, "/auth/permissions/?resource="+resource, f.token)
	rbacBudgetCollection(t, rr, "Permissions retrieved successfully", permissions)
	f.successFrame(t, "auth.permissions")
	for _, queries := range roleMeasurements {
		querybudgets.AssertQueryBudget(t, "api.auth.roles.list.platform_denied", queries)
	}
	for _, queries := range permissionMeasurements {
		querybudgets.AssertQueryBudget(t, "api.auth.permissions.list.platform_denied", queries)
	}
}

func rbacPlatformClaims(t *testing.T, token string, operatorID int64) string {
	t.Helper()
	claims, err := testutil.TestTokenAuth(t).ParseAccessJWT(token)
	require.NoError(t, err)
	require.Equal(t, operatorID, int64(claims.ID))
	require.Equal(t, tenant.ScopePlatform, claims.Scope)
	require.Zero(t, claims.TenantID)
	require.Zero(t, claims.OrgID)
	require.Equal(t, []string{"operator"}, claims.Roles)
	require.Empty(t, claims.Permissions)
	require.False(t, claims.IsAdmin)
	require.NotEmpty(t, claims.FamilyID)
	t.Logf("verified_operator=%d scope=%s tenant_id=%d roles=%v permissions=%v is_admin=%t", operatorID, claims.Scope, claims.TenantID, claims.Roles, claims.Permissions, claims.IsAdmin)
	return claims.FamilyID
}

type rbacPlatformReadback struct {
	Catalogue        rbacBudgetState
	GlobalRoles      []identityaccess.Role
	Operator         identityaccess.Operator
	Session          ports.OperatorSession
	PrivateStateJSON string `json:"-"`
}

func rbacPlatformState(t *testing.T, f *rbacBudgetFixture, operatorID int64, family string) rbacPlatformReadback {
	t.Helper()
	state := rbacPlatformReadback{Catalogue: f.state(t)}
	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), f.db, func(ctx context.Context) error {
		var err error
		state.GlobalRoles, err = f.module.Auth.ListRoles(ctx, identityaccess.RoleFilter{})
		return err
	}))
	var err error
	state.Operator, err = f.module.Auth.FindOperator(context.Background(), operatorID)
	require.NoError(t, err)
	state.Session, err = f.module.Auth.(ports.Records).LatestOperatorSessionInFamily(context.Background(), family)
	require.NoError(t, err)
	private, err := json.Marshal([]any{state.Operator, state.Session})
	require.NoError(t, err)
	state.PrivateStateJSON = string(private)
	// Keep the complete credential-bearing JSON only in unlogged memory.
	// Boolean equality below never formats it, and json:"-" excludes it
	// from the public evidence snapshots.
	state.Operator.PasswordHash = ""
	state.Session.Token, state.Session.TokenHash = "", ""
	state.Session.ReplacementTokenHash, state.Session.RecoveryProofHash = nil, nil
	return state
}

func rbacPlatformGet(t *testing.T, f *rbacBudgetFixture, path, token string, operatorID int64, family string) (*httptest.ResponseRecorder, []string) {
	t.Helper()
	before := rbacPlatformState(t, f, operatorID, family)
	req := testutil.NewJSONRequest(t, http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	f.counter.Reset()
	f.counter.Start()
	rr := testutil.ExecuteRequestForTest(t, f.router, req)
	f.counter.Stop()
	queries := f.counter.Queries()
	t.Logf("GET %s status=%d headers=%v body=%s SQL_count=%d driverDML=%d", path, rr.Code, rr.Header(), rr.Body.String(), len(queries), f.counter.WriteRows())
	for i, sql := range queries {
		t.Logf("SQL[%d]=%s", i, sql)
	}
	after := rbacPlatformState(t, f, operatorID, family)
	for _, snapshot := range []struct {
		name  string
		state rbacPlatformReadback
	}{{"before", before}, {"after", after}} {
		raw, err := json.Marshal(snapshot.state)
		require.NoError(t, err)
		t.Logf("%s=%s", snapshot.name, raw)
	}
	require.ElementsMatch(t, before.Catalogue.Roles, after.Catalogue.Roles)
	require.ElementsMatch(t, before.Catalogue.ForeignRoles, after.Catalogue.ForeignRoles)
	require.ElementsMatch(t, before.Catalogue.Permissions, after.Catalogue.Permissions)
	require.Equal(t, before.Catalogue.Mappings, after.Catalogue.Mappings)
	require.ElementsMatch(t, before.GlobalRoles, after.GlobalRoles)
	require.Equal(t, before.Operator, after.Operator)
	require.Equal(t, before.Session, after.Session)
	require.True(t, before.PrivateStateJSON == after.PrivateStateJSON, "credential-bearing state must also remain unchanged")
	for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
		require.Empty(t, f.counter.Operation(operation))
	}
	require.Zero(t, f.counter.WriteRows())
	return rr, queries
}

func rbacPlatformDenied(t *testing.T, f *rbacBudgetFixture, rr *httptest.ResponseRecorder, events []api.TenantRequestEventForTests) {
	t.Helper()
	rbacBudgetProblem(t, rr, http.StatusForbidden, "general.permission", "Forbidden", "anleitung-zugriff-pruefen")
	require.Len(t, events, 1)
	require.Equal(t, tenant.ScopePlatform, events[0].Scope)
	require.Zero(t, events[0].TenantID)
	require.Equal(t, http.StatusForbidden, events[0].Status)
	require.Equal(t, "commit", events[0].Outcome)
	require.Equal(t, []string{"BEGIN"}, f.counter.Operation("BEGIN"))
	require.Equal(t, []string{"SET LOCAL ROLE phoenix_admin"}, f.counter.Operation("SET"))
	require.Equal(t, []string{"COMMIT"}, f.counter.Operation("COMMIT"))
	require.Empty(t, f.counter.Operation("SELECT"), "the guard must reject before reading any catalogue or binding a tenant")
	require.Empty(t, f.counter.Operation("ROLLBACK"))
}
