//go:build testhooks

package account_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Only the protected /auth resource is mounted. The Serve-host verifier's
// session-family liveness check and host routing are deliberately not claimed.
type rbacBudgetFixture struct {
	db           *bun.DB
	module       api.AuthTestModule
	resource     *authAPI.Resource
	router       chi.Router
	counter      *testpkg.QueryCounter
	token        string
	actor        int64
	roleContexts map[int64]context.Context
	foreign      context.Context
}

func newRBACBudgetFixture(t *testing.T) *rbacBudgetFixture {
	t.Helper()
	return newRBACBudgetFixtureOn(t, testpkg.SetupIsolatedTestDB(t))
}

func newRBACBudgetFixtureOn(t *testing.T, db *bun.DB) *rbacBudgetFixture {
	t.Helper()
	module := setupAuthModuleOn(t, db)
	settings := api.NewAuthTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	resource := authAPI.NewResource(module.Auth, module.Invitation,
		testSchoolDirectory{schools: api.NewOrganizationTestRows(testpkg.NewDatabase(t, db)).School, db: db, runtime: testpkg.TenantRuntime(t, db)},
		module.AccountAuthentication, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, settings)
	resource.RoleGrants = api.AccountRoleGrantPolicy()
	router := testutil.NewTenantRouter(db)
	router.Mount("/auth", resource.Router(api.TenantSessionChain))
	f := &rbacBudgetFixture{db: db, module: module, router: router, resource: resource, roleContexts: make(map[int64]context.Context)}
	f.token, f.actor = f.accountToken(t, "rbac-budget-reader", "roles:read", "permissions:read", "roles:manage")
	f.foreign = testpkg.NewTenantScope(t, db).Context()
	f.counter = testpkg.CaptureQueries(t, db)
	f.counter.Stop()
	return f
}

func (f *rbacBudgetFixture) accountToken(t *testing.T, name string, permissions ...string) (string, int64) {
	t.Helper()
	actor := testpkg.CreateTestAccount(t, f.db, name)
	testpkg.EnsureAccountTenant(t, f.db, actor.ID, testpkg.Tenant(t))
	for _, permission := range permissions {
		resource, action, ok := strings.Cut(permission, ":")
		require.True(t, ok)
		existing, err := f.module.AccountAuthentication.ListPermissions(testpkg.Ctx(t), identityaccess.PermissionFilter{Resource: resource, Action: action})
		require.NoError(t, err)
		if len(existing) == 0 {
			f.permission(t, permission, "RBAC budget guard", resource, action)
		}
		testpkg.GrantTestPermission(t, f.db, testpkg.Tenant(t), actor.ID, permission)
	}
	// The module loads the real persisted role grants, rather than accepting
	// invented permissions in the successful request's signed claims.
	token, _, err := f.module.Auth.IssueTokensForPasskey(testpkg.Ctx(t), actor.ID, testpkg.Tenant(t), "", "")
	require.NoError(t, err)
	claims, err := testutil.TestTokenAuth(t).ParseAccessJWT(token)
	require.NoError(t, err)
	require.Equal(t, actor.ID, int64(claims.ID))
	require.Equal(t, testpkg.Tenant(t), claims.TenantID)
	require.False(t, claims.IsAdmin, "the guard must evaluate actual specific permissions, not an admin bypass")
	require.ElementsMatch(t, permissions, claims.Permissions)
	return token, actor.ID
}

type rbacBudgetState struct {
	Roles        []identityaccess.Role
	ForeignRoles []identityaccess.Role
	Permissions  []identityaccess.Permission
	Mappings     map[int64][]identityaccess.Permission
}

func (f *rbacBudgetFixture) state(t *testing.T) rbacBudgetState {
	t.Helper()
	var state rbacBudgetState
	var err error
	state.Roles, err = f.module.AccountAuthentication.ListRoles(testpkg.Ctx(t), identityaccess.RoleFilter{})
	require.NoError(t, err)
	state.ForeignRoles, err = f.module.AccountAuthentication.ListRoles(f.foreign, identityaccess.RoleFilter{})
	require.NoError(t, err)
	state.Permissions, err = f.module.AccountAuthentication.ListPermissions(testpkg.Ctx(t), identityaccess.PermissionFilter{})
	require.NoError(t, err)
	state.Mappings = make(map[int64][]identityaccess.Permission)
	for id, ctx := range f.roleContexts {
		state.Mappings[id], err = f.module.AccountAuthentication.GetRolePermissions(ctx, id)
		require.NoError(t, err)
	}
	return state
}

