package e2e_timetable

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestPopulatedTimetableRequestGrowthQueryBudgets(t *testing.T) {
	t.Parallel()
	counts := map[string]int{}
	for _, n := range []int{3, 8} {
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.SetupIsolatedTestDB(t)
			s := setupTimetableScenarioModule(t)
			target := calendar.NewDate(2027, 4, 19)
			s.createActivePeriod(fmt.Sprintf("Growth %d", n), target)
			room := testpkg.CreateTestRoom(t, s.db, "Growth room")
			staff := testpkg.CreateTestStaff(t, s.db, "Growth", "Staff")
			students := []int64{}
			for i := range n {
				child := testpkg.CreateTestStudent(t, s.db, "Growth", fmt.Sprintf("Child %d", i), "3a")
				testpkg.CreateTestArrivalSchedule(t, s.db, child.ID, 1, staff.ID, "12:50")
				students = append(students, child.ID)
			}
			template := s.buildTemplate(templateSpec{name: "Growth template", weekday: 1, startHHMM: "13:00", endHHMM: "14:00", roomID: room.ID, staffIDs: []int64{staff.ID}, studentIDs: students})
			materialized := s.do(http.MethodPost, "/materialize", map[string]any{"from_date": target.String(), "to_date": target.String()}, s.primaryAdminClaims())
			require.Equal(t, http.StatusOK, materialized.Code, materialized.Body.String())
			counter := testpkg.CaptureQueries(t, s.db)
			var exceptionID int64
			for _, modified := range []bool{false, true} {
				kind, scenario := "cancelled", "e2e.timetable.exception_conflicts.cancelled"
				start := parseHHMM(t, "12:30")
				exception := &testpkg.ActivityException{ActivityGroupID: template.group.ID, ExceptionDate: testpkg.ScheduleDate(target), ExceptionType: testpkg.ScheduleExceptionCancelled}
				if modified {
					kind = "modified"
					scenario = "e2e.timetable.exception_conflicts.modified"
					exception.ExceptionType = testpkg.ScheduleExceptionModified
					exception.StartTime = &start
				}
				exception.TenantID = s.primaryTenant
				if exceptionID != 0 {
					_, err := s.db.NewDelete().Model((*testpkg.ActivityException)(nil)).ModelTableExpr("schedule.activity_exceptions").Where("id = ?", exceptionID).Where("tenant_id = ?", s.primaryTenant).Exec(s.tenantCtx())
					require.NoError(t, err)
				}
				_, err := s.db.NewInsert().Model(exception).ModelTableExpr("schedule.activity_exceptions").Exec(s.tenantCtx())
				require.NoError(t, err)
				exceptionID = exception.ID
				counter.Reset()
				response := s.do(http.MethodGet, fmt.Sprintf("/exception-conflicts?date=%s&date_to=%s", target, target), nil, s.primaryAdminClaims())
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				var body struct {
					Conflicts []struct {
						StudentID int64 `json:"student_id"`
					}
				}
				decodeResponse(t, response, &body)
				require.Len(t, body.Conflicts, n, kind)
				require.Zero(t, counter.WriteRows(), "read request must not write")
				queries := counter.Measurement(counter.Queries())
				if n == 3 {
					counts[scenario] = len(queries.Statements)
				} else {
					require.Equal(t, counts[scenario], len(queries.Statements), scenario)
				}
				t.Logf("%s N%d statements=%d", scenario, n, len(queries.Statements))
				querybudgets.AssertQueryBudget(t, scenario, queries)
			}
			for i := 1; i < n; i++ {
				instance := testpkg.CreateTestActivityInstance(t, s.db, target.AddDays(i), room.ID, testpkg.ActivityInstanceOpts{ActivityGroupID: &template.group.ID})
				testpkg.CreateTestInstanceStudent(t, s.db, instance.ID, students[0], testpkg.ScheduleAttendanceStatusExpected)
			}
			counter.Reset()
			response := s.do(http.MethodGet, fmt.Sprintf("/student/%d/week?from=%s&to=%s", students[0], target, target.AddDays(13)), nil, s.primaryAdminClaims())
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var body struct {
				Days []struct{ Instances []struct{ ID int64 } }
			}
			decodeResponse(t, response, &body)
			require.Len(t, body.Days, 14)
			total := 0
			for _, day := range body.Days {
				total += len(day.Instances)
			}
			require.Equal(t, n, total)
			require.Zero(t, counter.WriteRows(), "read request must not write")
			queries := counter.Measurement(counter.Queries())
			scenario := "e2e.timetable.student_week.14d"
			if n == 3 {
				counts[scenario] = len(queries.Statements)
			} else {
				require.Equal(t, counts[scenario], len(queries.Statements), scenario)
			}
			t.Logf("%s N%d statements=%d", scenario, n, len(queries.Statements))
			querybudgets.AssertQueryBudget(t, scenario, queries)
		})
	}
}

func TestEditedOccurrenceRouteGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	counts := 0
	for _, n := range []int{3, 8} {
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.SetupIsolatedTestDB(t)
			s := setupTimetableScenarioModule(t)
			target := calendar.NewDate(2027, 4, 19)
			period := s.createActivePeriod("Edited growth", target)
			room := testpkg.CreateTestRoom(t, s.db, "Edited room")
			staff := testpkg.CreateTestStaff(t, s.db, "Edited", "Staff")
			child := testpkg.CreateTestStudent(t, s.db, "Edited", "Child", "3a")
			template := s.buildTemplate(templateSpec{name: "Original title", weekday: 1, startHHMM: "13:00", endHHMM: "14:00", roomID: room.ID, staffIDs: []int64{staff.ID}, studentIDs: []int64{child.ID}})
			for i := range n {
				instance := testpkg.CreateTestActivityInstance(t, s.db, target.AddDays(i*7), room.ID, testpkg.ActivityInstanceOpts{ActivityGroupID: &template.group.ID, CalendarPeriodID: &period.ID, Title: fmt.Sprintf("Edited %d", i), StartHHMM: "13:00", EndHHMM: "14:00"})
				testpkg.CreateTestInstanceStaff(t, s.db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
				testpkg.CreateTestInstanceStudent(t, s.db, instance.ID, child.ID, testpkg.ScheduleAttendanceStatusExpected)
			}
			counter := testpkg.CaptureQueries(t, s.db)
			response := s.do(http.MethodGet, fmt.Sprintf("/instances/edited-in-window?activity_group_id=%d&from=%s&to=%s", template.group.ID, target, target.AddDays(55)), nil, s.primaryAdminClaims())
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var body struct {
				Count       int
				Occurrences []struct{ ID int64 }
			}
			decodeResponse(t, response, &body)
			require.Equal(t, n, body.Count)
			require.Len(t, body.Occurrences, n)
			require.Zero(t, counter.WriteRows(), "read request must not write")
			queries := counter.Measurement(counter.Queries())
			if n == 3 {
				counts = len(queries.Statements)
			} else {
				require.Equal(t, counts, len(queries.Statements))
			}
			t.Logf("edited occurrences N%d statements=%d", n, len(queries.Statements))
			querybudgets.AssertQueryBudget(t, "api.timetable.edited_in_window.list", queries)
		})
	}
}
