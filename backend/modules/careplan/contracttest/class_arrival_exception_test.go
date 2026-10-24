package contracttest_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The roster precheck saw the old class. A transfer committed before the
// exception command acquired the child must reject and roll back the report.
func TestClassArrivalExceptionRechecksMembershipAndRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	owner := careplantest.NewCarePlan(t, db, testpkg.StudentDirectory{})
	first := testpkg.CreateTestStudent(t, db, "Arrival", "First", "1a")
	second := testpkg.CreateTestStudent(t, db, "Arrival", "Transferred", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Arrival", "Author")
	people := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db))
	_, deps, err := owners.ArrivalScheduleBinding(people, nil, nil, nil)
	require.NoError(t, err)

	service, err := compose.NewArrivalSchedules(db, owner, nil, nil, deps)
	require.NoError(t, err)
	date := calendar.NewDate(2099, 3, 9)
	clock := time.Date(1, 1, 1, 13, 30, 0, 0, time.UTC)
	err = testpkg.TenantCommand0(t, ctx, db, func(txCtx context.Context) error {
		_, writeErr := service.CreateOrReclaimExceptionForClass(txCtx, " 1A ", first.ID, date, &clock, nil, staff.ID, nil)
		if writeErr != nil {
			return writeErr
		}
		// Commit the transfer after the roster precheck and the first child's write,
		// but before the second child's lock is acquired.
		_, updateErr := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET school_class = ? WHERE student_profile_id = ?`, "2b", second.ID)
		require.NoError(t, updateErr)

		_, writeErr = service.CreateOrReclaimExceptionForClass(txCtx, "1a", second.ID, date, &clock, nil, staff.ID, nil)
		return writeErr
	})
	require.ErrorIs(t, err, careplan.ErrBulkStudentNotFound)
	for _, id := range []int64{first.ID, second.ID} {
		row, readErr := service.GetStudentArrivalExceptionForDate(ctx, id, date)
		require.NoError(t, readErr)
		require.Nil(t, row, "a changed membership rolls back every child's arrival")
	}
}
