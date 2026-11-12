package compose_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// These are real root-bound owner reads, without transport/authentication or
// the participant route's additional person/staff hydration.
func TestPlannerOwnerGrowthQueryBudgets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		setup  func(*testing.T, *bun.DB, api.TimetableHTTPTestModule) (func(int), func(context.Context) (int, error))
		budget func(*testing.T, querybudgets.Measurement)
	}{
		{name: "participants", setup: func(t *testing.T, db *bun.DB, module api.TimetableHTTPTestModule) (func(int), func(context.Context) (int, error)) {
			room := testpkg.CreateTestRoom(t, db, "Participant growth")
			instance := testpkg.CreateTestActivityInstance(t, db, calendar.NewDate(2027, time.January, 11), room.ID, testpkg.ActivityInstanceOpts{})
			add := func(i int) {
				student := testpkg.CreateTestStudent(t, db, "Participant", fmt.Sprintf("Growth %d", i), "3a")
				testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, timetable.SlotAttendanceExpected)
			}
			read := func(ctx context.Context) (int, error) {
				rows, err := module.Data.ListBlockParticipants(ctx, instance.ID)
				return len(rows), err
			}
			return add, read
		}, budget: func(t *testing.T, queries querybudgets.Measurement) {
			querybudgets.AssertQueryBudget(t, "modules.timetable.block_participants.list", queries)
		}},
		{name: "corrections", setup: func(t *testing.T, db *bun.DB, module api.TimetableHTTPTestModule) (func(int), func(context.Context) (int, error)) {
			room := testpkg.CreateTestRoom(t, db, "Correction growth")
			instance := testpkg.CreateTestActivityInstance(t, db, calendar.NewDate(2027, time.January, 11), room.ID, testpkg.ActivityInstanceOpts{Status: timetable.InstanceStatusCompleted})
			student := testpkg.CreateTestStudent(t, db, "Correction", "Growth", "3a")
			testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, timetable.SlotAttendanceExpected)
			account := testpkg.CreateTestAccount(t, db, "correction-growth@example.com")
			add := func(i int) {
				note := fmt.Sprintf("Correction %d", i)
				require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
					_, err := module.AttendanceCorrections.CorrectInstanceStudentAttendance(ctx, instance.ID, student.ID, timetable.AttendancePatch{Note: &note}, "Fixture correction", account.ID)
					return err
				}))
			}
			read := func(ctx context.Context) (int, error) {
				rows, err := module.AttendanceCorrections.GetAttendanceCorrections(ctx, instance.ID, student.ID)
				return len(rows), err
			}
			return add, read
		}, budget: func(t *testing.T, queries querybudgets.Measurement) {
			querybudgets.AssertQueryBudget(t, "modules.timetable.attendance_corrections.list", queries)
		}},
		{name: "gaps", setup: func(t *testing.T, db *bun.DB, module api.TimetableHTTPTestModule) (func(int), func(context.Context) (int, error)) {
			date := calendar.NewDate(2027, time.January, 11)
			room := testpkg.CreateTestRoom(t, db, "Gap growth")
			add := func(i int) {
				instance := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{Title: fmt.Sprintf("Gap %d", i)})
				result, err := db.NewUpdate().TableExpr("schedule.activity_instances").Set("required_staff = ?", 2).Where("id = ?", instance.ID).Where("tenant_id = ?", testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
				require.NoError(t, err)
				affected, err := result.RowsAffected()
				require.NoError(t, err)
				require.EqualValues(t, 1, affected)
			}
			read := func(ctx context.Context) (int, error) {
				rows, err := module.Data.ListUnderstaffedInstances(ctx, date, date)
				return len(rows), err
			}
			return add, read
		}, budget: func(t *testing.T, queries querybudgets.Measurement) {
			querybudgets.AssertQueryBudget(t, "modules.timetable.staffing_gaps.list", queries)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			module, err := api.NewTimetableHTTPTestModule(testpkg.NewDatabase(t, db), api.TimetableHTTPTestOptions{})
			require.NoError(t, err)
			add, read := tc.setup(t, db, module)
			created, small := 0, 0
			for _, n := range []int{3, 8} {
				for created < n {
					add(created)
					created++
				}
				require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
					counter := testpkg.CaptureQueries(t, db)
					count, err := read(ctx)
					require.NoError(t, err)
					require.Equal(t, n, count)
					queries := counter.Queries()
					t.Logf("MEASURE seam=%s N=%d SQL=%d", tc.name, n, len(queries))
					if n == 3 {
						small = len(queries)
					} else {
						require.Len(t, queries, small)
					}
					tc.budget(t, counter.Measurement(queries))
					return nil
				}))
			}
		})
	}
}