func (f *rbacBudgetFixture) get(t *testing.T, path, token string) (*httptest.ResponseRecorder, testpkg.QueryMeasurement) {
	t.Helper()
	before := f.state(t)
	req := testutil.NewJSONRequest(t, http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	f.counter.Reset()
	f.counter.Start()
	rr := testutil.ExecuteRequestForTest(t, f.router, req)
	f.counter.Stop()
	queries := f.counter.Queries()
	t.Logf("GET %s status=%d headers=%v body=%s SQL_count=%d", path, rr.Code, rr.Header(), rr.Body.String(), len(queries))
	for i, sql := range queries {
		t.Logf("SQL[%d]=%s", i, sql)
	}
	after := f.state(t)
	raw, err := json.Marshal(after)
	require.NoError(t, err)
	t.Logf("readback=%s", raw)
	for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
		require.Empty(t, f.counter.Operation(operation), "GET must not execute DML")
	}
	require.Zero(t, f.counter.WriteRows())
	// These stores promise no order. Check complete row values as multisets,
	// never sort the HTTP array or assert an invented name/ID ordering.
	require.ElementsMatch(t, before.Roles, after.Roles)
	require.ElementsMatch(t, before.ForeignRoles, after.ForeignRoles)
	require.ElementsMatch(t, before.Permissions, after.Permissions)
	require.Len(t, after.Mappings, len(before.Mappings))
	for id, rows := range before.Mappings {
		require.ElementsMatch(t, rows, after.Mappings[id])
	}
	return rr, f.counter.Measurement(queries)
}

func rbacBudgetCollection(t *testing.T, rr *httptest.ResponseRecorder, message string, want []map[string]any) {
	t.Helper()
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, http.Header{"Content-Type": []string{"application/json"}}, rr.Header())
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
	require.Len(t, envelope, 3)
	require.JSONEq(t, `"success"`, string(envelope["status"]))
	expectedMessage, err := json.Marshal(message)
	require.NoError(t, err)
	require.JSONEq(t, string(expectedMessage), string(envelope["message"]))
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(envelope["data"], &rows))
	require.NotNil(t, rows, "collections must encode [] instead of null")
	require.ElementsMatch(t, want, rows, "exact fields, string IDs and membership; store order is unspecified")
}

func rbacBudgetRoleDTO(role identityaccess.Role) map[string]any {
	dto := map[string]any{"id": strconv.FormatInt(role.ID, 10), "name": role.Name, "description": role.Description,
		"is_system": role.IsSystem, "school_portal": role.SchoolPortal,
		"created_at": role.CreatedAt.Format(time.RFC3339), "updated_at": role.UpdatedAt.Format(time.RFC3339)}
	if role.BaseRole != nil {
		dto["base_role"] = *role.BaseRole
	}
	return dto
}

func rbacBudgetPermissionDTO(id int64, name, description, resource, action string, created, updated time.Time) map[string]any {
	return map[string]any{"id": strconv.FormatInt(id, 10), "name": name, "description": description,
		"resource": resource, "action": action, "created_at": created.Format(time.RFC3339), "updated_at": updated.Format(time.RFC3339)}
}

func (f *rbacBudgetFixture) permission(t *testing.T, name, description, resource, action string) identityaccess.Permission {
	t.Helper()
	var created identityaccess.Permission
	require.NoError(t, testpkg.WithinAdminContext(t, testpkg.Ctx(t), f.db, func(ctx context.Context) error {
		var err error
		created, err = f.module.AccountAuthentication.CreatePermission(ctx, name, description, resource, action)
		return err
	}))
	testpkg.OwnTestPermission(t, f.db, created.ID)
	stored, err := f.module.AccountAuthentication.GetPermission(testpkg.Ctx(t), created.ID)
	require.NoError(t, err)
	require.Equal(t, name, stored.Name)
	require.Equal(t, description, stored.Description)
	require.Equal(t, resource, stored.Resource)
	require.Equal(t, action, stored.Action)
	require.False(t, stored.CreatedAt.IsZero())
	require.False(t, stored.UpdatedAt.IsZero())
	return stored
}

func rbacBudgetProblem(t *testing.T, rr *httptest.ResponseRecorder, status int, code, detail, anchor string) {
	t.Helper()
	require.Equal(t, status, rr.Code, rr.Body.String())
	require.Equal(t, http.Header{"Content-Type": []string{"application/json"}}, rr.Header())
	expected, err := json.Marshal(map[string]any{"type": "https://moto-app.de/help/fehlermeldungen#" + anchor,
		"title": http.StatusText(status), "status": status, "detail": detail, "instance": "", "code": code})
	require.NoError(t, err)
	require.JSONEq(t, string(expected), rr.Body.String())
}

func rbacBudgetUnauthorized(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
	require.Equal(t, http.Header{"Content-Type": []string{"application/json"}}, rr.Header())
	// Without Serve-host error-renderer binding this token adapter answers its
	// own minimal coded envelope. Pin the actual resource scope, not a fabricated
	// whole-server RFC envelope.
	require.JSONEq(t, `{"status":401,"detail":"token unauthorized","code":"general.permission"}`, rr.Body.String())
}

func (f *rbacBudgetFixture) successFrame(t *testing.T, table string) {
	t.Helper()
	require.Equal(t, []string{"BEGIN"}, f.counter.Operation("BEGIN"))
	require.Equal(t, []string{"COMMIT"}, f.counter.Operation("COMMIT"))
	frame := f.counter.Matching(func(sql string) bool {
		return strings.Contains(sql, "set_config('role', 'phoenix_tenant', true)") &&
			strings.Contains(sql, "set_config('app.current_tenant_id', '"+strconv.FormatInt(testpkg.Tenant(t), 10)+"', true)")
	})
	require.Len(t, frame, 1, "measure the real tenant role and RLS scope, not just handler SQL")
	require.Len(t, f.counter.Selects(table), 1)
}
