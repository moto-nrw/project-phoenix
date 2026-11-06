package me_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestPresenceGroups20CallerQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tc := setupUserContextRoute(t)
	require.Same(t, db, tc.db)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/api/me", tc.resource.Router())
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Groups20", "Caller")
	claims := testutil.TeacherTestClaims(int(account.ID))
	claims.TenantID = testpkg.Tenant(t)
	token := testutil.MintTestJWT(t, claims)
	room := testpkg.CreateTestRoom(t, db, "Caller20Room")
	var groupIDs, activityIDs, sessionIDs, childIDs []int64
	expected := map[string]map[int64]map[string]any{}
	for _, key := range []string{"groups", "active", "activity", "supervised", "students"} {
		expected[key] = map[int64]map[string]any{}
	}
	foreign := testpkg.NewTenantScope(t, db)
	foreignSession := testpkg.CreateTestActiveGroupForTenant(t, db, foreign.TenantID)
	testpkg.CreateTestEducationGroupForTenant(t, db, foreign.TenantID, "ForeignCaller20")
	testpkg.CreateTestEducationGroup(t, db, "UnassignedCaller20")
	counter := testpkg.CaptureQueries(t, db)
	counter.Stop()
	type sample struct {
		key string
		n   int
		sql []string
	}
	var samples []sample
	for _, n := range []int{3, 8} {
		for len(groupIDs) < n {
			i := len(groupIDs)
			group := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("Caller20Group%02d", i))
			testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
			groupIDs = append(groupIDs, group.ID)
			expected["groups"][group.ID] = map[string]any{"id": group.ID, "tenant_id": group.TenantID, "created_at": group.CreatedAt, "updated_at": group.UpdatedAt, "name": group.Name, "via_substitution": false}
			activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Caller20Activity%02d", i))
			activityIDs = append(activityIDs, activity.ID)
			var category testpkg.ActivityCategory
			require.NoError(t, db.NewSelect().Model(&category).Where("id = ?", activity.CategoryID).Scan(testpkg.Ctx(t)))
			cat := map[string]any{"id": category.ID, "tenant_id": category.TenantID, "created_at": category.CreatedAt, "updated_at": category.UpdatedAt, "name": category.Name, "color": category.Color, "is_system": false}
			activityRow := map[string]any{"id": activity.ID, "tenant_id": activity.TenantID, "created_at": activity.CreatedAt, "updated_at": activity.UpdatedAt, "name": activity.Name, "max_participants": activity.MaxParticipants, "is_open": activity.IsOpen, "category_id": activity.CategoryID, "created_by": activity.CreatedBy, "type": "activity", "is_template": false, "is_system": false, "target_group_type": "none", "category": cat}
			expected["activity"][activity.ID] = activityRow
			supervisor := testpkg.ActivitySupervisor{TenantID: testpkg.Tenant(t), StaffID: teacher.StaffID, GroupID: activity.ID}
			_, err := db.NewInsert().Model(&supervisor).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			session := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			sessionIDs = append(sessionIDs, session.ID)
			roomRow := map[string]any{"id": room.ID, "tenant_id": room.TenantID, "created_at": room.CreatedAt, "updated_at": room.UpdatedAt, "name": room.Name, "building": room.Building, "capacity": room.Capacity, "is_system": false, "is_open_room": false}
			sessionRow := map[string]any{"id": session.ID, "tenant_id": session.TenantID, "created_at": session.CreatedAt, "updated_at": session.UpdatedAt, "start_time": session.StartTime, "last_activity": session.LastActivity, "timeout_minutes": session.TimeoutMinutes, "group_id": activity.ID, "room_id": room.ID, "room": roomRow, "actual_group": activityRow}
			expected["active"][session.ID] = sessionRow
			expected["supervised"][session.ID] = sessionRow
			testpkg.CreateTestGroupSupervisor(t, db, teacher.StaffID, session.ID, "supervisor")
			child := testpkg.CreateTestStudent(t, db, "Caller20", fmt.Sprintf("Child%02d", i), "C20")
			childIDs = append(childIDs, child.ID)
			expected["students"][child.ID] = map[string]any{"id": child.ID, "tenant_id": child.TenantID, "created_at": child.CreatedAt, "updated_at": child.UpdatedAt, "person_id": child.PersonID, "school_class": "C20", "sick": false, "excused": false, "status": "active"}
			testpkg.CreateTestVisit(t, db, child.ID, sessionIDs[0], time.Date(2026, 11, 6, 9, i, 0, 0, time.UTC), nil)
		}
		for _, route := range []struct {
			key, path string
			ids       []int64
		}{
			{"groups", "/groups/", groupIDs}, {"active", "/groups/active", sessionIDs}, {"activity", "/groups/activity", activityIDs}, {"supervised", "/groups/supervised", sessionIDs}, {"students", fmt.Sprintf("/groups/%d/students", sessionIDs[0]), childIDs},
		} {
			path := "/api/me" + route.path
			counter.Reset()
			counter.Start()
			rr := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "GET", path, nil, testutil.WithJWTBearer(token)))
			counter.Stop()
			require.Equal(t, 200, rr.Code, rr.Body.String())
			require.Zero(t, counter.WriteRows())
			for _, op := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
				require.Empty(t, counter.Operation(op))
			}
			var envelope struct {
				Data []map[string]any `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
			require.Len(t, envelope.Data, n)
			var ids []int64
			for _, row := range envelope.Data {
				id := int64(row["id"].(float64))
				ids = append(ids, id)
				want, err := json.Marshal(expected[route.key][id])
				require.NoError(t, err)
				actual, err := json.Marshal(row)
				require.NoError(t, err)
				require.JSONEq(t, string(want), string(actual), "complete fixture-derived DTO")
				require.EqualValues(t, testpkg.Tenant(t), row["tenant_id"])
				require.NotEmpty(t, row["created_at"])
				require.NotEmpty(t, row["updated_at"])
				if route.key == "groups" {
					require.Equal(t, false, row["via_substitution"])
				}
			}
			// The caller merges ID sets; no stable ordering is promised by these routes.
			require.ElementsMatch(t, route.ids, ids)
			t.Logf("PRESENCE20_CALLER key=%s n=%d statements=%d sql=%q body=%s", route.key, n, counter.Total(), counter.Queries(), rr.Body.String())
			samples = append(samples, sample{route.key, n, counter.Queries()})
			unauthorized := testutil.ExecuteRequest(router, testutil.NewRequest("GET", path, nil))
			require.Equal(t, 401, unauthorized.Code)
			wrong := claims
			wrong.Scope = "school"
			wrong.IsAdmin = true
			denied := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "GET", path, nil, testutil.WithJWTBearer(testutil.MintTestJWT(t, wrong))))
			require.Equal(t, 401, denied.Code)
		}
	}
	denied := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/api/me/groups/%d/students", foreignSession.ID), nil, testutil.WithJWTBearer(token)))
	require.Equal(t, 404, denied.Code)
	for _, s := range samples {
		t.Run(fmt.Sprintf("%s.n%d", s.key, s.n), func(t *testing.T) { assertPresence20CallerBudget(t, s.key, s.n, s.sql) })
	}
}

func assertPresence20CallerBudget(t *testing.T, key string, n int, queries []string) {
	t.Helper()
	switch fmt.Sprintf("%s.n%d", key, n) {
	case "groups.n3":
		querybudgets.AssertQueryBudget(t, "api.me.presence20.groups.n3", queries)
	case "groups.n8":
		querybudgets.AssertQueryBudget(t, "api.me.presence20.groups.n8", queries)
	case "active.n3":
		querybudgets.AssertQueryBudget(t, "api.me.presence20.active.n3", queries)
	case "active.n8":
		querybudgets.AssertQueryBudget(t, "api.me.presence20.active.n8", queries)
	case "activity.n3":
		querybudgets.AssertQueryBudget(t, "api.me.presence20.activity.n3", queries)
	case "activity.n8":
		querybudgets.AssertQueryBudget(t, "api.me.presence20.activity.n8", queries)
	case "supervised.n3":
		querybudgets.AssertQueryBudget(t, "api.me.presence20.supervised.n3", queries)
	case "supervised.n8":
		querybudgets.AssertQueryBudget(t, "api.me.presence20.supervised.n8", queries)
	case "students.n3":
		querybudgets.AssertQueryBudget(t, "api.me.presence20.students.n3", queries)
	case "students.n8":
		querybudgets.AssertQueryBudget(t, "api.me.presence20.students.n8", queries)
	default:
		t.Fatalf("unknown key %s N%d", key, n)
	}
}
