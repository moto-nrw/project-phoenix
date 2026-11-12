package compose

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestOwnerListGrowthQueryBudgets measures the real owner persistence seams,
// not HTTP hydration. Each N must return every fixture row and execute the
// same registered business SQL inside the tenant transaction.
func TestOwnerListGrowthQueryBudgets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, scenario string
		setup          func(*testing.T, *bun.DB, timetable.Owner, context.Context) (func(int), func(context.Context) (int, error))
	}{
		{name: "activity_exceptions", scenario: "modules.timetable.activity_exceptions.list", setup: func(t *testing.T, db *bun.DB, module timetable.Owner, ctx context.Context) (func(int), func(context.Context) (int, error)) {
			group := testpkg.CreateTestActivityGroup(t, db, "Exception growth")
			add := func(i int) {
				day := i + 1
				createOwnedActivityException(t, module, ctx, group.ID, fmt.Sprintf("2027-08-%02d", day))
			}
			return add, func(ctx context.Context) (int, error) {
				rows, err := module.ListActivityExceptions(ctx, timetable.ActivityExceptionFilter{ActivityGroupID: &group.ID})
				return len(rows), err
			}
		}},
		{name: "activity_instances", scenario: "modules.timetable.activity_instances.list", setup: func(t *testing.T, db *bun.DB, module timetable.Owner, ctx context.Context) (func(int), func(context.Context) (int, error)) {
			fixture := newOwnedActivityInstanceFixture(t, db, "budget")
			add := func(i int) {
				day := i + 17
				createOwnedActivityInstance(t, module, ctx, fixture, fmt.Sprintf("2027-09-%02d", day), "08:00:00", "Budget")
			}
			return add, func(ctx context.Context) (int, error) {
				rows, err := module.ListActivityInstances(ctx, timetable.ActivityInstanceFilter{ActivityGroupID: &fixture.groupID})
				return len(rows), err
			}
		}},
		{name: "enrollments", scenario: "modules.timetable.enrollments.list", setup: func(t *testing.T, db *bun.DB, module timetable.Owner, ctx context.Context) (func(int), func(context.Context) (int, error)) {
			student := testpkg.CreateTestStudent(t, db, "Enrollment", "Budget", "1a")
			add := func(i int) {
				group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("enrollment-owner-budget-%d", i))
				createOwnedEnrollment(t, module, ctx, student.ID, group.ID, "2026-09-01")
			}
			return add, func(ctx context.Context) (int, error) {
				rows, err := module.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{StudentIDs: []int64{student.ID}})
				return len(rows), err
			}
		}},
		{name: "instance_staff", scenario: "modules.timetable.instance_staff.list", setup: func(t *testing.T, db *bun.DB, module timetable.Owner, ctx context.Context) (func(int), func(context.Context) (int, error)) {
			fixture := newOwnedActivityInstanceFixture(t, db, "staff-budget")
			instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-10-08", "08:00:00", "Budget")
			add := func(index int) {
				staff := testpkg.CreateTestStaff(t, db, "Owner", fmt.Sprintf("Budget-%d", index))
				createOwnedInstanceStaff(t, module, ctx, instance.ID, staff.ID, false, false)
			}
			return add, func(ctx context.Context) (int, error) {
				rows, err := module.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{InstanceIDs: []int64{instance.ID}})
				return len(rows), err
			}
		}},
		{name: "instance_students", scenario: "modules.timetable.instance_students.list", setup: func(t *testing.T, db *bun.DB, module timetable.Owner, ctx context.Context) (func(int), func(context.Context) (int, error)) {
			fixture := newOwnedActivityInstanceFixture(t, db, "student-budget")
			instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-05", "08:00:00", "Budget")
			add := func(index int) {
				student := testpkg.CreateTestStudent(t, db, "Owner", fmt.Sprintf("Budget-%d", index), "3a")
				createOwnedInstanceStudent(t, module, ctx, instance.ID, student.ID)
			}
			return add, func(ctx context.Context) (int, error) {
				rows, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{instance.ID}})
				return len(rows), err
			}
		}},
		{name: "planning_tracks", scenario: "modules.timetable.planning_tracks.list", setup: func(t *testing.T, db *bun.DB, module timetable.Owner, ctx context.Context) (func(int), func(context.Context) (int, error)) {
			add := func(i int) {
				createOwnedPlanningTrack(t, module, ctx, string(rune('A'+i)), "#5080D8", i)
			}
			return add, func(ctx context.Context) (int, error) {
				rows, err := module.ListPlanningTracks(ctx, timetable.PlanningTrackFilter{Ordered: true})
				return len(rows), err
			}
		}},
		{name: "schedules", scenario: "modules.timetable.schedules.list", setup: func(t *testing.T, db *bun.DB, module timetable.Owner, ctx context.Context) (func(int), func(context.Context) (int, error)) {
			add := func(i int) {
				group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Schedule growth %d", i))
				weekday := 1
				_, err := module.CreateSchedule(ctx, timetable.ScheduleInput{ActivityGroupID: group.ID, Weekday: weekday})
				require.NoError(t, err)
			}
			return add, func(ctx context.Context) (int, error) {
				rows, err := module.ListSchedules(ctx, timetable.ScheduleFilter{})
				return len(rows), err
			}
		}},
		{name: "supervisors", scenario: "modules.timetable.supervisors.list", setup: func(t *testing.T, db *bun.DB, module timetable.Owner, ctx context.Context) (func(int), func(context.Context) (int, error)) {
			staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Budget")
			add := func(i int) {
				group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("supervisor-owner-budget-%d", i))
				createOwnedSupervisor(t, module, ctx, staff.ID, group.ID, i == 0)
			}
			return add, func(ctx context.Context) (int, error) {
				rows, err := module.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{StaffID: &staff.ID})
				return len(rows), err
			}
		}},
		{name: "timeframes", scenario: "modules.timetable.timeframes.list", setup: func(t *testing.T, db *bun.DB, module timetable.Owner, ctx context.Context) (func(int), func(context.Context) (int, error)) {
			add := func(i int) {
				createOwnedTimeframe(t, module, ctx, "08:00:00", "12:00:00", true, fmt.Sprintf("Budget %d", i))
			}
			return add, func(ctx context.Context) (int, error) {
				rows, err := module.ListTimeframes(ctx, timetable.TimeframeFilter{})
				return len(rows), err
			}
		}},
		{name: "conflict_acks", scenario: "modules.timetable.conflict_acks.list", setup: func(t *testing.T, db *bun.DB, module timetable.Owner, ctx context.Context) (func(int), func(context.Context) (int, error)) {
			accountID := conflictAckAccount(t, db)
			return func(i int) { require.NoError(t, module.AcknowledgeConflict(ctx, accountID, fmt.Sprintf("%032x", i+1))) }, func(ctx context.Context) (int, error) {
				rows, err := module.ListConflictAcks(ctx, accountID)
				return len(rows), err
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			module, ctx := buildModule(t, db), testpkg.Ctx(t)
			add, read := tc.setup(t, db, module, ctx)
			created, small := 0, 0
			for _, n := range []int{3, 8} {
				for created < n {
					add(created)
					created++
				}
				require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
					counter := testpkg.CaptureQueries(t, db)
					rows, err := read(txCtx)
					require.NoError(t, err)
					require.Equal(t, n, rows)
					queries := counter.Queries()
					t.Logf("MEASURE scenario=%s N=%d SQL=%d", tc.scenario, n, len(queries))
					if n == 3 {
						small = len(queries)
					} else {
						require.Len(t, queries, small, "owner SQL must stay flat")
					}
					// The register ratchet discovers literal scenario references.
					if tc.name == "conflict_acks" {
						querybudgets.AssertQueryBudget(t, "modules.timetable.conflict_acks.list", counter.Measurement(queries))
					} else {
						querybudgets.AssertQueryBudget(t, tc.scenario, counter.Measurement(queries))
					}
					return nil
				}))
			}
		})
	}
}
