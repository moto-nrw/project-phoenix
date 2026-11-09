package me_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestIdentityNavigationPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tc := setupUserContextRoute(t)
	require.Same(t, db, tc.db)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	// TenantSessionChain authenticates but deliberately does not attach host
	// caches. Match the production host's existing request memo here too.
	router.Use(rootAPI.RequestSettingsCacheMiddleware, rootAPI.RequestIdentityCacheMiddleware)
	router.Mount("/api/me", tc.resource.Router())
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Navigation", "Caller")
	claims := testutil.TeacherTestClaims(int(account.ID))
	claims.TenantID = testpkg.Tenant(t)
	token := testutil.MintTestJWT(t, claims)
	room := testpkg.CreateTestRoom(t, db, "NavigationRoom")
	var groupIDs, sessionIDs []int64
	expected := map[string]map[int64]map[string]any{}
	for _, key := range []string{"groups", "supervised"} {
		expected[key] = map[int64]map[string]any{}
	}
	foreign := testpkg.NewTenantScope(t, db)
	testpkg.CreateTestActiveGroupForTenant(t, db, foreign.TenantID)
	testpkg.CreateTestEducationGroupForTenant(t, db, foreign.TenantID, "ForeignNavigation")
	testpkg.CreateTestEducationGroup(t, db, "UnassignedNavigation")
	counter := testpkg.CaptureQueries(t, db)
	counter.Stop()
	for _, n := range []int{3, 8} {
		for len(groupIDs) < n {
			i := len(groupIDs)
			group := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("NavigationGroup%02d", i))
			if i == 0 {
				testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, teacher.StaffID, calendar.NewDate(2020, 1, 1), calendar.NewDate(2099, 12, 31))
			} else {
				testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
			}
			groupIDs = append(groupIDs, group.ID)
			expected["groups"][group.ID] = map[string]any{"id": group.ID, "tenant_id": group.TenantID, "created_at": group.CreatedAt, "updated_at": group.UpdatedAt, "name": group.Name, "via_substitution": i == 0}
			activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("NavigationActivity%02d", i))
			var category testpkg.ActivityCategory
			require.NoError(t, db.NewSelect().Model(&category).Where("id = ?", activity.CategoryID).Scan(testpkg.Ctx(t)))
			cat := map[string]any{"id": category.ID, "tenant_id": category.TenantID, "created_at": category.CreatedAt, "updated_at": category.UpdatedAt, "name": category.Name, "color": category.Color, "is_system": false}
			activityRow := map[string]any{"id": activity.ID, "tenant_id": activity.TenantID, "created_at": activity.CreatedAt, "updated_at": activity.UpdatedAt, "name": activity.Name, "max_participants": activity.MaxParticipants, "is_open": activity.IsOpen, "category_id": activity.CategoryID, "created_by": activity.CreatedBy, "type": "activity", "is_template": false, "is_system": false, "target_group_type": "none", "category": cat}
			session := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			// The API renders stored instants in the database session zone.
			require.NoError(t, db.NewSelect().Model(session).WherePK().Scan(testpkg.Ctx(t)))
			sessionIDs = append(sessionIDs, session.ID)
			roomRow := map[string]any{"id": room.ID, "tenant_id": room.TenantID, "created_at": room.CreatedAt, "updated_at": room.UpdatedAt, "name": room.Name, "building": room.Building, "capacity": room.Capacity, "is_system": false, "is_open_room": false}
			sessionRow := map[string]any{"id": session.ID, "tenant_id": session.TenantID, "created_at": session.CreatedAt, "updated_at": session.UpdatedAt, "start_time": session.StartTime, "last_activity": session.LastActivity, "timeout_minutes": session.TimeoutMinutes, "group_id": activity.ID, "room_id": room.ID, "room": roomRow, "actual_group": activityRow}
			expected["supervised"][session.ID] = sessionRow
			testpkg.CreateTestGroupSupervisor(t, db, teacher.StaffID, session.ID, "supervisor")
		}
		path := "/api/me/navigation"
		counter.Reset()
		counter.Start()
		rr := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(token)))
		counter.Stop()
		require.Equal(t, 200, rr.Code, rr.Body.String())
		require.Zero(t, counter.WriteRows())
		for _, op := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
			require.Empty(t, counter.Operation(op))
		}
		var envelope struct {
			Status  string                     `json:"status"`
			Message string                     `json:"message"`
			Data    map[string]json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
		require.Equal(t, "success", envelope.Status)
		require.Equal(t, "Navigation context retrieved successfully", envelope.Message)
		require.Len(t, envelope.Data, 6)
		require.JSONEq(t, "false", string(envelope.Data["incomplete"]))
		require.JSONEq(t, "[]", string(envelope.Data["unavailable_sections"]))
		require.JSONEq(t, `{"parent_news_enabled":true,"parent_invite_mode":"disabled","timetable_day_start_time":"09:00","timetable_day_end_time":"17:00"}`, string(envelope.Data["school_switches"]))
		staff := teacher.Staff
		rawStaff, err := json.Marshal(map[string]any{"id": staff.ID, "tenant_id": staff.TenantID, "created_at": staff.CreatedAt, "updated_at": staff.UpdatedAt, "person_id": staff.PersonID, "birthday_display_opt_out": false})
		require.NoError(t, err)
		require.JSONEq(t, string(rawStaff), string(envelope.Data["current_staff"]))
		for _, section := range []struct {
			name, key string
			ids       []int64
		}{{"educational_groups", "groups", groupIDs}, {"supervised_groups", "supervised", sessionIDs}} {
			var rows []map[string]any
			require.NoError(t, json.Unmarshal(envelope.Data[section.name], &rows))
			require.Len(t, rows, n)
			var ids []int64
			for _, row := range rows {
				id := int64(row["id"].(float64))
				ids = append(ids, id)
				want, err := json.Marshal(expected[section.key][id])
				require.NoError(t, err)
				got, err := json.Marshal(row)
				require.NoError(t, err)
				require.JSONEq(t, string(want), string(got))
			}
			require.ElementsMatch(t, section.ids, ids)
		}
		t.Logf("NAVIGATION N%d count=%d body=%s", n, counter.Total(), rr.Body.String())
		querybudgets.AssertQueryBudget(t, "api.me.navigation.positive_growth", counter.Measurement(counter.Queries()))
		require.Equal(t, 401, testutil.ExecuteRequest(router, testutil.NewRequest(http.MethodGet, path, nil)).Code)
		wrong := claims
		wrong.Scope = "school"
		require.Equal(t, 401, testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(testutil.MintTestJWT(t, wrong)))).Code)

	}
}
