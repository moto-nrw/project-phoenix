package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	model "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func planningTrackRepository(t *testing.T, db *bun.DB) model.PlanningTrackRepository {
	t.Helper()
	factory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	factory.BindTimetable(timetabletest.New(t, db))
	return factory.PlanningTrack
}

func TestPlanningTrackRepositoryTenantCRUDAndOrdering(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	repo := planningTrackRepository(t, db)
	service := scheduleSvc.NewPlanningTrackService(repo, db)
	ctx := scope.Context()

	first := &model.PlanningTrack{Name: "Früh", Color: "#5080D8", SortOrder: 0}
	second := &model.PlanningTrack{Name: "Mittag", Color: "#F78C10", SortOrder: 1}
	require.NoError(t, repo.Create(ctx, first))
	require.NoError(t, repo.Create(ctx, second))

	err := service.ReorderPlanningTracks(ctx, []int64{second.ID, first.ID})
	require.NoError(t, err)

	shared, err := repo.FindByIDForShare(ctx, second.ID)
	require.NoError(t, err)
	assert.Equal(t, second.ID, shared.ID)
	second.Name = "Spät"
	updatedActive, err := repo.UpdateIfActive(ctx, second)
	require.NoError(t, err)
	assert.True(t, updatedActive)

	archivedAt := time.Now()
	first.ArchivedAt = &archivedAt
	updated, err := repo.UpdateColumns(ctx, first, "archived_at")
	require.NoError(t, err)
	require.Positive(t, updated)

	tracks, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, tracks, 2)
	assert.Equal(t, second.ID, tracks[0].ID)
	assert.Equal(t, first.ID, tracks[1].ID)
	assert.True(t, tracks[1].IsArchived())
	updatedActive, err = repo.UpdateIfActive(ctx, first)
	require.NoError(t, err)
	assert.False(t, updatedActive)

	require.NoError(t, service.ReorderPlanningTracks(ctx, []int64{second.ID}))
	restored, err := service.RestorePlanningTrack(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, restored.SortOrder)
	assert.False(t, restored.IsArchived())

	otherScope := testpkg.NewTenantScope(t, db)
	_, err = repo.FindByID(otherScope.Context(), second.ID)
	require.Error(t, err)
}

func TestPlanningTrackRepositoryFindByIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	repo := planningTrackRepository(t, db)
	ctx := scope.Context()

	active := &model.PlanningTrack{Name: "Früh", Color: "#5080D8", SortOrder: 0}
	archived := &model.PlanningTrack{Name: "Mittag", Color: "#F78C10", SortOrder: 1}
	require.NoError(t, repo.Create(ctx, active))
	require.NoError(t, repo.Create(ctx, archived))
	archivedAt := time.Now()
	archived.ArchivedAt = &archivedAt
	updated, err := repo.UpdateColumns(ctx, archived, "archived_at")
	require.NoError(t, err)
	require.Positive(t, updated)

	empty, err := repo.FindByIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	// Archived rows resolve too (historical references keep their colour);
	// unknown IDs are simply absent instead of an error.
	tracks, err := repo.FindByIDs(ctx, []int64{active.ID, archived.ID, archived.ID + 1000})
	require.NoError(t, err)
	require.Len(t, tracks, 2)
	byID := map[int64]*model.PlanningTrack{}
	for _, track := range tracks {
		byID[track.ID] = track
	}
	require.NotNil(t, byID[active.ID])
	require.NotNil(t, byID[archived.ID])
	assert.True(t, byID[archived.ID].IsArchived())

	// Tenant isolation: another tenant sees none of these rows.
	otherScope := testpkg.NewTenantScope(t, db)
	foreign, err := repo.FindByIDs(otherScope.Context(), []int64{active.ID, archived.ID})
	require.NoError(t, err)
	assert.Empty(t, foreign)
}

func TestPlanningTrackRepositoryRejectsPartialOrder(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	repo := planningTrackRepository(t, db)
	service := scheduleSvc.NewPlanningTrackService(repo, db)
	first := &model.PlanningTrack{Name: "Früh", Color: "#5080D8", SortOrder: 0}
	second := &model.PlanningTrack{Name: "Mittag", Color: "#F78C10", SortOrder: 1}
	require.NoError(t, repo.Create(scope.Context(), first))
	require.NoError(t, repo.Create(scope.Context(), second))

	err := service.ReorderPlanningTracks(scope.Context(), []int64{first.ID})
	require.ErrorIs(t, err, scheduleSvc.ErrPlanningTrackNotFound)

	err = testpkg.WithTenantTx(t, context.Background(), db, scope.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		require.NoError(t, repo.UpdateSortOrders(txCtx, nil))
		return repo.UpdateSortOrders(txCtx, []int64{first.ID, first.ID + second.ID + 1000})
	})
	require.Error(t, err)
	require.Error(t, repo.UpdateSortOrders(context.Background(), []int64{first.ID, second.ID}))

	updated, err := repo.UpdateIfActive(scope.Context(), nil)
	require.Error(t, err)
	assert.False(t, updated)
	updated, err = repo.UpdateIfActive(scope.Context(), &model.PlanningTrack{
		Name: "Ungültig", Color: "blue",
	})
	require.Error(t, err)
	assert.False(t, updated)
}

func TestPlanningTrackServiceNameConflictAndArchiveLifecycle(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	service := scheduleSvc.NewPlanningTrackService(planningTrackRepository(t, db), db)
	input := scheduleSvc.PlanningTrackInput{Name: "Nord", Color: "#5080D8", SortOrder: 0}

	first, err := service.CreatePlanningTrack(scope.Context(), input)
	require.NoError(t, err)
	_, err = service.CreatePlanningTrack(scope.Context(), scheduleSvc.PlanningTrackInput{
		Name: " nord ", Color: "#83CD2D", SortOrder: 1,
	})
	require.ErrorIs(t, err, scheduleSvc.ErrPlanningTrackNameTaken)
	_, err = service.ArchivePlanningTrack(scope.Context(), first.ID)
	require.NoError(t, err)
	second, err := service.CreatePlanningTrack(scope.Context(), input)
	require.NoError(t, err)
	third, err := service.CreatePlanningTrack(scope.Context(), scheduleSvc.PlanningTrackInput{
		Name: "Süd", Color: "#F78C10", SortOrder: 1,
	})
	require.NoError(t, err)
	_, err = service.UpdatePlanningTrack(scope.Context(), third.ID, scheduleSvc.PlanningTrackInput{
		Name: "NORD", Color: "#83CD2D", SortOrder: 1,
	})
	require.ErrorIs(t, err, scheduleSvc.ErrPlanningTrackNameTaken)
	_, err = service.RestorePlanningTrack(scope.Context(), first.ID)
	require.ErrorIs(t, err, scheduleSvc.ErrPlanningTrackNameTaken)
	assert.NotEqual(t, first.ID, second.ID)
}
