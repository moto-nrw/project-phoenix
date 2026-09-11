package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentScheduleCareExitIsAtomicAndRetryable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Schedule", "Rollback", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Schedule", "Owner")
	date := careplan.Date("2032-05-10")
	_, err := module.CreateArrivalSchedule(ctx, careplan.ArrivalSchedule{StudentID: student.ID, Weekday: 1, CreatedBy: staff.ID})
	require.NoError(t, err)
	_, err = module.CreatePickupSchedule(ctx, careplan.PickupSchedule{StudentID: student.ID, Weekday: 1, PickupTime: time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC), CreatedBy: staff.ID})
	require.NoError(t, err)
	_, err = module.CreateArrivalException(ctx, careplan.ArrivalException{StudentID: student.ID, ExceptionDate: date, CreatedBy: staff.ID})
	require.NoError(t, err)
	_, err = module.CreatePickupException(ctx, careplan.PickupException{StudentID: student.ID, ExceptionDate: date, CreatedBy: staff.ID})
	require.NoError(t, err)

	rollback := errors.New("force care-exit rollback")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		rows, commandErr := module.EndStudentSchedulesForCareExit(txCtx, []int64{student.ID}, date)
		require.NoError(t, commandErr)
		assert.EqualValues(t, 4, rows)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	assertStudentScheduleCounts(t, module, ctx, student.ID, 1)
	removals, err := module.ListCareExitSourceRemovals(ctx, []int64{student.ID})
	require.NoError(t, err)
	assert.Empty(t, removals)

	rows, err := module.EndStudentSchedulesForCareExit(ctx, []int64{student.ID}, date)
	require.NoError(t, err)
	assert.EqualValues(t, 4, rows)
	assertStudentScheduleCounts(t, module, ctx, student.ID, 0)
	removals, err = module.ListCareExitSourceRemovals(ctx, []int64{student.ID})
	require.NoError(t, err)
	assert.Len(t, removals, 4)

	restored, err := module.RestoreStudentSchedulesForCareExit(ctx, []int64{student.ID})
	require.NoError(t, err)
	assert.EqualValues(t, 4, restored)
	assertStudentScheduleCounts(t, module, ctx, student.ID, 1)
}

func assertStudentScheduleCounts(t *testing.T, module *careplan.Module, ctx context.Context, studentID int64, want int) {
	t.Helper()
	filter := careplan.StudentScheduleFilter{StudentIDs: []int64{studentID}}
	arrivals, err := module.ListArrivalSchedules(ctx, filter)
	require.NoError(t, err)
	arrivalExceptions, err := module.ListArrivalExceptions(ctx, filter)
	require.NoError(t, err)
	pickups, err := module.ListPickupSchedules(ctx, filter)
	require.NoError(t, err)
	pickupExceptions, err := module.ListPickupExceptions(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, arrivals, want)
	assert.Len(t, arrivalExceptions, want)
	assert.Len(t, pickups, want)
	assert.Len(t, pickupExceptions, want)
}
