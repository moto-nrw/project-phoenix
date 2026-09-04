package compose

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleOwnsActivityExceptionLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Owner-Exception-%d", time.Now().UnixNano()))

	first := createOwnedActivityException(t, module, ctx, group.ID, "2027-04-02")
	second := createOwnedActivityException(t, module, ctx, group.ID, "2027-04-09")
	found, err := module.FindActivityException(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, "2027-04-02", found.ExceptionDate)

	listed, err := module.ListActivityExceptions(ctx, timetable.ActivityExceptionFilter{
		ActivityGroupID: &group.ID, FromDate: dateText("2027-04-01"), ToDate: dateText("2027-04-08"), OrderByDate: true,
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID}, activityExceptionIDs(listed))

	start, end, reason := "13:30:00", "15:00:00", "verschoben"
	updated, err := module.UpdateActivityException(ctx, second.ID, timetable.ActivityExceptionInput{
		ActivityGroupID: group.ID, ExceptionDate: second.ExceptionDate, ExceptionType: timetable.ActivityExceptionModified,
		StartTime: &start, EndTime: &end, Reason: &reason,
	})
	require.NoError(t, err)
	assert.Equal(t, &start, updated.StartTime)
	assert.True(t, updated.UpdatedAt.After(second.UpdatedAt))

	require.NoError(t, module.DeleteActivityException(ctx, second.ID))
	_, err = module.FindActivityException(ctx, second.ID)
	require.ErrorIs(t, err, timetable.ErrActivityExceptionNotFound)

	assert.EqualValues(t, 1, observedOperation(log.seen, "list_activity_exceptions").Stats.Queries)
}

func TestModuleOwnsActivityExceptionRetention(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Owner-Exception-Retention-%d", time.Now().UnixNano()))
	createOwnedActivityException(t, module, ctx, group.ID, "2027-04-02")
	createOwnedActivityException(t, module, ctx, group.ID, "2027-04-09")

	count, err := module.CountActivityExceptions(ctx, dateText("2027-04-09"))
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	oldest, err := module.OldestActivityExceptionBefore(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "2027-04-02", *oldest)
	deleted, err := module.DeleteActivityExceptionsBefore(ctx, "2027-04-09")
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
}

func TestModuleActivityExceptionsAreTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Owned-Exception-%d", time.Now().UnixNano()))
	owned := createOwnedActivityException(t, module, ctx, group.ID, "2027-05-01")

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)
	foreignGroup := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenantID, fmt.Sprintf("Foreign-Exception-%d", time.Now().UnixNano()))
	foreign := createOwnedActivityException(t, module, foreignCtx, foreignGroup.ID, "2027-05-01")

	_, err := module.CreateActivityException(ctx, cancelledActivityException(foreignGroup.ID, "2027-05-02"))
	require.Error(t, err, "cross-tenant group reference must be rejected")
	_, err = module.FindActivityException(foreignCtx, owned.ID)
	require.ErrorIs(t, err, timetable.ErrActivityExceptionNotFound)
	listed, err := module.ListActivityExceptions(ctx, timetable.ActivityExceptionFilter{})
	require.NoError(t, err)
	assert.Contains(t, activityExceptionIDs(listed), owned.ID)
	assert.NotContains(t, activityExceptionIDs(listed), foreign.ID)
	_, err = module.UpdateActivityException(foreignCtx, owned.ID, cancelledActivityException(group.ID, "2027-05-02"))
	require.ErrorIs(t, err, timetable.ErrActivityExceptionNotFound)
	require.NoError(t, module.DeleteActivityException(foreignCtx, owned.ID))
	_, err = module.FindActivityException(ctx, owned.ID)
	require.NoError(t, err)
}

