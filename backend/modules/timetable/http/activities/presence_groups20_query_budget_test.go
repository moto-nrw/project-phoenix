//go:build testhooks

package httpadapter_test

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"
	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activitiesHTTP "github.com/moto-nrw/project-phoenix/modules/timetable/http/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestPresenceGroups20ActivitiesQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, service, people, caller, err := rootAPI.NewActivitiesTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	resource := activitiesHTTP.NewResource(service, module.Timetable, rootAPI.NewTimetableSupervisorDirectory(people), caller, rootAPI.TimetableHTTPTransactions)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/api/activities", resource.Router(rootAPI.ProtectedTenantRoutes))
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Activities20", "Reader")
	claims := testutil.AdminTestClaimsForTenant(int(account.ID), testpkg.Tenant(t))
	token := testutil.MintTestJWT(t, claims)
	foreign := testpkg.NewTenantScope(t, db)
	hidden := testpkg.CreateTestActivityGroupForTenant(t, db, foreign.TenantID, "Foreign20")
	group := testpkg.CreateTestActivityGroup(t, db, "Activities20Target")
	expectedGroup := func(g *testpkg.ActivityGroup) activitiesHTTP.ActivityResponse {
		var category testpkg.ActivityCategory
		require.NoError(t, db.NewSelect().Model(&category).Where("id = ?", g.CategoryID).Scan(testpkg.Ctx(t)))
		return activitiesHTTP.ActivityResponse{ID: g.ID, Name: g.Name, MaxParticipants: &g.MaxParticipants, IsOpen: g.IsOpen, CategoryID: g.CategoryID, CreatedBy: g.CreatedBy, CreatedAt: g.CreatedAt.UTC(), UpdatedAt: g.UpdatedAt.UTC(), Category: &activitiesHTTP.CategoryResponse{ID: category.ID, Name: category.Name, Color: category.Color, CreatedAt: category.CreatedAt.UTC(), UpdatedAt: category.UpdatedAt.UTC()}}
	}
	expectedGroups := map[int64]activitiesHTTP.ActivityResponse{group.ID: expectedGroup(group)}
	var groups []int64
	groups = append(groups, group.ID)
	var students []activitiesHTTP.StudentResponse
	var supervisors []activitiesHTTP.SupervisorResponse
	var schedules []activitiesHTTP.ScheduleResponse
	counter := testpkg.CaptureQueries(t, db)
	counter.Stop()
	type sample struct {
		key string
		n   int
		sql testpkg.QueryMeasurement
	}
	var samples []sample
	for _, n := range []int{3, 8} {
		for len(students) < n {
			i := len(students)
			child := testpkg.CreateTestStudent(t, db, "Activities20", fmt.Sprintf("Child%02d", i), "A20")
			_, err := module.Timetable.CreateStudentEnrollment(testpkg.Ctx(t), timetable.StudentEnrollmentInput{StudentID: child.ID, ActivityGroupID: group.ID, ValidFrom: "2026-01-01"})
			require.NoError(t, err)
			students = append(students, activitiesHTTP.StudentResponse{ID: child.ID, FirstName: "Activities20", LastName: fmt.Sprintf("Child%02d", i)})
			staff := testpkg.CreateTestStaff(t, db, "Activities20", fmt.Sprintf("Staff%02d", i))
			supervisor, err := module.Timetable.CreatePlannedSupervisor(testpkg.Ctx(t), timetable.PlannedSupervisorInput{StaffID: staff.ID, GroupID: group.ID, ValidFrom: "2026-01-01"})
			require.NoError(t, err)
			supervisors = append(supervisors, activitiesHTTP.SupervisorResponse{ID: supervisor.ID, StaffID: staff.ID, FirstName: "Activities20", LastName: fmt.Sprintf("Staff%02d", i)})
			end := fmt.Sprintf("%02d:45:00", 8+i)
			timeframe, err := module.Timetable.CreateTimeframe(testpkg.Ctx(t), timetable.TimeframeInput{StartTime: fmt.Sprintf("%02d:15:00", 8+i), EndTime: &end, IsActive: true})
			require.NoError(t, err)
			schedule, err := module.Timetable.CreateSchedule(testpkg.Ctx(t), timetable.ScheduleInput{Weekday: i%5 + 1, ActivityGroupID: group.ID, TimeframeID: &timeframe.ID})
			require.NoError(t, err)
			schedules = append(schedules, activitiesHTTP.ScheduleResponse{ID: schedule.ID, Weekday: schedule.Weekday, ActivityGroupID: group.ID, TimeframeID: &timeframe.ID, CreatedAt: schedule.CreatedAt.UTC(), UpdatedAt: schedule.UpdatedAt.UTC()})
		}
		for len(groups) < n {
			g := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Activities20List%02d", len(groups)))
			groups = append(groups, g.ID)
			expectedGroups[g.ID] = expectedGroup(g)
		}
		sort.Slice(schedules, func(i, j int) bool {
			if schedules[i].Weekday != schedules[j].Weekday {
				return schedules[i].Weekday < schedules[j].Weekday
			}
			return *schedules[i].TimeframeID < *schedules[j].TimeframeID
		})
		target := expectedGroups[group.ID]
		target.EnrollmentCount = n
		target.Supervisors = supervisors
		expectedGroups[group.ID] = target
		for _, route := range []struct{ key, path string }{{"list", "/"}, {"detail", fmt.Sprintf("/%d", group.ID)}, {"schedules", fmt.Sprintf("/%d/schedules", group.ID)}, {"students", fmt.Sprintf("/%d/students", group.ID)}, {"supervisors", fmt.Sprintf("/%d/supervisors", group.ID)}} {
			path := "/api/activities" + route.path
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
			switch route.key {
			case "students":
				var rows []activitiesHTTP.StudentResponse
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Equal(t, students, rows)
			case "supervisors":
				var rows []activitiesHTTP.SupervisorResponse
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Equal(t, supervisors, rows)
			case "schedules":
				var rows []activitiesHTTP.ScheduleResponse
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				for i := range rows {
					rows[i].CreatedAt = rows[i].CreatedAt.UTC()
					rows[i].UpdatedAt = rows[i].UpdatedAt.UTC()
				}
				require.Equal(t, schedules, rows)
			case "detail":
				var row activitiesHTTP.ActivityResponse
				require.NoError(t, json.Unmarshal(envelope.Data, &row))
				row.CreatedAt = row.CreatedAt.UTC()
				row.UpdatedAt = row.UpdatedAt.UTC()
				row.Category.CreatedAt = row.Category.CreatedAt.UTC()
				row.Category.UpdatedAt = row.Category.UpdatedAt.UTC()
				for i := range row.Schedules {
					row.Schedules[i].CreatedAt = row.Schedules[i].CreatedAt.UTC()
					row.Schedules[i].UpdatedAt = row.Schedules[i].UpdatedAt.UTC()
				}
				want := expectedGroups[group.ID]
				want.Schedules = schedules
				for _, supervisor := range supervisors {
					want.SupervisorIDs = append(want.SupervisorIDs, supervisor.StaffID)
				}
				require.Equal(t, want, row)
				require.Equal(t, group.Name, row.Name)
				require.Equal(t, n, row.EnrollmentCount)
				require.ElementsMatch(t, supervisors, row.Supervisors)
				require.Len(t, row.Schedules, n)
			case "list":
				var rows []activitiesHTTP.ActivityResponse
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Len(t, rows, n)
				var ids []int64
				for _, row := range rows {
					ids = append(ids, row.ID)
					row.CreatedAt = row.CreatedAt.UTC()
					row.UpdatedAt = row.UpdatedAt.UTC()
					row.Category.CreatedAt = row.Category.CreatedAt.UTC()
					row.Category.UpdatedAt = row.Category.UpdatedAt.UTC()
					require.Equal(t, expectedGroups[row.ID], row)
					require.NotEqual(t, hidden.ID, row.ID)
					if row.ID == group.ID {
						require.Equal(t, n, row.EnrollmentCount)
						require.ElementsMatch(t, supervisors, row.Supervisors)
					}
				}
				require.ElementsMatch(t, groups, ids)

			}
			t.Logf("PRESENCE20_ACTIVITIES key=%s n=%d statements=%d sql=%q body=%s", route.key, n, counter.Total(), counter.Queries(), rr.Body.String())
			samples = append(samples, sample{route.key, n, counter.Measurement(counter.Queries())})
			denied := testutil.ExecuteRequest(router, testutil.NewRequest("GET", path, nil))
			require.Equal(t, 401, denied.Code)
		}
	}
	for _, suffix := range []string{"", "/students", "/supervisors", "/schedules"} {
		path := fmt.Sprintf("/api/activities/%d%s", hidden.ID, suffix)
		rr := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "GET", path, nil, testutil.WithJWTBearer(token)))
		require.Equal(t, 404, rr.Code)
	}
	for _, suffix := range []string{"/", fmt.Sprintf("/%d", group.ID), fmt.Sprintf("/%d/students", group.ID), fmt.Sprintf("/%d/supervisors", group.ID)} {
		denied := testutil.TeacherTestClaims(int(account.ID))
		denied.Permissions = nil
		rr := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "GET", "/api/activities"+suffix, nil, testutil.WithJWTBearer(testutil.MintTestJWT(t, denied))))
		require.Equal(t, 403, rr.Code)
	}
	for _, s := range samples {
		t.Run(fmt.Sprintf("%s.n%d", s.key, s.n), func(t *testing.T) { assertPresence20ActivitiesBudget(t, s.key, s.n, s.sql) })
	}
}

