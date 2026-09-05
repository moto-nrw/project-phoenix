package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleOwnsScheduleLifecycleAndObservesQueries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Schedule owner lifecycle")

	created, err := module.CreateSchedule(ctx, timetable.ScheduleInput{ActivityGroupID: group.ID, Weekday: 2})
	require.NoError(t, err)
	assert.Positive(t, created.ID)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)

	found, err := module.FindSchedule(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, found.Weekday)

	updated, err := module.UpdateSchedule(ctx, created.ID, timetable.ScheduleInput{ActivityGroupID: group.ID, Weekday: 4})
	require.NoError(t, err)
	assert.Equal(t, 4, updated.Weekday)

	listed, err := module.ListSchedules(ctx, timetable.ScheduleFilter{GroupIDs: []int64{group.ID}})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)
	assert.EqualValues(t, 1, observedOperation(log.seen, "list_schedules").Stats.Queries)

	require.NoError(t, module.DeleteSchedule(ctx, created.ID))
	_, err = module.FindSchedule(ctx, created.ID)
	require.ErrorIs(t, err, timetable.ErrScheduleNotFound)
}

func TestModuleScheduleRowsAreTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Owned schedule")
	owned, err := module.CreateSchedule(ctx, timetable.ScheduleInput{ActivityGroupID: group.ID, Weekday: 1})
	require.NoError(t, err)

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)
	foreignGroup := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenantID, "Foreign schedule")
	foreign, err := module.CreateSchedule(foreignCtx, timetable.ScheduleInput{ActivityGroupID: foreignGroup.ID, Weekday: 3})
	require.NoError(t, err)

	_, err = module.FindSchedule(foreignCtx, owned.ID)
	require.ErrorIs(t, err, timetable.ErrScheduleNotFound)
	listed, err := module.ListSchedules(ctx, timetable.ScheduleFilter{GroupIDs: []int64{group.ID, foreignGroup.ID}})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, owned.ID, listed[0].ID)

	_, err = module.UpdateSchedule(foreignCtx, owned.ID, timetable.ScheduleInput{ActivityGroupID: foreignGroup.ID, Weekday: 5})
	require.ErrorIs(t, err, timetable.ErrScheduleNotFound)
	require.NoError(t, module.DeleteSchedule(foreignCtx, owned.ID))
	rows, err := module.CapScheduleValidUntil(foreignCtx, group.ID, "2026-09-10")
	require.NoError(t, err)
	assert.Zero(t, rows)
	stillOwned, err := module.FindSchedule(ctx, owned.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, stillOwned.Weekday)
	_, err = module.FindSchedule(ctx, foreign.ID)
	require.ErrorIs(t, err, timetable.ErrScheduleNotFound)
}

func TestModuleScheduleWritesRespectOuterRollbackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Schedule rollback")
	wantErr := errors.New("abort schedule write")

	var rolledBackID int64
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		created, createErr := module.CreateSchedule(txCtx, timetable.ScheduleInput{ActivityGroupID: group.ID, Weekday: 1})
		rolledBackID = created.ID
		if createErr != nil {
			return createErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	_, err = module.FindSchedule(ctx, rolledBackID)
	require.ErrorIs(t, err, timetable.ErrScheduleNotFound)

	schedule, err := module.CreateSchedule(ctx, timetable.ScheduleInput{ActivityGroupID: group.ID, Weekday: 1})
	require.NoError(t, err)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, updateErr := module.UpdateSchedule(txCtx, schedule.ID, timetable.ScheduleInput{ActivityGroupID: group.ID, Weekday: 2})
		if updateErr != nil {
			return updateErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	unchanged, err := module.FindSchedule(ctx, schedule.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, unchanged.Weekday)
	_, err = module.UpdateSchedule(ctx, schedule.ID, timetable.ScheduleInput{ActivityGroupID: group.ID, Weekday: 2})
	require.NoError(t, err)

	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if deleteErr := module.DeleteSchedule(txCtx, schedule.ID); deleteErr != nil {
			return deleteErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	_, err = module.FindSchedule(ctx, schedule.ID)
	require.NoError(t, err)
	require.NoError(t, module.DeleteSchedule(ctx, schedule.ID))
}

func TestModuleBulkScheduleWritesRollBackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Bulk schedule rollback")
	_, err := module.CreateSchedule(ctx, timetable.ScheduleInput{ActivityGroupID: group.ID, Weekday: 1})
	require.NoError(t, err)
	_, err = module.CreateSchedule(ctx, timetable.ScheduleInput{ActivityGroupID: group.ID, Weekday: 3})
	require.NoError(t, err)
	wantErr := errors.New("abort bulk schedule write")

	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		rows, capErr := module.CapScheduleValidUntil(txCtx, group.ID, "2026-09-10")
		require.EqualValues(t, 2, rows)
		if capErr != nil {
			return capErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	assertSchedulesValidUntil(t, ctx, module, group.ID, nil)
	rows, err := module.CapScheduleValidUntil(ctx, group.ID, "2026-09-10")
	require.NoError(t, err)
	assert.EqualValues(t, 2, rows)
	value := "2026-09-10"
	assertSchedulesValidUntil(t, ctx, module, group.ID, &value)

	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if deleteErr := module.DeleteSchedulesByGroup(txCtx, group.ID); deleteErr != nil {
			return deleteErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	assertSchedulesValidUntil(t, ctx, module, group.ID, &value)
	require.NoError(t, module.DeleteSchedulesByGroup(ctx, group.ID))
	listed, err := module.ListSchedules(ctx, timetable.ScheduleFilter{GroupIDs: []int64{group.ID}})
	require.NoError(t, err)
	assert.Empty(t, listed)
}

func TestModuleScheduleReadFailureIsNotSwallowed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	badCtx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, err := module.FindSchedule(badCtx, 1)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, "internal_error", timetable.ErrorCode(err))
	_, listErr := module.ListSchedules(badCtx, timetable.ScheduleFilter{})
	require.ErrorIs(t, listErr, context.Canceled)
}

func TestScheduleListQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Schedule query budget")
	for weekday := 1; weekday <= 5; weekday++ {
		_, err := module.CreateSchedule(ctx, timetable.ScheduleInput{ActivityGroupID: group.ID, Weekday: weekday})
		require.NoError(t, err)
	}
	counter := testpkg.CaptureQueries(t, db)
	_, err := module.ListSchedules(ctx, timetable.ScheduleFilter{GroupIDs: []int64{group.ID}})
	require.NoError(t, err)
	testpkg.AssertQueryBudget(t, "modules.timetable.schedules.list", counter.Queries())
}

func assertSchedulesValidUntil(t *testing.T, ctx context.Context, module *timetable.Module, groupID int64, want *string) {
	t.Helper()
	rows, err := module.ListSchedules(ctx, timetable.ScheduleFilter{GroupIDs: []int64{groupID}})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, row := range rows {
		assert.Equal(t, want, row.ValidUntil)
	}
}
