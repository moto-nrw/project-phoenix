package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The class roster replaced the student repository's generic filter surface in
// #3349. It keeps what that surface was pinned for: the departure plan comes
// back hydrated, the rows arrive in a total order, and a selection that matches
// nothing is nil rather than an empty slice.
func TestStudentClassRosterHydratesOrdersAndReturnsNilWhenEmpty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := testutil.NewPeopleRepositorySuiteFactory(db).Student
	ctx := testpkg.Ctx(t)

	class := "List behavior"
	person := testpkg.CreateTestPerson(t, db, "Student list", "Hydrated")
	hydrated := &testpkg.Student{
		PersonID:    person.ID,
		SchoolClass: class,
		BusDays:     departure.BusDaysFromLegacyFlag(true),
	}
	require.NoError(t, repo.Create(ctx, hydrated))
	second := testpkg.CreateTestStudent(t, db, "Student list", "Second", class)

	rows, err := repo.ListClassRoster(ctx, class)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, hydrated.ID, rows[0].ID, "rows arrive in a total order")
	assert.Equal(t, second.ID, rows[1].ID)
	assert.True(t, rows[0].BusDays.HasAny(), "the departure plan comes back hydrated")

	empty, err := repo.ListClassRoster(ctx, "Missing student class")
	require.NoError(t, err)
	assert.Nil(t, empty)
	assert.Empty(t, empty)
}

func TestGuardianProfileRepositoryListWithOptionsPreservesDefaultOrderAndEmptySlice(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := testutil.NewPeopleRepositorySuiteFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)
	zulu := testpkg.CreateTestGuardianProfile(t, db, "guardian-list-zulu")
	alpha := testpkg.CreateTestGuardianProfile(t, db, "guardian-list-alpha")
	zulu.LastName = "Zulu"
	alpha.LastName = "Alpha"
	require.NoError(t, repo.Update(ctx, zulu))
	require.NoError(t, repo.Update(ctx, alpha))

	options := testpkg.NewQueryOptions().WithPagination(1, 1)
	options.Filter.In("id", zulu.ID, alpha.ID)
	rows, err := repo.ListWithOptions(ctx, options)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, alpha.ID, rows[0].ID)

	emptyOptions := testpkg.NewQueryOptions()
	emptyOptions.Filter.Equal("last_name", "Missing guardian")
	empty, err := repo.ListWithOptions(ctx, emptyOptions)
	require.NoError(t, err)
	assert.Nil(t, empty)
	assert.Empty(t, empty)
}

func TestGuardianProfileRepositoryListWithOptionsStaysWithinTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := testutil.NewPeopleRepositorySuiteFactory(db).GuardianProfile
	ctx := testpkg.Ctx(t)
	own := testpkg.CreateTestGuardianProfile(t, db, "guardian-list-own")
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	foreign := testpkg.CreateTestGuardianProfileForTenant(t, db, otherTenant, "Foreign", "Guardian", "guardian-list-foreign")

	rows, err := repo.ListWithOptions(ctx, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, own.ID, rows[0].ID)

	byEmail, err := repo.FindByEmails(ctx, []string{*own.Email, *foreign.Email})
	require.NoError(t, err)
	require.Len(t, byEmail, 1)
	assert.Equal(t, own.ID, byEmail[0].ID)
}
