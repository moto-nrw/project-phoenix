//go:build testhooks

package account_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

func TestIdentityRoleDetailPermissionsGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixtureOn(t, testpkg.SetupIsolatedTestDB(t))
	roleFixture := testpkg.CreateTestRole(t, f.db, "identity-detail-budget")
	role, err := f.module.AccountAuthentication.GetRole(testpkg.Ctx(t), roleFixture.ID)
	require.NoError(t, err)
	f.roleContexts[role.ID] = testpkg.Ctx(t)
	want := rbacBudgetRoleDTO(role)
	names := []string{}
	for _, n := range []int{3, 8} {
		for len(names) < n {
			name := fmt.Sprintf("identity-detail-%d:read-%d", role.ID, len(names))
			permission := f.permission(t, name, "Role detail growth", fmt.Sprintf("identity-detail-%d", role.ID), fmt.Sprintf("read-%d", len(names)))
			require.NoError(t, f.module.AccountAuthentication.AssignPermissionToRole(testpkg.Ctx(t), role.ID, permission.ID))
			names = append(names, name)
		}
		rr, queries := f.get(t, fmt.Sprintf("/auth/roles/%d/", role.ID), f.token)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var envelope struct {
			Status  string         `json:"status"`
			Message string         `json:"message"`
			Data    map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
		require.Equal(t, "success", envelope.Status)
		require.Equal(t, "Role retrieved successfully", envelope.Message)
		permissions := envelope.Data["permissions"]
		delete(envelope.Data, "permissions")
		require.Equal(t, want, envelope.Data)
		raw, err := json.Marshal(permissions)
		require.NoError(t, err)
		var got []string
		require.NoError(t, json.Unmarshal(raw, &got))
		require.ElementsMatch(t, names, got)
		t.Logf("identity role detail N%d statements=%d", n, len(queries.Statements))
		querybudgets.AssertQueryBudget(t, "api.auth.roles.detail.permissions_growth", queries)
	}
}

func TestIdentityAccountsGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixtureOn(t, testpkg.SetupIsolatedTestDB(t))
	token, reader := f.accountToken(t, "identity-accounts-reader", "users:list", "users:read")
	role := testpkg.CreateTestRole(t, f.db, "identity-account-budget")
	// Include both real readers in the same role filter, so each route
	// returns exactly N3/N8 rows, not N3/N8 plus hidden baseline accounts.
	var baseline []struct {
		ID    int64  `bun:"id"`
		Email string `bun:"email"`
	}
	require.NoError(t, f.db.NewSelect().TableExpr("auth.accounts").Column("id", "email").Where("id = ? OR id = ?", f.actor, reader).Scan(context.Background(), &baseline))
	all := []map[string]any{}
	for _, row := range baseline {
		all = append(all, map[string]any{"id": float64(row.ID), "email": row.Email, "active": true})
	}
	for _, id := range []int64{f.actor, reader} {
		_, err := f.db.ExecContext(context.Background(), "INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)", id, role.ID, testpkg.Tenant(t))
		require.NoError(t, err)
	}
	for _, n := range []int{3, 8} {
		for len(all) < n {
			account := testpkg.CreateTestAccount(t, f.db, fmt.Sprintf("identity-account-%d", len(all)))
			testpkg.EnsureAccountTenant(t, f.db, account.ID, testpkg.Tenant(t))
			_, err := f.db.ExecContext(context.Background(), "INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)", account.ID, role.ID, testpkg.Tenant(t))
			require.NoError(t, err)
			dto := map[string]any{"id": float64(account.ID), "email": account.Email, "active": true}
			all = append(all, dto)
		}
		for _, tc := range []struct {
			path, scenario string
			rows           []map[string]any
		}{
			{"/auth/accounts/", "api.auth.accounts.list.positive_growth", all},
			{"/auth/accounts/by-role/" + role.Name, "api.auth.accounts.by_role.positive_growth", all},
		} {
			rr, queries := f.get(t, tc.path, token)
			rbacBudgetCollection(t, rr, "Accounts retrieved successfully", tc.rows)
			t.Logf("%s N%d statements=%d", tc.scenario, n, len(queries.Statements))
			if tc.scenario == "api.auth.accounts.list.positive_growth" {
				querybudgets.AssertQueryBudget(t, "api.auth.accounts.list.positive_growth", queries)
			} else {
				querybudgets.AssertQueryBudget(t, "api.auth.accounts.by_role.positive_growth", queries)
			}
		}
	}
}

func TestIdentityStaffPreviewCandidatesGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixtureOn(t, testpkg.SetupIsolatedTestDB(t))
	token, _ := f.accountToken(t, "identity-preview-reader", "admin:*")
	// The unrelated RBAC helper actor has a role and is therefore an eligible
	// candidate too. Remove only this test-owned actor's mapping before measuring.
	_, err := f.db.ExecContext(context.Background(), "DELETE FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?", f.actor, testpkg.Tenant(t))
	require.NoError(t, err)
	want := []map[string]any{}
	for _, n := range []int{3, 8} {
		for len(want) < n {
			first := fmt.Sprintf("Preview%d", len(want))
			_, account := testpkg.CreateTestCalendarStaff(t, f.db, first, "Budget")
			want = append(want, map[string]any{"account_id": fmt.Sprint(account.ID), "first_name": first, "last_name": "Budget", "email": account.Email, "roles": []any{"user"}})
		}
		rr, queries := f.get(t, "/auth/staff-preview/candidates", token)
		rbacBudgetCollection(t, rr, "Preview candidates retrieved", want)
		t.Logf("identity preview N%d statements=%d", n, len(queries.Statements))
		querybudgets.AssertQueryBudget(t, "api.auth.staff_preview.candidates.positive_growth", queries)
	}
}

func TestIdentityAccountTokensGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixtureOn(t, testpkg.SetupIsolatedTestDB(t))
	token, _ := f.accountToken(t, "identity-token-reader", "users:manage")
	account := testpkg.CreateTestAccount(t, f.db, "identity-token-owner")
	want := []map[string]any{}
	for _, n := range []int{3, 8} {
		for len(want) < n {
			fixture := testpkg.CreateTestToken(t, f.db, account.ID, "refresh")
			var created, expiry time.Time
			require.NoError(t, f.db.NewSelect().TableExpr("auth.tokens").Column("created_at", "expiry").Where("id = ?", fixture.ID).Scan(context.Background(), &created, &expiry))
			want = append(want, map[string]any{"id": float64(fixture.ID), "expiry": expiry.Format(time.RFC3339), "mobile": false, "created_at": created.Format(time.RFC3339)})
		}
		rr, queries := f.get(t, fmt.Sprintf("/auth/accounts/%d/tokens/", account.ID), token)
		rbacBudgetCollection(t, rr, "Active tokens retrieved successfully", want)
		t.Logf("identity tokens N%d statements=%d", n, len(queries.Statements))
		querybudgets.AssertQueryBudget(t, "api.auth.accounts.tokens.positive_growth", queries)
	}
}

func TestIdentityAccountPasskeysGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixtureOn(t, testpkg.SetupIsolatedTestDB(t))
	f.resource.PasskeyService = f.module.Passkeys
	owner, err := identityCompose.New(identityCompose.Dependencies{DB: f.db, TenantRuntime: testpkg.TenantRuntime(t, f.db), Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	records := owner.(ports.Records)
	want := []identityaccess.PasskeyCredentialSummary{}
	for _, n := range []int{3, 8} {
		for len(want) < n {
			name := fmt.Sprintf("Budget key %d", len(want))
			row, err := records.CreateAccountPasskey(testpkg.Ctx(t), identityaccess.AccountPasskeyCredential{AccountID: f.actor, UserHandle: []byte(fmt.Sprintf("budget-handle-%d", f.actor)), CredentialID: []byte(fmt.Sprintf("budget-credential-%d-%d", f.actor, len(want))), CredentialJSON: json.RawMessage(`{"id":"fixture"}`), Name: name})
			require.NoError(t, err)
			want = append(want, identityaccess.PasskeyCredentialSummary{ID: fmt.Sprint(row.ID), Name: name, CreatedAt: row.CreatedAt})
		}
		rr, queries := f.get(t, "/auth/passkeys/", f.token)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		raw, err := json.Marshal(want)
		require.NoError(t, err)
		require.JSONEq(t, string(raw), rr.Body.String())
		t.Logf("identity passkeys N%d statements=%d", n, len(queries.Statements))
		querybudgets.AssertQueryBudget(t, "api.auth.passkeys.positive_growth", queries)
	}
}

func TestIdentityTrustedDevicesGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixtureOn(t, testpkg.SetupIsolatedTestDB(t))
	f.resource.MFAService = f.module.MFA
	want := []map[string]any{}
	created := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC)
	expires := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	for _, n := range []int{3, 8} {
		for len(want) < n {
			agent := fmt.Sprintf("Budget browser %d", len(want))
			var id int64
			require.NoError(t, f.db.NewRaw("INSERT INTO auth.mfa_trusted_devices (account_id, tenant_id, token_hash, user_agent, ip_address, created_at, expires_at) VALUES (?, ?, ?, ?, '127.0.0.1', ?, ?) RETURNING id", f.actor, testpkg.Tenant(t), fmt.Sprintf("example-token-hash-fixture-%d", len(want)), agent, created, expires).Scan(context.Background(), &id))
			want = append(want, map[string]any{"id": float64(id), "user_agent": agent, "ip_address": "127.0.0.1", "created_at": created.Format(time.RFC3339), "expires_at": expires.Format(time.RFC3339)})
		}
		rr, queries := f.get(t, "/auth/mfa/trusted-devices", f.token)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		raw, err := json.Marshal(want)
		require.NoError(t, err)
		var rows []map[string]any
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &rows))
		require.ElementsMatch(t, want, rows, string(raw))
		t.Logf("identity trusted devices N%d statements=%d", n, len(queries.Statements))
		querybudgets.AssertQueryBudget(t, "api.auth.mfa.trusted_devices.positive_growth", queries)
	}
}
