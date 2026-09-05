package compose

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleOwnsTimeframeLifecycleAndQueries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)

	first := createOwnedTimeframe(t, module, ctx, "08:00:00", "12:00:00", true, "Frühbetreuung")
	createOwnedTimeframe(t, module, ctx, "13:00:00", "16:00:00", false, "Nachmittag")
	found, err := module.FindTimeframe(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, "08:00:00", found.StartTime)

	end := "12:30:00"
	updated, err := module.UpdateTimeframe(ctx, first.ID, timetable.TimeframeInput{
		StartTime: "08:30:00", EndTime: &end, IsActive: true, Description: "Frühbetreuung aktualisiert",
	})
	require.NoError(t, err)
	assert.Equal(t, "12:30:00", *updated.EndTime)

	start, rangeEnd := "09:00:00", "14:00:00"
	listed, err := module.ListTimeframes(ctx, timetable.TimeframeFilter{
		DescriptionContains: "betreuung", ActiveOnly: true, OverlapsStart: &start, OverlapsEnd: &rangeEnd,
	})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, first.ID, listed[0].ID)
	assert.EqualValues(t, 1, observedOperation(log.seen, "list_timeframes").Stats.Queries)

	require.NoError(t, module.DeleteTimeframe(ctx, first.ID))
	_, err = module.FindTimeframe(ctx, first.ID)
	require.ErrorIs(t, err, timetable.ErrTimeframeNotFound)
}

func TestModuleTimeframesAreTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	owned := createOwnedTimeframe(t, module, ctx, "08:00:00", "12:00:00", true, "Owned timeframe")

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)
	foreign := createOwnedTimeframe(t, module, foreignCtx, "09:00:00", "13:00:00", true, "Foreign timeframe")

	_, err := module.FindTimeframe(foreignCtx, owned.ID)
	require.ErrorIs(t, err, timetable.ErrTimeframeNotFound)
	listed, err := module.ListTimeframes(ctx, timetable.TimeframeFilter{})
	require.NoError(t, err)
	assert.Contains(t, timeframeIDs(listed), owned.ID)
	assert.NotContains(t, timeframeIDs(listed), foreign.ID)
	_, err = module.UpdateTimeframe(foreignCtx, owned.ID, timeframeInput("10:00:00", "14:00:00", true, "No"))
	require.ErrorIs(t, err, timetable.ErrTimeframeNotFound)
	require.NoError(t, module.DeleteTimeframe(foreignCtx, owned.ID))
	_, err = module.FindTimeframe(ctx, owned.ID)
	require.NoError(t, err)
}

func TestModuleTimeframeReadFailuresAreNotSwallowed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, err := module.FindTimeframe(ctx, 1)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.ListTimeframes(ctx, timetable.TimeframeFilter{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestModuleTimeframeWritesRollBackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	wantErr := errors.New("abort timeframe write")
	var rolledBackID int64

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		created, createErr := module.CreateTimeframe(txCtx, timeframeInput("08:00:00", "12:00:00", true, "Rollback"))
		rolledBackID = created.ID
		if createErr != nil {
			return createErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	_, err = module.FindTimeframe(ctx, rolledBackID)
	require.ErrorIs(t, err, timetable.ErrTimeframeNotFound)

	created := createOwnedTimeframe(t, module, ctx, "08:00:00", "12:00:00", true, "Retry")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, updateErr := module.UpdateTimeframe(txCtx, created.ID, timeframeInput("09:00:00", "13:00:00", true, "Changed"))
		if updateErr != nil {
			return updateErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	unchanged, err := module.FindTimeframe(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "08:00:00", unchanged.StartTime)

	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if deleteErr := module.DeleteTimeframe(txCtx, created.ID); deleteErr != nil {
			return deleteErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	_, err = module.FindTimeframe(ctx, created.ID)
	require.NoError(t, err)
	require.NoError(t, module.DeleteTimeframe(ctx, created.ID))
}

func TestTimeframeListQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	for i := 0; i < 8; i++ {
		createOwnedTimeframe(t, module, ctx, "08:00:00", "12:00:00", true, fmt.Sprintf("Budget %d", i))
	}
	counter := testpkg.CaptureQueries(t, db)
	_, err := module.ListTimeframes(ctx, timetable.TimeframeFilter{})
	require.NoError(t, err)
	testpkg.AssertQueryBudget(t, "modules.timetable.timeframes.list", counter.Queries())
}

func createOwnedTimeframe(t *testing.T, module *timetable.Module, ctx context.Context, start, end string, active bool, description string) timetable.Timeframe {
	t.Helper()
	value, err := module.CreateTimeframe(ctx, timeframeInput(start, end, active, description))
	require.NoError(t, err)
	return value
}

func timeframeInput(start, end string, active bool, description string) timetable.TimeframeInput {
	return timetable.TimeframeInput{StartTime: start, EndTime: &end, IsActive: active, Description: description}
}

func timeframeIDs(values []timetable.Timeframe) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}
