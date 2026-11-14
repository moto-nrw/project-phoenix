package presence_test

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	presenceAPI "github.com/moto-ogs/project-phoenix/modules/studentpresence/inbound/presence"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// These fixtures use valid signed tenant claims through setupProtectedRouter,
// not a cold production login/JWT issuance or account-resolution measurement.
// Capture includes the production registration, authorizer and TenantTx work.
func TestCombinedListsRouteQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc, router := setupProtectedRouter(t)
	owner := testPresenceQueries(t, tc.db)
	ctx := testpkg.Ctx(t)
	foreign := combinationBudgetForeign(t, tc, owner)
	var open, ended []studentpresence.CombinedGroup
	futureEnd := time.Date(2100, 1, 2, 12, 0, 0, 0, time.UTC)
	var scheduled *studentpresence.CombinedGroup
	run := func() {
		active := append([]studentpresence.CombinedGroup{}, open...)
		if scheduled != nil {
			active = append(active, *scheduled)
		}
		cases := []struct {
			path string
			want []studentpresence.CombinedGroup
		}{
			{"/active/combined/", append(append([]studentpresence.CombinedGroup{}, active...), ended...)},
			{"/active/combined/?active=true", active},
			{"/active/combined/?active=1", active},
			{"/active/combined/?active=false", ended},
			{"/active/combined/?active=0", ended},
			{"/active/combined/active", open},
		}
		// Every list is ordered by identity, not by status or insertion bucket.
		for _, c := range cases {
			t.Run(fmt.Sprintf("%s/open%d", c.path, len(open)), func(t *testing.T) {
				want := append([]studentpresence.CombinedGroup{}, c.want...)
				slices.SortFunc(want, func(a, b studentpresence.CombinedGroup) int { return cmp.Compare(a.ID, b.ID) })
				rr, queries := combinationBudgetRequest(t, tc, router, c.path, []string{permissions.GroupsRead})
				if c.path == "/active/combined/active" {
					querybudgets.AssertQueryBudget(t, "api.active.combined.active", queries)
				} else {
					querybudgets.AssertQueryBudget(t, "api.active.combined.list", queries)
				}
				require.Equal(t, testutil.StatusOK, rr.Code, rr.Body.String())
				var body struct {
					Data []presenceAPI.CombinedGroupResponse `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
				require.NotNil(t, body.Data)
				require.Len(t, body.Data, len(want))
				for i, row := range body.Data {
					require.Equal(t, want[i].ID, row.ID)
					require.NotEqual(t, foreign.combined.ID, row.ID)
					require.Equal(t, want[i].EndTime, row.EndTime)
					require.Equal(t, want[i].EndTime == nil || want[i].EndTime.Equal(futureEnd), row.IsActive)
				}
			})
		}
	}
	run()
	// A future-ended combination is active=true, but not OpenOnly.
	future := combinationBudgetRecord(t, owner, ctx, &futureEnd)
	scheduled = &future
	pastEnd := time.Date(2001, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, n := range []int{3, 8} {
		for len(open) < n {
			open = append(open, combinationBudgetRecord(t, owner, ctx, nil))
		}
		for len(ended) < n {
			ended = append(ended, combinationBudgetRecord(t, owner, ctx, &pastEnd))
		}
		run()
	}
	for _, path := range []string{"/active/combined/", "/active/combined/active"} {
		rr, _ := combinationBudgetRequest(t, tc, router, path, []string{})
		testutil.AssertForbidden(t, rr)
	}
}

type combinationBudgetForeignRows struct {
	combined studentpresence.CombinedGroup
	group    *testpkg.ActiveGroupRow
}

func combinationBudgetRecord(t *testing.T, owner studentpresence.Owner, ctx context.Context, end *time.Time) studentpresence.CombinedGroup {
	t.Helper()
	row, err := owner.RecordCombination(ctx, time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC), end)
	require.NoError(t, err)
	return row
}

// Arrange through the same public owner command used by the mapping write
// route, then read the identity outside request measurement.
func combinationBudgetMapping(t *testing.T, owner studentpresence.Owner, ctx context.Context, combinedID, groupID int64) studentpresence.GroupMapping {
	t.Helper()
	require.NoError(t, owner.AddGroupToCombination(ctx, combinedID, groupID))
	rows, err := owner.ListGroupMappings(ctx, studentpresence.GroupMappingFilter{CombinedGroupID: &combinedID, ActiveGroupID: &groupID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	return rows[0]
}

func combinationBudgetForeign(t *testing.T, tc *testContext, owner studentpresence.Owner) combinationBudgetForeignRows {
	t.Helper()
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, tc.db, tenantID)
	group := testpkg.CreateTestActiveGroupForTenant(t, tc.db, tenantID)
	var rows combinationBudgetForeignRows
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), tc.db, tenantID, func(ctx context.Context) error {
		rows.combined = combinationBudgetRecord(t, owner, ctx, nil)
		rows.group = group
		return owner.AddGroupToCombination(ctx, rows.combined.ID, group.ID)
	}))
	return rows
}

func TestCombinedMappingsRouteQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc, router := setupProtectedRouter(t)
	owner := testPresenceQueries(t, tc.db)
	ctx := testpkg.Ctx(t)
	foreign := combinationBudgetForeign(t, tc, owner)
	combined := combinationBudgetRecord(t, owner, ctx, nil)
	unrelated := combinationBudgetRecord(t, owner, ctx, nil)
	unrelatedGroup := testpkg.CreateTestActiveGroupForTenant(t, tc.db, testpkg.Tenant(t))
	require.NoError(t, owner.AddGroupToCombination(ctx, unrelated.ID, unrelatedGroup.ID))
	path := fmt.Sprintf("/active/mappings/combined/%d", combined.ID)
	var want []presenceAPI.GroupMappingResponse
	run := func(path string, want []presenceAPI.GroupMappingResponse) {
		queries := assertCombinationBudgetMappings(t, tc, router, path, want)
		querybudgets.AssertQueryBudget(t, "api.active.mappings.combined", queries)
	}
	run(path, want)
	for _, n := range []int{3, 8} {
		for len(want) < n {
			group := testpkg.CreateTestActiveGroupForTenant(t, tc.db, testpkg.Tenant(t))
			mapping := combinationBudgetMapping(t, owner, ctx, combined.ID, group.ID)
			want = append(want, presenceAPI.GroupMappingResponse{ID: mapping.ID, ActiveGroupID: group.ID, CombinedGroupID: combined.ID})
		}
		run(path, want)
	}
	run(fmt.Sprintf("/active/mappings/combined/%d", foreign.combined.ID), nil)
	missing := combinationBudgetRecord(t, owner, ctx, nil)
	require.NoError(t, owner.DeleteCombination(ctx, missing.ID))
	run(fmt.Sprintf("/active/mappings/combined/%d", missing.ID), nil)
	combinationBudgetErrorControls(t, tc, router, "/active/mappings/combined/invalid", path)
}

func TestGroupMappingsRouteQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc, router := setupProtectedRouter(t)
	owner := testPresenceQueries(t, tc.db)
	ctx := testpkg.Ctx(t)
	foreign := combinationBudgetForeign(t, tc, owner)
	group := testpkg.CreateTestActiveGroupForTenant(t, tc.db, testpkg.Tenant(t))
	unrelatedGroup := testpkg.CreateTestActiveGroupForTenant(t, tc.db, testpkg.Tenant(t))
	path := fmt.Sprintf("/active/mappings/group/%d", group.ID)
	var want []presenceAPI.GroupMappingResponse
	run := func(path string, want []presenceAPI.GroupMappingResponse) {
		queries := assertCombinationBudgetMappings(t, tc, router, path, want)
		querybudgets.AssertQueryBudget(t, "api.active.mappings.group", queries)
	}
	run(path, want)
	current := combinationBudgetRecord(t, owner, ctx, nil)
	mapping := combinationBudgetMapping(t, owner, ctx, current.ID, group.ID)
	want = append(want, presenceAPI.GroupMappingResponse{ID: mapping.ID, ActiveGroupID: group.ID, CombinedGroupID: current.ID})
	run(path, want)
	pastEnd := time.Date(2001, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, n := range []int{3, 8} {
		for len(want) < n {
			// Distinct, ended combinations are legal history, not duplicate pairs.
			historical := combinationBudgetRecord(t, owner, ctx, &pastEnd)
			mapping := combinationBudgetMapping(t, owner, ctx, historical.ID, group.ID)
			want = append(want, presenceAPI.GroupMappingResponse{ID: mapping.ID, ActiveGroupID: group.ID, CombinedGroupID: historical.ID})
			require.NoError(t, owner.AddGroupToCombination(ctx, historical.ID, unrelatedGroup.ID))
		}
		run(path, want)
	}
	run(fmt.Sprintf("/active/mappings/group/%d", foreign.group.ID), nil)
	missing := testpkg.CreateTestActiveGroupForTenant(t, tc.db, testpkg.Tenant(t))
	require.NoError(t, owner.DeleteGroup(ctx, missing.ID))
	run(fmt.Sprintf("/active/mappings/group/%d", missing.ID), nil)
	combinationBudgetErrorControls(t, tc, router, "/active/mappings/group/invalid", path)
}

func assertCombinationBudgetMappings(t *testing.T, tc *testContext, router testutil.Router, path string, want []presenceAPI.GroupMappingResponse) testpkg.QueryMeasurement {
	t.Helper()
	rr, queries := combinationBudgetRequest(t, tc, router, path, []string{permissions.GroupsRead})
	require.Equal(t, testutil.StatusOK, rr.Code, rr.Body.String())
	var body struct {
		Data []presenceAPI.GroupMappingResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.NotNil(t, body.Data)
	require.Equal(t, append([]presenceAPI.GroupMappingResponse{}, want...), body.Data, "exact mapping identities and ID order")
	t.Logf("verified GET %s matching mappings=%d", path, len(want))
	return queries
}

func combinationBudgetErrorControls(t *testing.T, tc *testContext, router testutil.Router, malformed, valid string) {
	t.Helper()
	rr, _ := combinationBudgetRequest(t, tc, router, malformed, []string{permissions.GroupsRead})
	testutil.AssertBadRequest(t, rr)
	rr, _ = combinationBudgetRequest(t, tc, router, valid, []string{})
	testutil.AssertForbidden(t, rr)
}

func combinationBudgetRequest(t *testing.T, tc *testContext, router testutil.Router, path string, perms []string) (*httptest.ResponseRecorder, testpkg.QueryMeasurement) {
	t.Helper()
	before := combinationBudgetState(t, tc)
	counter := testpkg.CaptureQueries(t, tc.db)
	rr := testutil.ExecuteWithAuthPermissions(t, testpkg.MountedQueryBudgetRouter("/api", router), testutil.NewRequest("GET", path, nil), testutil.AdminTestClaims(1), perms)
	counter.Stop()
	queries := counter.Queries()
	require.Zero(t, counter.WriteRows(), "no successful request DML, including rolled-back writes")
	for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
		require.Empty(t, counter.Operation(operation), "no attempted request %s", operation)
	}
	require.Equal(t, before, combinationBudgetState(t, tc), "GET must preserve rows, timestamps and sequences in its private clone")
	t.Logf("measured GET %s status=%d statements=%d", path, rr.Code, len(queries))
	if os.Getenv("PHX_QUERY_DUMP") == "1" {
		for i, sql := range queries {
			t.Logf("complete SQL %d: %s", i+1, sql)
		}
	}
	return rr, counter.Measurement(queries)
}

func combinationBudgetState(t *testing.T, tc *testContext) string {
	t.Helper()
	var snapshot string
	// Isolated clone: include foreign rows and every schema's sequence, not only
	// result counts. This observer is outside the request's CaptureQueries scope.
	require.NoError(t, tc.db.NewRaw(`SELECT jsonb_build_object(
 'combinations', (SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY id),'[]'::jsonb) FROM active.combined_groups r),
 'mappings', (SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY id),'[]'::jsonb) FROM active.group_mappings r),
 'groups', (SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY id),'[]'::jsonb) FROM active.groups r),
 'sequences', (SELECT jsonb_agg(to_jsonb(s) ORDER BY schemaname,sequencename) FROM pg_sequences s)
 )::text`).Scan(testpkg.Ctx(t), &snapshot))
	return snapshot
}
