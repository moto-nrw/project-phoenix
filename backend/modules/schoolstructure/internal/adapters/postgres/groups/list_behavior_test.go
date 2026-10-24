package groups_test

import (
	"testing"

	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	api "github.com/moto-nrw/project-phoenix/roots/owners"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupRepositoryListWithRoomsPreservesJoinSortPaginationAndEmptySlice(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := api.NewSchoolStructureSuiteRepositories(testpkg.NewDatabase(t, db)).Group
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "Group list behavior")
	alpha := &schoolStructureCompose.Group{Name: "Group list alpha", RoomID: &room.ID}
	beta := &schoolStructureCompose.Group{Name: "Group list beta", RoomID: &room.ID}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Create, alpha))
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Create, beta))

	query := &schoolStructureCompose.GroupListQuery{NameContains: "Group list ", Limit: 1, SortByName: true, Descending: true}
	rows, err := testpkg.TenantQuery1(t, ctx, db, repo.ListWithRooms, query)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, beta.ID, rows[0].ID)
	require.NotNil(t, rows[0].Room)
	assert.Equal(t, room.ID, rows[0].Room.ID)
	assert.Equal(t, room.Name, rows[0].Room.Name)
	total, err := testpkg.TenantQuery1(t, ctx, db, repo.CountGroups, query)
	require.NoError(t, err)
	assert.Equal(t, 2, total)

	empty, err := testpkg.TenantQuery1(t, ctx, db, repo.ListWithRooms, &schoolStructureCompose.GroupListQuery{NameContains: "missing group list behavior"})
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}
