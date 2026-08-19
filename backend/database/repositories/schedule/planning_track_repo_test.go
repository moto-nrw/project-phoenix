package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	model "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPlanningTrackRepositoryTenantCRUDAndOrdering(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	defer testpkg.CleanupTenantTestData(t, db, scope.TenantID)
	repo := repositories.NewFactory(db).PlanningTrack
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
	defer testpkg.CleanupTenantTestData(t, db, otherScope.TenantID)
	_, err = repo.FindByID(otherScope.Context(), second.ID)
	require.Error(t, err)
}

func TestPlanningTrackRepositoryRejectsPartialOrder(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	defer testpkg.CleanupTenantTestData(t, db, scope.TenantID)
	repo := repositories.NewFactory(db).PlanningTrack
	service := scheduleSvc.NewPlanningTrackService(repo, db)
	first := &model.PlanningTrack{Name: "Früh", Color: "#5080D8", SortOrder: 0}
	second := &model.PlanningTrack{Name: "Mittag", Color: "#F78C10", SortOrder: 1}
	require.NoError(t, repo.Create(scope.Context(), first))
	require.NoError(t, repo.Create(scope.Context(), second))

	err := service.ReorderPlanningTracks(scope.Context(), []int64{first.ID})
	require.ErrorIs(t, err, scheduleSvc.ErrPlanningTrackNotFound)

	err = tenant.WithTenantTx(context.Background(), db, scope.TenantID, func(txCtx context.Context, _ bun.Tx) error {
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
	defer testpkg.CleanupTenantTestData(t, db, scope.TenantID)
	service := scheduleSvc.NewPlanningTrackService(repositories.NewFactory(db).PlanningTrack, db)
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
