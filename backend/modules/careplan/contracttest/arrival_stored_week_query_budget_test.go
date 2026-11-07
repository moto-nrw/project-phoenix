package contracttest_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	careports "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestStoredArrivalWeekQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	owner := careplantest.NewCarePlan(t, db, testpkg.StudentDirectory{})
	service, err := compose.NewArrivalSchedules(db, owner, nil, nil, compose.ArrivalScheduleDependencies{})
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Stored", "Arrival")
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	foreignStudent := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Other", "Arrival", "1a")
	foreignStaff := testpkg.CreateTestStaffForTenant(t, db, otherTenant, "Other", "Staff")
	_, err = owner.CreateArrivalSchedule(tenant.WithTenantID(ctx, otherTenant), careplan.ArrivalSchedule{
		StudentID: foreignStudent.ID, Weekday: 1, CreatedBy: foreignStaff.ID,
		ExpectedArrival: time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	date := calendar.NewDate(2026, time.November, 2)
	counter := testpkg.CaptureQueriesForContext(t, db)

	for _, n := range []int{3, 8} {
		t.Run(fmt.Sprintf("n%d", n), func(t *testing.T) {
			ids := make([]int64, 0, n)
			for i := range n {
				student := testpkg.CreateTestStudent(t, db, "Stored", fmt.Sprintf("Arrival%d-%d", n, i), "1a")
				ids = append(ids, student.ID)
				// An empty plan must remain empty, not inherit another child's rows.
				if i == n-1 {
					continue
				}
				for _, day := range []int{5, 1} {
					notes := fmt.Sprintf("student %d weekday %d", student.ID, day)
					_, createErr := owner.CreateArrivalSchedule(ctx, careplan.ArrivalSchedule{
						StudentID: student.ID, Weekday: day, CreatedBy: staff.ID, Notes: &notes,
						ExpectedArrival: time.Date(1, 1, 1, 8, i, 0, 0, time.UTC),
					})
					require.NoError(t, createErr)
				}
			}
			expected, listErr := owner.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: ids})
			require.NoError(t, listErr)
			slices.Reverse(ids)
			counter.Reset()
			actual, readErr := testpkg.TenantQuery2(t, counter.Context(ctx), db, service.GetWeeklySchedulesByStudentIDsForDate, ids, date)
			require.NoError(t, readErr)
			values := make([]careplan.ArrivalSchedule, len(actual))
			for i, row := range actual {
				values[i] = *row
			}
			require.Equal(t, expected, values, "include IDs, timestamps, tenant, notes and row ordering")
			if n == 3 {
				querybudgets.AssertQueryBudget(t, "modules.careplan.arrival.stored_week.n3", counter.Queries())
			} else {
				querybudgets.AssertQueryBudget(t, "modules.careplan.arrival.stored_week.n8", counter.Queries())
			}
			mixed := append(slices.Clone(ids), ids[0], foreignStudent.ID, 0, -1)
			actual, readErr = testpkg.TenantQuery2(t, ctx, db, service.GetWeeklySchedulesByStudentIDsForDate, mixed, date)
			require.NoError(t, readErr)
			values = make([]careplan.ArrivalSchedule, len(actual))
			for i, row := range actual {
				values[i] = *row
			}
			require.Equal(t, expected, values, "duplicates, invalid IDs and foreign-tenant IDs do not add rows")
		})
	}
}

type failingArrivalList struct {
	careports.ArrivalScheduleRecords
	err error
}

func (r failingArrivalList) ListArrivalSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error) {
	return nil, r.err
}

func TestStoredArrivalWeekEmptyAndFailureContracts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	failure := errors.New("arrival list unavailable")
	service, err := compose.NewArrivalSchedules(db, failingArrivalList{err: failure}, nil, nil, compose.ArrivalScheduleDependencies{})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	date := calendar.NewDate(2026, time.November, 2)
	rows, err := service.GetWeeklySchedulesByStudentIDsForDate(ctx, nil, date)
	require.NoError(t, err, "an empty ID set must not query the repository")
	require.NotNil(t, rows)
	require.Empty(t, rows)
	student := testpkg.CreateTestStudent(t, db, "Failure", "Arrival", "1a")
	rows, err = service.GetWeeklySchedulesByStudentIDsForDate(ctx, []int64{student.ID, student.ID}, date)
	require.Nil(t, rows)
	require.ErrorIs(t, err, failure)
	require.EqualError(t, err, "schedule error during get student arrival schedules: arrival list unavailable")
}