func TestModuleActivityExceptionReadFailuresAreNotSwallowed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, err := module.FindActivityException(ctx, 1)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.ListActivityExceptions(ctx, timetable.ActivityExceptionFilter{})
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.CountActivityExceptions(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.OldestActivityExceptionBefore(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
}

func TestModuleActivityExceptionWritesParticipateInCallerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Exception-Rollback-%d", time.Now().UnixNano()))
	var rolledBackID int64

	requireActivityExceptionRollback(t, ctx, func(txCtx context.Context) error {
		created, err := module.CreateActivityException(txCtx, cancelledActivityException(group.ID, "2027-06-01"))
		rolledBackID = created.ID
		return err
	})
	_, err := module.FindActivityException(ctx, rolledBackID)
	require.ErrorIs(t, err, timetable.ErrActivityExceptionNotFound)
	retry := createOwnedActivityException(t, module, ctx, group.ID, "2027-06-01")
	assert.NotZero(t, retry.ID)

	first := createOwnedActivityException(t, module, ctx, group.ID, "2027-06-02")
	requireActivityExceptionRollback(t, ctx, func(txCtx context.Context) error {
		_, updateErr := module.UpdateActivityException(txCtx, first.ID, cancelledActivityException(group.ID, "2027-06-04"))
		return updateErr
	})
	stored, err := module.FindActivityException(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, "2027-06-02", stored.ExceptionDate)
	_, err = module.UpdateActivityException(ctx, first.ID, cancelledActivityException(group.ID, "2027-06-04"))
	require.NoError(t, err)
}

func TestModuleActivityExceptionDeletesParticipateInCallerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Exception-Delete-Rollback-%d", time.Now().UnixNano()))
	first := createOwnedActivityException(t, module, ctx, group.ID, "2027-06-02")
	second := createOwnedActivityException(t, module, ctx, group.ID, "2027-06-03")
	requireActivityExceptionRollback(t, ctx, func(txCtx context.Context) error {
		return module.DeleteActivityException(txCtx, first.ID)
	})
	_, err := module.FindActivityException(ctx, first.ID)
	require.NoError(t, err)
	require.NoError(t, module.DeleteActivityException(ctx, first.ID))
	_, err = module.FindActivityException(ctx, first.ID)
	require.ErrorIs(t, err, timetable.ErrActivityExceptionNotFound)

	requireActivityExceptionRollback(t, ctx, func(txCtx context.Context) error {
		_, deleteErr := module.DeleteActivityExceptionsBefore(txCtx, "2027-06-04")
		return deleteErr
	})
	count, err := module.CountActivityExceptions(ctx, dateText("2027-06-04"))
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	_, err = module.FindActivityException(ctx, second.ID)
	require.NoError(t, err)
	deleted, err := module.DeleteActivityExceptionsBefore(ctx, "2027-06-04")
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
}

func TestModuleActivityExceptionRecordsDuplicateConflicts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Exception-Duplicate-%d", time.Now().UnixNano()))
	input := cancelledActivityException(group.ID, "2027-07-01")
	createOwnedActivityException(t, module, ctx, group.ID, input.ExceptionDate)

	log.seen = nil
	_, err := module.CreateActivityException(ctx, input)
	require.Error(t, err)
	assert.EqualValues(t, 1, observedOperation(log.seen, "create_activity_exception").Stats.DuplicatePreventionConflicts)
}

func TestActivityExceptionListQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Exception-Budget-%d", time.Now().UnixNano()))
	for day := 1; day <= 8; day++ {
		createOwnedActivityException(t, module, ctx, group.ID, fmt.Sprintf("2027-08-%02d", day))
	}
	counter := testpkg.CaptureQueries(t, db)
	_, err := module.ListActivityExceptions(ctx, timetable.ActivityExceptionFilter{ActivityGroupID: &group.ID})
	require.NoError(t, err)
	testpkg.AssertQueryBudget(t, "modules.timetable.activity_exceptions.list", counter.Queries())
}

func createOwnedActivityException(t *testing.T, module *timetable.Module, ctx context.Context, groupID int64, date string) timetable.ActivityException {
	t.Helper()
	value, err := module.CreateActivityException(ctx, cancelledActivityException(groupID, date))
	require.NoError(t, err)
	return value
}

func cancelledActivityException(groupID int64, date string) timetable.ActivityExceptionInput {
	return timetable.ActivityExceptionInput{
		ActivityGroupID: groupID, ExceptionDate: date, ExceptionType: timetable.ActivityExceptionCancelled,
	}
}

func activityExceptionIDs(values []timetable.ActivityException) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}

func dateText(value string) *string { return &value }

func requireActivityExceptionRollback(t *testing.T, ctx context.Context, write func(context.Context) error) {
	t.Helper()
	wantErr := errors.New("abort activity exception write")
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if err := write(txCtx); err != nil {
			return err
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
}
