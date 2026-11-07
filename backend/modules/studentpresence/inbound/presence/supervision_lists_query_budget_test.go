package presence_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	presenceAPI "github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// TestSupervisionListsQueryBudget covers the basic supervision DTO routes,
// not /supervisors/all or /groups/{id}/supervisors and their different reads.
func TestSupervisionListsQueryBudget(t *testing.T) {
	t.Parallel()
	type sample struct {
		scenario, label string
		queries         []string
	}
	type expected struct {
		row    *testpkg.GroupSupervisorRow
		active bool
	}
	type listCase struct {
		name, scenario, message string
		paths                   []string
	}
	cases := []listCase{
		{"all", "api.active.supervisors.list", "Supervisors retrieved successfully", []string{"/active/supervisors/"}},
		{"active", "api.active.supervisors.list.active", "Supervisors retrieved successfully", []string{"/active/supervisors/?active=true", "/active/supervisors/?active=1"}},
		{"ended", "api.active.supervisors.list.ended", "Supervisors retrieved successfully", []string{"/active/supervisors/?active=false", "/active/supervisors/?active=0"}},
		{"staff", "api.active.supervisors.staff", "Staff supervisions retrieved successfully", nil},
		{"staff_active", "api.active.supervisors.staff.active", "Staff active supervisions retrieved successfully", nil},
		{"group", "api.active.supervisors.group", "Group supervisors retrieved successfully", nil},
	}
	var samples []sample
	behaviorPassed := true
	for _, selection := range cases {
		passed := t.Run(selection.name, func(t *testing.T) {
			// OwnTenant makes this subtest its own clone/lifecycle key. There is
			// deliberately no parent clone holding the one-isolated-clone slot.
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			tc, router := setupProtectedRouter(t)
			require.Same(t, db, tc.db)
			ctx := testpkg.Ctx(t)
			tenantID := testpkg.Tenant(t)
			account := testpkg.CreateTestAccount(t, db, "supervision-list-budget")
			claims := testutil.AdminTestClaimsForTenant(int(account.ID), tenantID)
			room := testpkg.CreateTestRoom(t, db, "SupervisionBudgetRoom")
			activity := testpkg.CreateTestActivityGroup(t, db, "SupervisionBudgetActivity")
			group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			otherGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			staff := testpkg.CreateTestStaff(t, db, "SupervisionBudget", "Target")
			today := calendar.TodayDate()
			pastStart, pastEnd := today.AddDays(-20), today.AddDays(-10)
			futureStart, futureEnd := today.AddDays(10), today.AddDays(20)
			// Broad day gaps keep the expectations valid across a midnight while
			// requests run, without freezing a process-global calendar clock.
			insert := func(staffID, groupID, schoolID int64, start calendar.Date, end *calendar.Date) *testpkg.GroupSupervisorRow {
				row := &testpkg.GroupSupervisorRow{
					TenantID: schoolID, StaffID: staffID, GroupID: groupID,
					Role: "supervisor", StartDate: start, EndDate: end,
				}
				require.NoError(t, db.NewInsert().Model(row).ModelTableExpr("active.group_supervisors").Scan(ctx))
				return row
			}
			created := 0
			makeRow := func(state string, matching bool) expected {
				created++
				staffID := staff.ID
				groupID := group.ID
				if selection.name == "staff" || selection.name == "staff_active" {
					// Distinct group per row avoids unique(staff,group,role).
					groupID = testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID).ID
					if !matching {
						staffID = testpkg.CreateTestStaff(t, db, "SupervisionBudget", fmt.Sprintf("Other%d", created)).ID
					}
				} else {
					staffID = testpkg.CreateTestStaff(t, db, "SupervisionBudget", fmt.Sprintf("Row%d", created)).ID
					if !matching {
						groupID = otherGroup.ID
					}
				}
				start, end, active := pastStart, (*calendar.Date)(nil), true
				switch state {
				case "future_end":
					end = &futureEnd
				case "ended":
					end, active = &pastEnd, false
				case "future_start":
					start, active = futureStart, false
				}
				return expected{insert(staffID, groupID, tenantID, start, end), active}
			}
			if selection.name == "active" || selection.name == "staff" || selection.name == "staff_active" || selection.name == "group" {
				makeRow("ended", true)
				makeRow("future_start", true)
			}
			if selection.name == "ended" {
				makeRow("open", true)
				makeRow("future_end", true)
				makeRow("future_start", true) // inactive is not the same as EndedBy.
			}
			if selection.name == "staff" || selection.name == "staff_active" || selection.name == "group" {
				makeRow("open", false)
			}
			foreignTenant := testpkg.UniqueTestTenantID(t)
			testpkg.EnsureTestTenant(t, db, foreignTenant)
			testpkg.OwnTenantRows(t, db, foreignTenant)
			foreignStaff := testpkg.CreateTestStaffForTenant(t, db, foreignTenant, "SupervisionBudget", "Foreign")
			foreignGroup := testpkg.CreateTestActiveGroupForTenant(t, db, foreignTenant)
			foreignEnd := &futureEnd
			if selection.name == "ended" {
				foreignEnd = &pastEnd
			}
			insert(foreignStaff.ID, foreignGroup.ID, foreignTenant, pastStart, foreignEnd)
			paths := selection.paths
			switch selection.name {
			case "staff":
				paths = []string{fmt.Sprintf("/active/supervisors/staff/%d", staff.ID)}
			case "staff_active":
				paths = []string{fmt.Sprintf("/active/supervisors/staff/%d/active", staff.ID)}
			case "group":
				paths = []string{fmt.Sprintf("/active/supervisors/group/%d", group.ID)}
			}
			counter := testpkg.CaptureQueriesForContext(t, db)
			counter.Stop()
			run := func(path, label string, want []expected) []string {
				t.Helper()
				counter.Reset()
				counter.Start()
				req := testutil.NewRequest("GET", path, nil)
				req = req.WithContext(counter.Context(req.Context()))
				rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{permissions.GroupsRead})
				counter.Stop()
				queries := counter.Queries()
				rawSQL, err := json.Marshal(queries)
				require.NoError(t, err)
				t.Logf("SUPERVISION_LIST label=%s path=%s n=%d status=%d statements=%d writes=%d queries=%s body=%s", label, path, len(want), rr.Code, len(queries), counter.WriteRows(), rawSQL, rr.Body.String())
				require.Equal(t, testutil.StatusOK, rr.Code, "body: %s", rr.Body.String())
				require.Zero(t, counter.WriteRows())
				for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
					require.Empty(t, counter.Operation(operation))
				}
				var envelope struct {
					Status, Message string
					Data            json.RawMessage
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
				require.Equal(t, "success", envelope.Status)
				require.Equal(t, selection.message, envelope.Message)
				var rows []presenceAPI.SupervisorResponse
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.NotNil(t, rows, "zero results must serialize as [], not null")
				require.Len(t, rows, len(want))
				var objects []map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(envelope.Data, &objects))
				for index, dto := range rows {
					expect := want[index]
					require.Equal(t, expect.row.ID, dto.ID, "ascending persisted ID order")
					require.Equal(t, expect.row.StaffID, dto.StaffID)
					require.Equal(t, expect.row.GroupID, dto.ActiveGroupID)
					require.Equal(t, expect.active, dto.IsActive)
					require.True(t, expect.row.CreatedAt.Equal(dto.CreatedAt))
					require.True(t, expect.row.UpdatedAt.Equal(dto.UpdatedAt))
					require.Equal(t, `"`+expect.row.StartDate.String()+`T00:00:00Z"`, string(objects[index]["start_time"]), "DATE projects to UTC midnight, not Berlin midnight")
					if expect.row.EndDate == nil {
						require.Nil(t, dto.EndTime)
						require.NotContains(t, objects[index], "end_time")
					} else {
						require.NotNil(t, dto.EndTime)
						require.Equal(t, `"`+expect.row.EndDate.String()+`T00:00:00Z"`, string(objects[index]["end_time"]))
					}
					for _, key := range []string{"id", "staff_id", "active_group_id", "start_time", "is_active", "created_at", "updated_at"} {
						require.Contains(t, objects[index], key)
					}
					for _, key := range []string{"staff_name", "active_group_name", "tenant_id", "role"} {
						require.NotContains(t, objects[index], key, "basic responses do not hydrate relations")
					}
				}
				return queries
			}
			want := make([]expected, 0, 8)
			counts := make(map[string]map[int]int)
			for _, size := range []int{0, 3, 8} {
				for len(want) < size {
					state := "open"
					if len(want)%2 == 1 {
						state = "future_end"
					}
					switch selection.name {
					case "ended":
						state = "ended"
					case "all":
						state = []string{"open", "future_end", "ended", "future_start"}[len(want)%4]
					}
					want = append(want, makeRow(state, true))
				}
				for variant, path := range paths {
					label := fmt.Sprintf("%s/n%d/v%d", selection.name, size, variant)
					queries := run(path, label, want)
					samples = append(samples, sample{selection.scenario, label, queries})
					if counts[path] == nil {
						counts[path] = make(map[int]int)
					}
					counts[path][size] = len(queries)
				}
			}
			for path, measured := range counts {
				require.Equal(t, measured[3], measured[8], "%s must not grow per supervision", path)
			}
			switch selection.name {
			case "staff":
				run(fmt.Sprintf("/active/supervisors/staff/%d", foreignStaff.ID), "foreign_staff", nil)
			case "staff_active":
				run(fmt.Sprintf("/active/supervisors/staff/%d/active", foreignStaff.ID), "foreign_staff_active", nil)
			case "group":
				run(fmt.Sprintf("/active/supervisors/group/%d", foreignGroup.ID), "foreign_group", nil)
			}
			t.Logf("SUPERVISION_LIST_CASE_BEHAVIOR_COMPLETE case=%s counts=%v", selection.name, counts)
		})
		behaviorPassed = behaviorPassed && passed
	}
	require.True(t, behaviorPassed, "all isolated DTO/date/filter/tenant/DML cases must pass before any registration lookup")
	require.Len(t, samples, 24)
	t.Log("SUPERVISION_LIST_ALL_BEHAVIOR_COMPLETE measured_requests=24 foreign_id_requests=3 no_budget_lookup_yet=true")
	for _, captured := range samples {
		t.Run("budget_registration/"+captured.label, func(t *testing.T) {
			// The repository's register ratchet requires literal scenario names.
			switch captured.scenario {
			case "api.active.supervisors.list":
				querybudgets.AssertQueryBudget(t, "api.active.supervisors.list", captured.queries)
			case "api.active.supervisors.list.active":
				querybudgets.AssertQueryBudget(t, "api.active.supervisors.list.active", captured.queries)
			case "api.active.supervisors.list.ended":
				querybudgets.AssertQueryBudget(t, "api.active.supervisors.list.ended", captured.queries)
			case "api.active.supervisors.staff":
				querybudgets.AssertQueryBudget(t, "api.active.supervisors.staff", captured.queries)
			case "api.active.supervisors.staff.active":
				querybudgets.AssertQueryBudget(t, "api.active.supervisors.staff.active", captured.queries)
			case "api.active.supervisors.group":
				querybudgets.AssertQueryBudget(t, "api.active.supervisors.group", captured.queries)
			default:
				t.Fatalf("unexpected budget scenario %q", captured.scenario)
			}
		})
	}
}
