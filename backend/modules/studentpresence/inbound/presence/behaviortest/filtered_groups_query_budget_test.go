package presence_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	presenceAPI "github.com/moto-ogs/project-phoenix/modules/studentpresence/inbound/presence"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// TestFilteredActiveGroupListsQueryBudget measures the two filtered list
// handlers, not GET /groups?active=true and its separate relation hydration.
// Both routes are exercised at N0/3/8 before either register lookup. Root must
// register the observed values; this source draft encodes no guessed ceiling.
func TestFilteredActiveGroupListsQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tc, router := setupProtectedRouter(t)
	// SetupTestDB inside the existing route helper detects the top-level
	// isolated database and must return this exact clone, never the package DB.
	require.Same(t, db, tc.db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	account := testpkg.CreateTestAccount(t, db, "filtered-group-budget")
	claims := testutil.AdminTestClaimsForTenant(int(account.ID), tenantID)

	// Separate cohorts prevent one route's filter from accidentally standing in
	// for the other: room sessions use a different activity, activity sessions
	// use a different room. Both lists independently grow from0 to3 to8 rows.
	room := testpkg.CreateTestRoom(t, db, "FilteredBudgetTargetRoom")
	roomActivity := testpkg.CreateTestActivityGroup(t, db, "FilteredBudgetRoomActivity")
	activity := testpkg.CreateTestActivityGroup(t, db, "FilteredBudgetTargetActivity")
	activityRoom := testpkg.CreateTestRoom(t, db, "FilteredBudgetActivityRoom")

	// An open row matching neither requested dimension must never leak into
	// either list. Ended rows match each route's ID but must also stay absent.
	testpkg.CreateTestActiveGroup(t, db, roomActivity.ID, activityRoom.ID)
	endedRoom := testpkg.CreateTestActiveGroup(t, db, roomActivity.ID, room.ID)
	endedActivity := testpkg.CreateTestActiveGroup(t, db, activity.ID, activityRoom.ID)
	for _, ended := range []*testpkg.ActiveGroupRow{endedRoom, endedActivity} {
		endedAt := ended.StartTime.Add(time.Minute)
		update, err := db.NewUpdate().Model(ended).ModelTableExpr("active.groups").
			Set("end_time = ?", endedAt).Where("id = ?", ended.ID).Where("tenant_id = ?", tenantID).Exec(ctx)
		require.NoError(t, err)
		affected, err := update.RowsAffected()
		require.NoError(t, err)
		require.EqualValues(t, 1, affected)
	}

	// All foreign IDs are persisted fixtures, not guessed numeric sentinels.
	// The isolated clone owns the data; the existing fixture lifecycle owns
	// the additional school's rows before the clone is retired.
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	testpkg.OwnTenantRows(t, db, foreignTenantID)
	foreignRoom := testpkg.CreateTestRoomForTenant(t, db, foreignTenantID, "FilteredBudgetForeignRoom")
	foreignActivity := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenantID, "FilteredBudgetForeignActivity")
	foreignSession := testpkg.CreateTestActiveGroupWithIDsForTenant(t, db, foreignTenantID, foreignActivity.ID, foreignRoom.ID)
	require.Positive(t, foreignSession.ID)

	type routeCase struct {
		name    string
		path    string
		message string
	}
	roomRoute := routeCase{"room", fmt.Sprintf("/active/groups/room/%d", room.ID), "Room active groups retrieved successfully"}
	activityRoute := routeCase{"activity", fmt.Sprintf("/active/groups/group/%d", activity.ID), "Group active sessions retrieved successfully"}
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	run := func(route routeCase, want []*testpkg.ActiveGroupRow) testpkg.QueryMeasurement {
		t.Helper()
		counter.Reset()
		counter.Start()
		req := testutil.NewRequest("GET", route.path, nil)
		req = req.WithContext(counter.Context(req.Context()))
		rr := testutil.ExecuteWithAuthPermissions(t, testpkg.MountedQueryBudgetRouter("/api", router), req, claims, []string{permissions.GroupsRead})
		counter.Stop()
		queries := counter.Queries()
		rawSQL, err := json.Marshal(queries)
		require.NoError(t, err)
		t.Logf("FILTERED_GROUP_LIST route=%s path=%s n=%d status=%d statements=%d write_rows=%d queries=%s body=%s",
			route.name, route.path, len(want), rr.Code, len(queries), counter.WriteRows(), rawSQL, rr.Body.String())
		require.Equal(t, testutil.StatusOK, rr.Code, "body: %s", rr.Body.String())
		require.Zero(t, counter.WriteRows())
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
			require.Empty(t, counter.Operation(operation), "%s must be read-only", route.name)
		}

		var envelope struct {
			Status  string          `json:"status"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
		require.Equal(t, "success", envelope.Status)
		require.Equal(t, route.message, envelope.Message)
		require.NotEmpty(t, envelope.Data)
		require.NotEqual(t, "null", string(envelope.Data))
		var rows []presenceAPI.ActiveGroupResponse
		require.NoError(t, json.Unmarshal(envelope.Data, &rows))
		require.NotNil(t, rows, "empty lists serialize as [], never null")
		require.Len(t, rows, len(want))
		var objects []map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(envelope.Data, &objects))
		for index, row := range rows {
			expected := want[index]
			// Store.Order("id") promises ascending persisted session IDs.
			require.Equal(t, expected.ID, row.ID)
			require.NotNil(t, row.GroupID)
			require.Equal(t, *expected.GroupID, *row.GroupID)
			require.Equal(t, expected.RoomID, row.RoomID)
			require.True(t, expected.StartTime.Equal(row.StartTime))
			require.True(t, expected.CreatedAt.Equal(row.CreatedAt))
			require.True(t, expected.UpdatedAt.Equal(row.UpdatedAt))
			require.Nil(t, row.EndTime)
			require.True(t, row.IsActive)
			for _, key := range []string{"id", "group_id", "room_id", "start_time", "is_active", "created_at", "updated_at"} {
				require.Contains(t, objects[index], key)
			}
			for _, key := range []string{"end_time", "room", "supervisors", "supervisor_count", "visit_count"} {
				require.NotContains(t, objects[index], key, "filtered routes do not hydrate relations or counts")
			}
		}
		return counter.Measurement(queries)
	}

	roomRows := make([]*testpkg.ActiveGroupRow, 0, 8)
	activityRows := make([]*testpkg.ActiveGroupRow, 0, 8)
	roomCounts := make(map[int]int)
	activityCounts := make(map[int]int)
	roomSamples := make(map[int]testpkg.QueryMeasurement)
	activitySamples := make(map[int]testpkg.QueryMeasurement)
	for _, size := range []int{0, 3, 8} {
		for len(roomRows) < size {
			roomRows = append(roomRows, testpkg.CreateTestActiveGroup(t, db, roomActivity.ID, room.ID))
			activityRows = append(activityRows, testpkg.CreateTestActiveGroup(t, db, activity.ID, activityRoom.ID))
		}
		roomSamples[size] = run(roomRoute, roomRows)
		activitySamples[size] = run(activityRoute, activityRows)
		roomCounts[size] = len(roomSamples[size].Statements)
		activityCounts[size] = len(activitySamples[size].Statements)
	}
	// No magic statement count: this checks scaling, not an invented ceiling.
	require.Equal(t, roomCounts[3], roomCounts[8], "room list must not add one query per returned session")
	require.Equal(t, activityCounts[3], activityCounts[8], "activity list must not add one query per returned session")

	// Same authorized own-school caller, but IDs that demonstrably belong to a
	// different school. Both successful responses must be exactly empty lists.
	run(routeCase{"foreign_room", fmt.Sprintf("/active/groups/room/%d", foreignRoom.ID), roomRoute.message}, nil)
	run(routeCase{"foreign_activity", fmt.Sprintf("/active/groups/group/%d", foreignActivity.ID), activityRoute.message}, nil)
	t.Logf("FILTERED_GROUP_LIST_BEHAVIOR_COMPLETE room_counts=%v activity_counts=%v all_dto_tenant_dml_assertions_passed=true", roomCounts, activityCounts)

	// Keep registration failures AFTER every real operation and behavioral
	// assertion. Each size retains its full query slice under the same route
	// scenario, protecting the empty path too and emitting six report samples.
	t.Run("room_budget_registration", func(t *testing.T) {
		for _, size := range []int{0, 3, 8} {
			t.Run(fmt.Sprintf("n%d", size), func(t *testing.T) {
				querybudgets.AssertQueryBudget(t, "api.active.groups.by_room", roomSamples[size])
			})
		}
	})
	t.Run("activity_budget_registration", func(t *testing.T) {
		for _, size := range []int{0, 3, 8} {
			t.Run(fmt.Sprintf("n%d", size), func(t *testing.T) {
				querybudgets.AssertQueryBudget(t, "api.active.groups.by_activity", activitySamples[size])
			})
		}
	})
}
