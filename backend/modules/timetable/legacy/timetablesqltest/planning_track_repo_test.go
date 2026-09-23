package timetablesqltest_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	model "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func planningTrackRepository(t *testing.T, db *bun.DB) model.PlanningTrackRepository {
	t.Helper()
	factory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	return factory.PlanningTrack
}

// reorderPlanningTracks stores the order in the scope's tenant transaction,
// the way the planning-track administration does.
func reorderPlanningTracks(t *testing.T, db *bun.DB, scope testpkg.TenantScope, ids []int64) error {
	t.Helper()
	repo := planningTrackRepository(t, db)
	return testpkg.WithTenantTx(t, context.Background(), db, scope.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return repo.UpdateSortOrders(txCtx, ids)
	})
}

func TestPlanningTrackRepositoryTenantCRUDAndOrdering(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	repo := planningTrackRepository(t, db)
	ctx := scope.Context()

	first := &model.PlanningTrack{Name: "Früh", Color: "#5080D8", SortOrder: 0}
	second := &model.PlanningTrack{Name: "Mittag", Color: "#F78C10", SortOrder: 1}
	require.NoError(t, repo.Create(ctx, first))
	require.NoError(t, repo.Create(ctx, second))

	require.NoError(t, reorderPlanningTracks(t, db, scope, []int64{second.ID, first.ID}))

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

	require.NoError(t, reorderPlanningTracks(t, db, scope, []int64{second.ID}))
	restored, err := repo.RestoreAtEnd(ctx, first)
	require.NoError(t, err)
	assert.True(t, restored)
	assert.Equal(t, 1, first.SortOrder)
	assert.False(t, first.IsArchived())

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
	first := &model.PlanningTrack{Name: "Früh", Color: "#5080D8", SortOrder: 0}
	second := &model.PlanningTrack{Name: "Mittag", Color: "#F78C10", SortOrder: 1}
	require.NoError(t, repo.Create(scope.Context(), first))
	require.NoError(t, repo.Create(scope.Context(), second))

	err := reorderPlanningTracks(t, db, scope, []int64{first.ID})
	require.True(t, modelBase.IsNoRows(err), "partial order must report not found, got %v", err)

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