func assertPresence20ActivitiesBudget(t *testing.T, key string, n int, queries querybudgets.Measurement) {
	t.Helper()
	switch fmt.Sprintf("%s.n%d", key, n) {
	case "list.n3":
		querybudgets.AssertQueryBudget(t, "api.activities.presence20.list.n3", queries)
	case "list.n8":
		querybudgets.AssertQueryBudget(t, "api.activities.presence20.list.n8", queries)
	case "detail.n3":
		querybudgets.AssertQueryBudget(t, "api.activities.presence20.detail.n3", queries)
	case "detail.n8":
		querybudgets.AssertQueryBudget(t, "api.activities.presence20.detail.n8", queries)
	case "schedules.n3":
		querybudgets.AssertQueryBudget(t, "api.activities.presence20.schedules.n3", queries)
	case "schedules.n8":
		querybudgets.AssertQueryBudget(t, "api.activities.presence20.schedules.n8", queries)
	case "students.n3":
		querybudgets.AssertQueryBudget(t, "api.activities.presence20.students.n3", queries)
	case "students.n8":
		querybudgets.AssertQueryBudget(t, "api.activities.presence20.students.n8", queries)
	case "supervisors.n3":
		querybudgets.AssertQueryBudget(t, "api.activities.presence20.supervisors.n3", queries)
	case "supervisors.n8":
		querybudgets.AssertQueryBudget(t, "api.activities.presence20.supervisors.n8", queries)
	default:
		t.Fatalf("unknown key %s N%d", key, n)
	}
}
