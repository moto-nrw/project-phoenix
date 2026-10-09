package compose

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestGroupRecordCompositionPreservesUnboundRoomFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	group := testpkg.CreateTestEducationGroup(t, db, "Unbound room owner")
	repo := NewGroupRepository(NewLegacyRepositoryRuntime(db), GroupRepositoryDependencies{})
	ctx := testpkg.Ctx(t)
	_, err := repo.FindWithRoom(ctx, group.ID)
	require.EqualError(t, err, "database error during find with room: education repositories: room directory is not bound")
	_, err = repo.FindByIDsWithRooms(ctx, []int64{group.ID})
	require.EqualError(t, err, "database error during find by IDs with rooms: education repositories: room directory is not bound")
	_, err = repo.ListWithRooms(ctx, nil)
	require.EqualError(t, err, "database error during list with options: education repositories: room directory is not bound")
}

func TestGroupRecordCompositionPreservesEqualityFilterValues(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	group := testpkg.CreateTestEducationGroup(t, db, "Record equality filter")
	repo := NewGroupRepository(NewLegacyRepositoryRuntime(db), GroupRepositoryDependencies{})
	ctx := testpkg.Ctx(t)
	stored, err := repo.FindByID(ctx, group.ID)
	require.NoError(t, err)
	rows, err := repo.List(ctx, map[string]any{"created_at": stored.CreatedAt.Format(time.RFC3339Nano)})
	require.NoError(t, err)
	require.Len(t, rows, 1, "a timestamp string stays a SQL equality predicate")
	require.Equal(t, group.ID, rows[0].ID)
	err = testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		_, err := repo.List(ctx, map[string]any{"name": 42})
		return err
	})
	require.Error(t, err, "an invalid equality value must not broaden the query")
	var storage *legacyGroupStorageTestError
	require.ErrorAs(t, err, &storage)
}
