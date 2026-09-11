package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleOwnsPlanningTrackLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)

	first := createOwnedPlanningTrack(t, module, ctx, "Früh", "#5080D8", 0)
	second := createOwnedPlanningTrack(t, module, ctx, "Mittag", "#F78C10", 1)
	log.seen = nil
	_, err := module.CreatePlanningTrack(ctx, planningTrackInput(" früh ", "#83CD2D", 2))
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNameExists)
	assert.EqualValues(t, 1, observedOperation(log.seen, "create_planning_track").Stats.DuplicatePreventionConflicts)

	found, err := module.FindPlanningTrackForShare(ctx, second.ID)
	require.NoError(t, err)
	assert.Equal(t, second.ID, found.ID)
	updated, ok, err := module.UpdateActivePlanningTrack(ctx, second.ID, planningTrackInput("Spät", "#83CD2D", 1))
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "Spät", updated.Name)
	assert.True(t, updated.UpdatedAt.After(second.UpdatedAt))

	archivedAt := time.Now()
	_, ok, err = module.SetPlanningTrackArchivedAt(ctx, first.ID, &archivedAt)
	require.NoError(t, err)
	assert.True(t, ok)
	_, ok, err = module.UpdateActivePlanningTrack(ctx, first.ID, planningTrackInput("Nein", "#5080D8", 0))
	require.NoError(t, err)
	assert.False(t, ok)
	require.NoError(t, module.ReorderPlanningTracks(ctx, []int64{second.ID}))

	restored, ok, err := module.RestorePlanningTrackAtEnd(ctx, first.ID)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 1, restored.SortOrder)
	listed, err := module.ListPlanningTracks(ctx, timetable.PlanningTrackFilter{Ordered: true})
	require.NoError(t, err)
	assert.Equal(t, []int64{second.ID, first.ID}, planningTrackIDs(listed))
}

func TestModulePlanningTracksAreTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	owned := createOwnedPlanningTrack(t, module, ctx, "Owned", "#5080D8", 0)

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)
	foreign := createOwnedPlanningTrack(t, module, foreignCtx, "Foreign", "#F78C10", 0)

	_, err := module.FindPlanningTrack(foreignCtx, owned.ID)
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNotFound)
	listed, err := module.ListPlanningTracks(ctx, timetable.PlanningTrackFilter{IDs: []int64{owned.ID, foreign.ID}})
	require.NoError(t, err)
	assert.Equal(t, []int64{owned.ID}, planningTrackIDs(listed))
	_, err = module.UpdatePlanningTrack(foreignCtx, owned.ID, planningTrackInput("No", "#83CD2D", 0))
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNotFound)
	require.NoError(t, module.DeletePlanningTrack(foreignCtx, owned.ID))
	_, err = module.FindPlanningTrack(ctx, owned.ID)
	require.NoError(t, err)
}

func TestModulePlanningTrackReadFailuresAreNotSwallowed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, err := module.FindPlanningTrack(ctx, 1)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.ListPlanningTracks(ctx, timetable.PlanningTrackFilter{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestModulePlanningTrackWritesParticipateInCallerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	wantErr := errors.New("abort planning track write")
	var rolledBackID int64

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		created, createErr := module.CreatePlanningTrack(txCtx, planningTrackInput("Rollback", "#5080D8", 0))
		rolledBackID = created.ID
		if createErr != nil {
			return createErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	_, err = module.FindPlanningTrack(ctx, rolledBackID)
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNotFound)

	track := createOwnedPlanningTrack(t, module, ctx, "Stable", "#5080D8", 0)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, updateErr := module.UpdatePlanningTrack(txCtx, track.ID, planningTrackInput("Changed", "#83CD2D", 0))
		if updateErr != nil {
			return updateErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	unchanged, err := module.FindPlanningTrack(ctx, track.ID)
	require.NoError(t, err)
	assert.Equal(t, "Stable", unchanged.Name)
}

func TestModulePlanningTrackArchiveAndRestoreRollBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	track := createOwnedPlanningTrack(t, module, ctx, "Archive", "#5080D8", 0)

	requirePlanningTrackRollback(t, ctx, func(txCtx context.Context) error {
		_, _, err := module.SetPlanningTrackArchivedAt(txCtx, track.ID, planningTrackTime(time.Now()))
		return err
	})
	stored, err := module.FindPlanningTrack(ctx, track.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.ArchivedAt)

	_, _, err = module.SetPlanningTrackArchivedAt(ctx, track.ID, planningTrackTime(time.Now()))
	require.NoError(t, err)
	requirePlanningTrackRollback(t, ctx, func(txCtx context.Context) error {
		_, _, restoreErr := module.RestorePlanningTrackAtEnd(txCtx, track.ID)
		return restoreErr
	})
	stored, err = module.FindPlanningTrack(ctx, track.ID)
	require.NoError(t, err)
	assert.NotNil(t, stored.ArchivedAt)
}

func TestModulePlanningTrackReorderRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	first := createOwnedPlanningTrack(t, module, ctx, "First", "#5080D8", 0)
	second := createOwnedPlanningTrack(t, module, ctx, "Second", "#F78C10", 1)

	requirePlanningTrackRollback(t, ctx, func(txCtx context.Context) error {
		return module.ReorderPlanningTracks(txCtx, []int64{second.ID, first.ID})
	})
	listed, err := module.ListPlanningTracks(ctx, timetable.PlanningTrackFilter{Ordered: true})
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID, second.ID}, planningTrackIDs(listed))
}

func TestModulePlanningTrackDeleteRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	track := createOwnedPlanningTrack(t, module, ctx, "Delete", "#5080D8", 0)

	requirePlanningTrackRollback(t, ctx, func(txCtx context.Context) error {
		return module.DeletePlanningTrack(txCtx, track.ID)
	})
	_, err := module.FindPlanningTrack(ctx, track.ID)
	require.NoError(t, err)
}

func TestPlanningTrackListQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	for i := 0; i < 8; i++ {
		createOwnedPlanningTrack(t, module, ctx, string(rune('A'+i)), "#5080D8", i)
	}
	counter := testpkg.CaptureQueries(t, db)
	_, err := module.ListPlanningTracks(ctx, timetable.PlanningTrackFilter{Ordered: true})
	require.NoError(t, err)
	testpkg.AssertQueryBudget(t, "modules.timetable.planning_tracks.list", counter.Queries())
}

func createOwnedPlanningTrack(t *testing.T, module *timetable.Module, ctx context.Context, name, color string, order int) timetable.PlanningTrack {
	t.Helper()
	value, err := module.CreatePlanningTrack(ctx, planningTrackInput(name, color, order))
	require.NoError(t, err)
	return value
}

func planningTrackInput(name, color string, order int) timetable.PlanningTrackInput {
	return timetable.PlanningTrackInput{Name: name, Color: color, SortOrder: order}
}

func planningTrackIDs(values []timetable.PlanningTrack) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}

func requirePlanningTrackRollback(t *testing.T, ctx context.Context, write func(context.Context) error) {
	t.Helper()
	wantErr := errors.New("abort planning track write")
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if err := write(txCtx); err != nil {
			return err
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
}

func planningTrackTime(value time.Time) *time.Time { return &value }
