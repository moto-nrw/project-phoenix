package activities_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivityGroupRepositoryListWithCategoryPreservesJoinAndEmptySlice(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Activity list behavior")

	rows, err := repo.ListWithCategory(ctx, &activities.GroupListQuery{IDs: []int64{group.ID}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Category)
	assert.Equal(t, group.CategoryID, rows[0].Category.ID)

	empty, err := repo.ListWithCategory(ctx, &activities.GroupListQuery{Name: "missing activity list behavior"})
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}
