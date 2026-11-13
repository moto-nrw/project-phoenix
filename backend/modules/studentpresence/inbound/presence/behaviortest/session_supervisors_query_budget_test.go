package presence_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	presenceAPI "github.com/moto-ogs/project-phoenix/modules/studentpresence/inbound/presence"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// TestSessionSupervisorsQueryBudget covers /groups/{id}/supervisors, whose
// existence gate differs from /supervisors/group/{groupId}.
func TestSessionSupervisorsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.OwnTenant(t)
	db := testpkg.SetupIsolatedTestDB(t)
	tc, router := setupProtectedRouter(t)
	require.Same(t, db, tc.db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	account := testpkg.CreateTestAccount(t, db, "session-supervisors-budget")
	room := testpkg.CreateTestRoom(t, db, "SessionSupervisorsBudgetRoom")
	activity := testpkg.CreateTestActivityGroup(t, db, "SessionSupervisorsBudgetActivity")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	otherGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	endedGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	// Parent existence, not parent openness, gates this route.
	endedAt := endedGroup.StartTime.Add(-time.Hour)
	endedGroup.StartTime = endedAt.Add(-time.Hour)
	endedGroup.EndTime = &endedAt
	updated, err := db.NewUpdate().Model(endedGroup).ModelTableExpr(`active.groups AS "group"`).Column("start_time", "end_time").WherePK().Exec(ctx)
	require.NoError(t, err)
	affected, err := updated.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)
	today := calendar.TodayDate()
	pastStart, pastEnd := today.AddDays(-20), today.AddDays(-10)
	futureStart, futureEnd := today.AddDays(10), today.AddDays(20)
	// Broad calendar-day gaps avoid dependence on one side of midnight.
	insert := func(staffID, groupID, schoolID int64, start calendar.Date, end *calendar.Date) *testpkg.GroupSupervisorRow {
		row := &testpkg.GroupSupervisorRow{
			TenantID: schoolID, StaffID: staffID, GroupID: groupID,
			Role: "supervisor", StartDate: start, EndDate: end,
		}
		require.NoError(t, db.NewInsert().Model(row).ModelTableExpr("active.group_supervisors").Scan(ctx))
		return row
	}
	created := 0
	makeOwnRow := func(groupID int64, start calendar.Date, end *calendar.Date) *testpkg.GroupSupervisorRow {
		created++
		staff := testpkg.CreateTestStaff(t, db, "SessionSupervisorsBudget", fmt.Sprintf("Row%d", created))
		return insert(staff.ID, groupID, tenantID, start, end)
	}
	makeOwnRow(group.ID, pastStart, &pastEnd)
	makeOwnRow(group.ID, futureStart, nil)
	makeOwnRow(otherGroup.ID, pastStart, nil)
	endedParentRow := makeOwnRow(endedGroup.ID, pastStart, &futureEnd)
	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	testpkg.OwnTenantRows(t, db, foreignTenant)
	foreignStaff, foreignAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, foreignTenant, "SessionSupervisorsBudget", "Foreign")
	foreignGroup := testpkg.CreateTestActiveGroupForTenant(t, db, foreignTenant)
	foreignRow := insert(foreignStaff.ID, foreignGroup.ID, foreignTenant, pastStart, &futureEnd)
	// Reserve an unused real sequence ID instead of guessing an absent sentinel.
	var missingGroupID int64
	require.NoError(t, db.NewRaw("SELECT nextval(pg_get_serial_sequence('active.groups', 'id'))").Scan(ctx, &missingGroupID))
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	request := func(groupID, accountID, schoolID int64, label string, status int) ([]byte, testpkg.QueryMeasurement) {
		t.Helper()
		path := fmt.Sprintf("/active/groups/%d/supervisors", groupID)
		claims := testutil.AdminTestClaimsForTenant(int(accountID), schoolID)
		counter.Reset()
		counter.Start()
		req := testutil.NewRequest("GET", path, nil)
		req = req.WithContext(counter.Context(req.Context()))
		rr := testutil.ExecuteWithAuthPermissions(t, testpkg.MountedQueryBudgetRouter("/api", router), req, claims, []string{permissions.GroupsRead})
		counter.Stop()
		queries := counter.Queries()
		rawSQL, marshalErr := json.Marshal(queries)
		require.NoError(t, marshalErr)
		t.Logf("SESSION_SUPERVISORS label=%s path=%s tenant=%d status=%d statements=%d writes=%d queries=%s body=%s", label, path, schoolID, rr.Code, len(queries), counter.WriteRows(), rawSQL, rr.Body.String())
		require.Equal(t, status, rr.Code, "body: %s", rr.Body.String())
		require.Zero(t, counter.WriteRows())
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
			require.Empty(t, counter.Operation(operation))
		}
		return rr.Body.Bytes(), counter.Measurement(queries)
	}
	assertRows := func(body []byte, want []*testpkg.GroupSupervisorRow) {
		t.Helper()
		var envelope struct {
			Status, Message string
			Data            json.RawMessage
		}
		require.NoError(t, json.Unmarshal(body, &envelope))
		require.Equal(t, "success", envelope.Status)
		require.Equal(t, "Active group supervisors retrieved successfully", envelope.Message)
		var rows []presenceAPI.SupervisorResponse
		require.NoError(t, json.Unmarshal(envelope.Data, &rows))
		require.NotNil(t, rows, "existing parent with no current supervision must return [], not null")
		require.Len(t, rows, len(want))
		var objects []map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(envelope.Data, &objects))
		for index, dto := range rows {
			expect := want[index]
			require.Equal(t, expect.ID, dto.ID, "ascending persisted ID order")
			require.Equal(t, expect.StaffID, dto.StaffID)
			require.Equal(t, expect.GroupID, dto.ActiveGroupID)
			require.True(t, dto.IsActive, "future end is still active")
			require.True(t, expect.CreatedAt.Equal(dto.CreatedAt))
			require.True(t, expect.UpdatedAt.Equal(dto.UpdatedAt))
			require.Equal(t, `"`+expect.StartDate.String()+`T00:00:00Z"`, string(objects[index]["start_time"]))
			keys := []string{"id", "staff_id", "active_group_id", "start_time", "is_active", "created_at", "updated_at"}
			if expect.EndDate == nil {
				require.Nil(t, dto.EndTime)
				require.NotContains(t, objects[index], "end_time")
			} else {
				require.NotNil(t, dto.EndTime)
				require.Equal(t, `"`+expect.EndDate.String()+`T00:00:00Z"`, string(objects[index]["end_time"]))
				keys = append(keys, "end_time")
			}
			require.Len(t, objects[index], len(keys), "full public DTO, no relation hydration or persistence fields")
			for _, key := range keys {
				require.Contains(t, objects[index], key)
			}
			for _, key := range []string{"staff_name", "active_group_name", "tenant_id", "role"} {
				require.NotContains(t, objects[index], key)
			}
		}
	}
	type sample struct {
		size    int
		queries testpkg.QueryMeasurement
	}
	var samples []sample
	want := make([]*testpkg.GroupSupervisorRow, 0, 8)
	for _, size := range []int{0, 3, 8} {
		for len(want) < size {
			var end *calendar.Date
			if len(want)%2 == 1 {
				end = &futureEnd
			}
			want = append(want, makeOwnRow(group.ID, pastStart, end))
		}
		body, queries := request(group.ID, account.ID, tenantID, fmt.Sprintf("n%d", size), testutil.StatusOK)
		assertRows(body, want)
		samples = append(samples, sample{size, queries})
	}
	require.Len(t, samples, 3)
	require.Equal(t, len(samples[1].queries.Statements), len(samples[2].queries.Statements), "N3 to N8 must not add per-supervision reads")
	body, _ := request(endedGroup.ID, account.ID, tenantID, "ended_own_parent", testutil.StatusOK)
	assertRows(body, []*testpkg.GroupSupervisorRow{endedParentRow})
	for _, control := range []struct {
		label string
		id    int64
	}{
		{"missing_parent", missingGroupID},
		{"foreign_parent", foreignGroup.ID},
	} {
		body, _ := request(control.id, account.ID, tenantID, control.label, 404)
		require.JSONEq(t, `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen","title":"Not Found","status":404,"detail":"active: GetActiveGroupWithSupervisors: active group not found","instance":"","code":"general.input"}`, string(body))
	}
	// The row hidden from the first tenant is visible through its own tenant.
	body, _ = request(foreignGroup.ID, foreignAccount.ID, foreignTenant, "foreign_tenant_own_parent", testutil.StatusOK)
	assertRows(body, []*testpkg.GroupSupervisorRow{foreignRow})
	t.Log("SESSION_SUPERVISORS_ALL_BEHAVIOR_COMPLETE measured_requests=3 control_requests=4 no_budget_lookup_yet=true")
	for _, captured := range samples {
		t.Run(fmt.Sprintf("budget_registration/n%d", captured.size), func(t *testing.T) {
			querybudgets.AssertQueryBudget(t, "api.active.groups.supervisors", captured.queries)
		})
	}
}
