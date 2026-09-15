package education_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/database/repositories/education"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupRepositoryListWithRoomsPreservesJoinSortPaginationAndEmptySlice(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Group
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "Group list behavior")
	alpha := &educationModels.Group{Name: "Group list alpha", RoomID: &room.ID}
	beta := &educationModels.Group{Name: "Group list beta", RoomID: &room.ID}
	require.NoError(t, repo.Create(ctx, alpha))
	require.NoError(t, repo.Create(ctx, beta))

	query := &educationModels.GroupListQuery{NameContains: "Group list ", Limit: 1, SortByName: true, Descending: true}
	rows, err := repo.ListWithRooms(ctx, query)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, beta.ID, rows[0].ID)
	require.NotNil(t, rows[0].Room)
	assert.Equal(t, room.ID, rows[0].Room.ID)
	assert.Equal(t, room.Name, rows[0].Room.Name)
	countOptions := modelBase.NewQueryOptions()
	countOptions.Filter = query.Filter()
	total, err := repo.CountWithOptions(ctx, countOptions)
	require.NoError(t, err)
	assert.Equal(t, 2, total)

	empty, err := repo.ListWithRooms(ctx, &educationModels.GroupListQuery{NameContains: "missing group list behavior"})
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestGradeTransitionRepositoryListPreservesPageCountAndDefaultOrder(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := education.NewGradeTransitionRepository(db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "transition-list-behavior")

	first := &educationModels.GradeTransition{
		AcademicYear: "2026-2027",
		Status:       educationModels.TransitionStatusDraft,
		CreatedBy:    account.ID,
	}
	first.CreatedAt = time.Date(2026, time.January, 1, 8, 0, 0, 0, time.UTC)
	first.UpdatedAt = first.CreatedAt
	first.SetTenantID(testpkg.Tenant(t))
	second := &educationModels.GradeTransition{
		AcademicYear: "2027-2028",
		Status:       educationModels.TransitionStatusDraft,
		CreatedBy:    account.ID,
	}
	second.CreatedAt = first.CreatedAt.Add(time.Hour)
	second.UpdatedAt = second.CreatedAt
	second.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, first))
	require.NoError(t, repo.Create(ctx, second))

	options := modelBase.NewQueryOptions().WithPagination(1, 1)
	options.Filter.Equal("status", educationModels.TransitionStatusDraft)
	rows, total, err := repo.List(ctx, options)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, second.ID, rows[0].ID)
	assert.Equal(t, 2, total)

}

func TestGradeTransitionRepositoryListPreservesEmptyNilSlice(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := education.NewGradeTransitionRepository(db)
	options := modelBase.NewQueryOptions()
	options.Filter.Equal("academic_year", "missing-transition-year")
	empty, total, err := repo.List(testpkg.Ctx(t), options)
	require.NoError(t, err)
	assert.Nil(t, empty)
	assert.Zero(t, total)
}
