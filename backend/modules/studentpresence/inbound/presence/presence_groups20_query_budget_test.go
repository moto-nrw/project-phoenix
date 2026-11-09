package presence_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	presenceAPI "github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestPresenceGroups20ActiveQueryBudget(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"unclaimed", "detail", "supervisors_all", "student_visits"} {
		t.Run(key, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			tc, active := setupProtectedRouter(t)
			require.Same(t, db, tc.db)
			router := chi.NewRouter()
			router.Use(testpkg.TenantRuntimeMiddleware(t, db))
			router.Mount("/api", active)
			_, account := testpkg.CreateTestStaffWithAccount(t, db, "Presence20", "Reader")
			claims := testutil.AdminTestClaimsForTenant(int(account.ID), testpkg.Tenant(t))
			token := testutil.MintTestJWT(t, claims)
			room := testpkg.CreateTestRoom(t, db, "Presence20Room")
			if key == "unclaimed" {
				_, err := db.NewUpdate().Model(room).Set("name = ?", "Schulhof").WherePK().Exec(testpkg.Ctx(t))
				require.NoError(t, err)
				room.Name = "Schulhof"
			}
			activity := testpkg.CreateTestActivityGroup(t, db, "Presence20Activity")
			student := testpkg.CreateTestStudent(t, db, "Presence20", "Target", "P20")
			foreign := testpkg.NewTenantScope(t, db)
			foreignGroup := testpkg.CreateTestActiveGroupForTenant(t, db, foreign.TenantID)
			if key == "unclaimed" {
				_, err := db.NewUpdate().TableExpr("facilities.rooms").Set("name = ?", "Schulhof").Where("id = ?", foreignGroup.RoomID).Exec(testpkg.Ctx(t))
				require.NoError(t, err)
			}
			var groups []*testpkg.ActiveGroupRow
			var visits []presenceAPI.VisitResponse
			var expected []presenceAPI.ActiveGroupResponse
			counter := testpkg.CaptureQueries(t, db)
			counter.Stop()
			type sample struct {
				n   int
				sql testpkg.QueryMeasurement
			}
			var samples []sample
			for _, n := range []int{3, 8} {
				for len(groups) < n {
					i := len(groups)
					group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
					// The API renders stored instants in the database session zone.
					require.NoError(t, db.NewSelect().Model(group).WherePK().Scan(testpkg.Ctx(t)))
					groups = append(groups, group)
					row := presenceAPI.ActiveGroupResponse{ID: group.ID, GroupID: &activity.ID, RoomID: room.ID, StartTime: group.StartTime.UTC(), IsActive: true, CreatedAt: group.CreatedAt.UTC(), UpdatedAt: group.UpdatedAt.UTC(), Room: &presenceAPI.RoomSimple{ID: room.ID, Name: room.Name}}
					if key == "supervisors_all" {
						staff := testpkg.CreateTestStaff(t, db, "Presence20", fmt.Sprintf("Supervisor%d", i))
						testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
						row.Supervisors = []presenceAPI.GroupSupervisorSimple{{StaffID: staff.ID, Role: "supervisor"}}
						row.SupervisorCount = 1
					}
					if key == "student_visits" || key == "detail" {
						groupID := group.ID
						if key == "detail" {
							groupID = groups[0].ID
						}
						childID := student.ID
						if key == "detail" {
							childID = testpkg.CreateTestStudent(t, db, "Presence20", fmt.Sprintf("Child%d", i), "P20").ID
						}
						var ended *time.Time
						if key == "student_visits" && i > 0 {
							end := time.Date(2026, 11, 6, 10, i, 0, 0, time.UTC)
							ended = &end
						}
						visit := testpkg.CreateTestVisit(t, db, childID, groupID, time.Date(2026, 11, 6, 9, i, 0, 0, time.UTC), ended)
						visits = append(visits, presenceAPI.VisitResponse{ID: visit.ID, StudentID: childID, ActiveGroupID: groupID, CheckInTime: visit.EntryTime.UTC(), IsActive: true, CreatedAt: visit.CreatedAt.UTC(), UpdatedAt: visit.UpdatedAt.UTC()})
					}
					expected = append(expected, row)
				}
				// One open visit per child is a database invariant. N counts
				// all persisted visits, with N-1 closed rows excluded by HTTP.
				if key == "student_visits" {
					visits = visits[:1]
				}
				path := "/api/active/groups/unclaimed"
				switch key {
				case "detail":
					path = fmt.Sprintf("/api/active/groups/%d", groups[0].ID)
				case "supervisors_all":
					path = "/api/active/supervisors/all"
				case "student_visits":
					path = fmt.Sprintf("/api/active/visits/student/%d", student.ID)
				}
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
					Data json.RawMessage `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
				switch key {
				case "detail":
					var got presenceAPI.ActiveGroupResponse
					require.NoError(t, json.Unmarshal(envelope.Data, &got))
					got.StartTime = got.StartTime.UTC()
					got.CreatedAt = got.CreatedAt.UTC()
					got.UpdatedAt = got.UpdatedAt.UTC()
					require.Equal(t, expected[0], got)
				case "supervisors_all":
					var got []presenceAPI.ActiveGroupResponse
					require.NoError(t, json.Unmarshal(envelope.Data, &got))
					for i := range got {
						got[i].StartTime = got[i].StartTime.UTC()
						got[i].CreatedAt = got[i].CreatedAt.UTC()
						got[i].UpdatedAt = got[i].UpdatedAt.UTC()
					}
					require.Equal(t, expected, got)
				case "student_visits":
					var got []presenceAPI.VisitResponse
					require.NoError(t, json.Unmarshal(envelope.Data, &got))
					for i := range got {
						got[i].CheckInTime = got[i].CheckInTime.UTC()
						got[i].CreatedAt = got[i].CreatedAt.UTC()
						got[i].UpdatedAt = got[i].UpdatedAt.UTC()
					}
					require.ElementsMatch(t, visits, got)
				default:
					var got []map[string]any
					require.NoError(t, json.Unmarshal(envelope.Data, &got))
					require.Len(t, got, n)
					for i, row := range got {
						g := groups[n-1-i]
						want := map[string]any{"id": g.ID, "created_at": g.CreatedAt, "updated_at": g.UpdatedAt, "tenant_id": g.TenantID, "start_time": g.StartTime, "last_activity": g.LastActivity, "timeout_minutes": g.TimeoutMinutes, "group_id": g.GroupID, "room_id": g.RoomID, "actual_group": map[string]any{"id": activity.ID, "name": activity.Name}, "room": map[string]any{"id": room.ID, "name": room.Name}}
						expected, err := json.Marshal(want)
						require.NoError(t, err)
						actual, err := json.Marshal(row)
						require.NoError(t, err)
						require.JSONEq(t, string(expected), string(actual), "full DTO in descending start-time order")
					}
				}
				t.Logf("PRESENCE20_ACTIVE key=%s n=%d statements=%d sql=%q body=%s", key, n, counter.Total(), counter.Queries(), rr.Body.String())
				samples = append(samples, sample{n, counter.Measurement(counter.Queries())})
				denied := testutil.ExecuteRequest(router, testutil.NewRequest("GET", path, nil))
				require.Equal(t, 401, denied.Code)
				wrong := claims
				wrong.Scope = "school"
				denied = testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "GET", path, nil, testutil.WithJWTBearer(testutil.MintTestJWT(t, wrong))))
				require.Equal(t, 401, denied.Code)
			}
			foreignResponse := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/api/active/groups/%d", foreignGroup.ID), nil, testutil.WithJWTBearer(token)))
			require.Equal(t, 404, foreignResponse.Code)
			for _, s := range samples {
				t.Run(fmt.Sprintf("n%d", s.n), func(t *testing.T) { assertPresence20ActiveBudget(t, key, s.n, s.sql) })
			}
		})
	}
}

func assertPresence20ActiveBudget(t *testing.T, key string, n int, queries querybudgets.Measurement) {
	t.Helper()
	switch fmt.Sprintf("%s.n%d", key, n) {
	case "unclaimed.n3":
		querybudgets.AssertQueryBudget(t, "api.active.presence20.unclaimed.n3", queries)
	case "unclaimed.n8":
		querybudgets.AssertQueryBudget(t, "api.active.presence20.unclaimed.n8", queries)
	case "detail.n3":
		querybudgets.AssertQueryBudget(t, "api.active.presence20.detail.n3", queries)
	case "detail.n8":
		querybudgets.AssertQueryBudget(t, "api.active.presence20.detail.n8", queries)
	case "supervisors_all.n3":
		querybudgets.AssertQueryBudget(t, "api.active.presence20.supervisors_all.n3", queries)
	case "supervisors_all.n8":
		querybudgets.AssertQueryBudget(t, "api.active.presence20.supervisors_all.n8", queries)
	case "student_visits.n3":
		querybudgets.AssertQueryBudget(t, "api.active.presence20.student_visits.n3", queries)
	case "student_visits.n8":
		querybudgets.AssertQueryBudget(t, "api.active.presence20.student_visits.n8", queries)
	default:
		t.Fatalf("unknown key %s N%d", key, n)
	}
}
