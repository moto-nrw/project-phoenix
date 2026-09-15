package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentRepositoryListWithOptionsPreservesHydrationOrderAndEmptySlice(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Student
	ctx := testpkg.Ctx(t)
	person := testpkg.CreateTestPerson(t, db, "Student list", "Hydrated")
	hydrated := &userModels.Student{
		PersonID:    person.ID,
		SchoolClass: "List behavior",
		BusDays:     userModels.BusDaysFromLegacyFlag(true),
	}
	require.NoError(t, repo.Create(ctx, hydrated))
	second := testpkg.CreateTestStudent(t, db, "Student list", "Second", "List behavior")

	options := modelBase.NewQueryOptions().WithPagination(1, 1)
	options.Filter.In("id", hydrated.ID, second.ID)
	rows, err := repo.ListWithOptions(ctx, options)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, hydrated.ID, rows[0].ID)
	assert.True(t, rows[0].BusDays.HasAny())

	emptyOptions := modelBase.NewQueryOptions()
	emptyOptions.Filter.Equal("school_class", "Missing student class")
	empty, err := repo.ListWithOptions(ctx, emptyOptions)
	require.NoError(t, err)
	assert.Nil(t, empty)
	assert.Empty(t, empty)
}

func TestGuardianProfileRepositoryListWithOptionsPreservesDefaultOrderAndEmptySlice(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).GuardianProfile
	ctx := testpkg.Ctx(t)
	zulu := testpkg.CreateTestGuardianProfile(t, db, "guardian-list-zulu")
	alpha := testpkg.CreateTestGuardianProfile(t, db, "guardian-list-alpha")
	zulu.LastName = "Zulu"
	alpha.LastName = "Alpha"
	require.NoError(t, repo.Update(ctx, zulu))
	require.NoError(t, repo.Update(ctx, alpha))

	options := modelBase.NewQueryOptions().WithPagination(1, 1)
	options.Filter.In("id", zulu.ID, alpha.ID)
	rows, err := repo.ListWithOptions(ctx, options)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, alpha.ID, rows[0].ID)

	emptyOptions := modelBase.NewQueryOptions()
	emptyOptions.Filter.Equal("last_name", "Missing guardian")
	empty, err := repo.ListWithOptions(ctx, emptyOptions)
	require.NoError(t, err)
	assert.Nil(t, empty)
	assert.Empty(t, empty)
}
