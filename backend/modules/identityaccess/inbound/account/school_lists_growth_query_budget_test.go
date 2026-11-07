//go:build testhooks

package account_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPublicSchoolListsGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	schoolListsGrowthBudget(t, false)
}

func TestAccountSchoolListsGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	schoolListsGrowthBudget(t, true)
}

func schoolListsGrowthBudget(t *testing.T, accountList bool) {
	t.Helper()
	// The public catalog is global. Hide baseline rows only in this test's
	// disposable clone, never in the package clone or shared template.
	db := testpkg.SetupIsolatedTestDB(t)
	module := setupAuthModuleOn(t, db)
	database := testpkg.NewDatabase(t, db)
	unit := testpkg.TenantRuntime(t, db)
	resource := authAPI.NewResource(module.Auth, module.Invitation,
		api.NewAuthSchoolDirectory(api.NewOrganizationTestRows(database).School, module.Auth),
		module.AccountAuthentication, identityCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation,
		api.NewAuthTestSettings(database, unit))
	// Same protected account-resource/session/tenant runtime as the existing
	// account tests. Serve host routing and session-family liveness are not mounted.
	router := testutil.NewTenantRouter(db)
	router.Mount("/auth", resource.Router(api.TenantSessionChain))
	actor := testpkg.CreateTestAccount(t, db, "school-list-reader")
	testpkg.EnsureAccountTenant(t, db, actor.ID, testpkg.Tenant(t))
	token, _, err := module.Auth.IssueTokensForAuthenticatedAccount(testpkg.Ctx(t), actor.ID, testpkg.Tenant(t), "", "")
	require.NoError(t, err)
	other := testpkg.CreateTestAccount(t, db, "school-list-other")
	_, err = db.ExecContext(context.Background(), "UPDATE platform.schools SET hidden = true")
	require.NoError(t, err)
	// The session's school is not an operational result. Its real persisted
	// membership and issued JWT still exercise the unchanged tenant runtime.
	_, err = db.ExecContext(context.Background(), "UPDATE platform.schools SET active = false WHERE id = ?", testpkg.Tenant(t))
	require.NoError(t, err)

	createSchool := func(name string, member int64, hidden bool) (int64, map[string]any) {
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger " + name, Slug: "org-" + name, Active: true})
		id := testpkg.CreateTestSchoolInOrganization(t, db, organization.ID, "Schule "+name)
		if member != 0 {
			testpkg.EnsureAccountTenant(t, db, member, id)
		}
		_, updateErr := db.ExecContext(context.Background(), "UPDATE platform.schools SET hidden = ? WHERE id = ?", hidden, id)
		require.NoError(t, updateErr)
		var school struct {
			Slug      string
			Subdomain string
		}
		require.NoError(t, db.NewSelect().TableExpr("platform.schools").Column("slug", "subdomain").Where("id = ?", id).Scan(context.Background(), &school))
		dto := map[string]any{"slug": school.Slug, "name": "Schule " + name, "subdomain": school.Subdomain, "organization_name": organization.Name}
		if accountList {
			dto["tenant_id"] = float64(id)
			dto["organization_id"] = float64(organization.ID)
		}
		t.Logf("persisted school id=%d organization_id=%d hidden=%v member=%d DTO=%v", id, organization.ID, hidden, member, dto)
		return id, dto
	}
	// Hidden operational schools ARE available to members, but not public.
	_, hiddenDTO := createSchool("hidden-decoy", actor.ID, true)
	inactive, _ := createSchool("inactive-decoy", actor.ID, false)
	deleted, _ := createSchool("deleted-decoy", actor.ID, false)
	_, err = db.ExecContext(context.Background(), "UPDATE platform.schools SET active = false WHERE id = ?", inactive)
	require.NoError(t, err)
	testpkg.SoftDeleteTestSchool(t, db, deleted)
	if accountList {
		inactiveMember, _ := createSchool("inactive-membership-decoy", actor.ID, false)
		_, err = db.ExecContext(context.Background(), "UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?", actor.ID, inactiveMember)
		require.NoError(t, err)
		_, _ = createSchool("other-account-decoy", other.ID, false)
		_, _ = createSchool("no-membership-decoy", 0, false)
	}
	want := []map[string]any{}
	if accountList {
		// Include the hidden operational membership as one of the N positives.
		want = append(want, hiddenDTO)
	}
	path, message := "/auth/tenants", "Tenants retrieved successfully"
	if accountList {
		path, message = "/auth/account/tenants", "Account tenants retrieved successfully"
	}
	counter := testpkg.CaptureQueries(t, db)
	counter.Stop()
	for _, n := range []int{3, 8} {
		for len(want) < n {
			_, dto := createSchool(fmt.Sprintf("positive-%d", len(want)), actor.ID, false)
			want = append(want, dto)
		}
		before := schoolListsState(t, db)
		req := testutil.NewRequest(http.MethodGet, path, nil)
		if accountList {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		counter.Reset()
		counter.Start()
		rr := testutil.ExecuteRequestForTest(t, router, req)
		counter.Stop()
		queries := counter.Queries()
		rbacBudgetCollection(t, rr, message, want)
		var envelope struct {
			Data []map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
		require.Len(t, envelope.Data, n)
		require.Equal(t, want, envelope.Data, "preserve the catalog name ordering")
		if accountList {
			querybudgets.AssertQueryBudget(t, "api.auth.account.tenants.positive_growth", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "api.auth.tenants.positive_growth", queries)
		}
		require.Equal(t, before, schoolListsState(t, db), "GET preserves rows and sequence state")
		require.Zero(t, counter.WriteRows())
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
			require.Empty(t, counter.Operation(operation))
		}
		t.Logf("GET %s N%d rows=%d statements=%d body=%s", path, n, n, len(queries), rr.Body.String())
		for i, sql := range queries {
			t.Logf("SQL[%d]=%s", i, sql)
		}
	}
	if accountList {
		rr := testutil.ExecuteRequestForTest(t, router, testutil.NewRequest(http.MethodGet, path, nil))
		rbacBudgetUnauthorized(t, rr)
	}
}

func schoolListsState(t *testing.T, db *bun.DB) map[string]string {
	t.Helper()
	state := map[string]string{}
	for _, table := range []string{"platform.schools", "platform.organizations", "auth.accounts", "auth.account_tenants", "auth.tokens", "auth.account_roles"} {
		var rows []json.RawMessage
		require.NoError(t, db.NewRaw("SELECT to_jsonb(r) FROM "+table+" AS r ORDER BY to_jsonb(r)::text").Scan(context.Background(), &rows))
		raw, err := json.Marshal(rows)
		require.NoError(t, err)
		state[table] = string(raw)
	}
	var sequences []json.RawMessage
	require.NoError(t, db.NewRaw("SELECT to_jsonb(s) FROM pg_sequences AS s ORDER BY schemaname, sequencename").Scan(context.Background(), &sequences))
	raw, err := json.Marshal(sequences)
	require.NoError(t, err)
	state["sequences"] = string(raw)
	return state
}
