package groups_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupRepositoryListWithRoomsPreservesJoinSortPaginationAndEmptySlice(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "Group list behavior")
	alpha := &testutil.SchoolStructureGroup{Name: "Group list alpha", RoomID: &room.ID}
	beta := &testutil.SchoolStructureGroup{Name: "Group list beta", RoomID: &room.ID}
	require.NoError(t, repo.Create(ctx, alpha))
	require.NoError(t, repo.Create(ctx, beta))

	query := &testutil.SchoolStructureGroupListQuery{NameContains: "Group list ", Limit: 1, SortByName: true, Descending: true}
	rows, err := repo.ListWithRooms(ctx, query)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, beta.ID, rows[0].ID)
	require.NotNil(t, rows[0].Room)
	assert.Equal(t, room.ID, rows[0].Room.ID)
	assert.Equal(t, room.Name, rows[0].Room.Name)
	total, err := repo.CountGroups(ctx, query)
	require.NoError(t, err)
	assert.Equal(t, 2, total)

	empty, err := repo.ListWithRooms(ctx, &testutil.SchoolStructureGroupListQuery{NameContains: "missing group list behavior"})
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}
