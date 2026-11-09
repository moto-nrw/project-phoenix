package httpintegration_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// Materialization must load schedules once for the whole locked run, not once
// per template. Different weekdays also guard the partition of that batch.
func TestMaterializationScheduleBatchQueryBudget(t *testing.T) {
	t.Parallel()
	from := calendar.NewDate(2026, time.April, 20)
	s := makeScenario(t, activityContract.WeekdayMonday, from)
	defer s.runCleanup(t)
	s.ctx = testpkg.WithTestTenantRuntime(t, s.ctx)
	templateDays := map[int64]calendar.Date{s.template.ID: from}
	add := func(index int) {
		template := &activityContract.Group{
			Name: fmt.Sprintf("Schedule batch %02d", index), CategoryID: s.categoryID,
			MaxParticipants: 20, IsOpen: true, PlannedRoomID: &s.roomID, IsTemplate: true,
		}
		template.SetTenantID(s.tenantID)
		_, err := s.db.NewInsert().Model(activityGroupFixture(template)).ModelTableExpr(`activities.groups AS "group"`).Exec(s.ctx)
		require.NoError(t, err)
		weekday := activityContract.WeekdayMonday + index%2
		schedule := &activityContract.Schedule{ActivityGroupID: template.ID, TimeframeID: &s.timeframe.ID, Weekday: weekday}
		schedule.SetTenantID(s.tenantID)
		_, err = s.db.NewInsert().Model(activityScheduleFixture(schedule)).ModelTableExpr(`activities.schedules`).Exec(s.ctx)
		require.NoError(t, err)
		templateDays[template.ID] = from.AddDays(index % 2)
		for _, studentID := range s.students[:2] {
			row := &activityContract.StudentEnrollment{StudentID: studentID, ActivityGroupID: template.ID, ValidFrom: activityContract.Date(from.AddDays(-30))}
			row.SetTenantID(s.tenantID)
			_, err = s.db.NewInsert().Model(activityStudentEnrollmentFixture(row)).ModelTableExpr(`activities.student_enrollments`).ExcludeColumn("selected_weekdays").Exec(s.ctx)
			require.NoError(t, err)
		}
	}
	counter := testpkg.CaptureQueriesForContext(t, s.db)
	s.ctx = counter.Context(s.ctx)
	previous := 0
	for _, n := range []int{3, 8} {
		for len(templateDays) < n {
			add(len(templateDays))
		}
		counter.Reset()
		result, err := s.svc.MaterializeForTenant(s.ctx, from, from.AddDays(1), timetable.MaterializationSourceManual)
		require.NoError(t, err)
		t.Logf("MEASURE materialize N=%d SQL=%d schedules=%d", n, counter.Total(), len(counter.Selects("activities.schedules")))
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "modules.timetable.materialization.schedules.create.n3", counter.Measurement(counter.Queries()))
		} else {
			querybudgets.AssertQueryBudget(t, "modules.timetable.materialization.schedules.create.n8", counter.Measurement(counter.Queries()))
		}
		require.Len(t, counter.Selects("activities.schedules"), 1, "schedules are a run-wide input, not a per-template read")
		require.Equal(t, n-previous, result.InstancesCreated)
		require.Equal(t, 2*(n-previous), result.InstanceStudentsCreated)
		require.Equal(t, previous, result.CandidatesSkippedExisting)
		for templateID, day := range templateDays {
			instances := listInstancesForDate(t, s.db, templateID, day)
			require.Len(t, instances, 1)
		}
		counter.Reset()
		result, err = s.svc.MaterializeForTenant(s.ctx, from, from.AddDays(1), timetable.MaterializationSourceManual)
		require.NoError(t, err)
		t.Logf("MEASURE materialize repeat N=%d SQL=%d schedules=%d", n, counter.Total(), len(counter.Selects("activities.schedules")))
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "modules.timetable.materialization.schedules.repeat.n3", counter.Measurement(counter.Queries()))
		} else {
			querybudgets.AssertQueryBudget(t, "modules.timetable.materialization.schedules.repeat.n8", counter.Measurement(counter.Queries()))
		}
		require.Len(t, counter.Selects("activities.schedules"), 1)
		require.Zero(t, result.InstancesCreated)
		require.Zero(t, result.InstanceStudentsCreated)
		require.Zero(t, result.InstanceStaffCreated)
		require.Equal(t, n, result.CandidatesSkippedExisting)
		previous = n
	}
}
