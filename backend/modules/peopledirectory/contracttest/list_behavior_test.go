package contracttest_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
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
	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Student
	ctx := testpkg.Ctx(t)

	class := "List behavior"
	person := testpkg.CreateTestPerson(t, db, "Student list", "Hydrated")
	hydrated := &peopledirectory.StudentState{
		PersonID:    person.ID,
		SchoolClass: class,
		BusDays:     peopledirectory.BusDaysFromLegacyFlag(true),
	}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Create, hydrated))
	second := testpkg.CreateTestStudent(t, db, "Student list", "Second", class)

	rows, err := testpkg.TenantQuery1(t, ctx, db, repo.ListClassRoster, class)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, hydrated.ID, rows[0].ID, "rows arrive in a total order")
	assert.Equal(t, second.ID, rows[1].ID)
	assert.True(t, rows[0].BusDays.HasAny(), "the departure plan comes back hydrated")

	empty, err := testpkg.TenantQuery1(t, ctx, db, repo.ListClassRoster, "Missing student class")
	require.NoError(t, err)
	assert.Nil(t, empty)
	assert.Empty(t, empty)
}

func TestGuardianProfileRepositoryListWithOptionsPreservesDefaultOrderAndEmptySlice(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).GuardianProfile
	ctx := testpkg.Ctx(t)
	zuluFixture := testpkg.CreateTestGuardianProfile(t, db, "guardian-list-zulu")
	zulu, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, zuluFixture.ID)
	require.NoError(t, err)
	alphaFixture := testpkg.CreateTestGuardianProfile(t, db, "guardian-list-alpha")
	alpha, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, alphaFixture.ID)
	require.NoError(t, err)
	zulu.LastName = "Zulu"
	alpha.LastName = "Alpha"
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Update, zulu))
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Update, alpha))

	options := peoplePorts.NewQueryOptions().WithPagination(1, 1)
	options.Filter.In("id", zulu.ID, alpha.ID)
	rows, err := testpkg.TenantQuery1(t, ctx, db, repo.ListWithOptions, options)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, alpha.ID, rows[0].ID)

	emptyOptions := peoplePorts.NewQueryOptions()
	emptyOptions.Filter.Equal("last_name", "Missing guardian")
	empty, err := testpkg.TenantQuery1(t, ctx, db, repo.ListWithOptions, emptyOptions)
	require.NoError(t, err)
	assert.Nil(t, empty)
	assert.Empty(t, empty)
}

func TestGuardianProfileRepositoryListWithOptionsStaysWithinTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).GuardianProfile
	ctx := testpkg.Ctx(t)
	own := testpkg.CreateTestGuardianProfile(t, db, "guardian-list-own")
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	foreign := testpkg.CreateTestGuardianProfileForTenant(t, db, otherTenant, "Foreign", "Guardian", "guardian-list-foreign")

	rows, err := testpkg.TenantQuery1(t, ctx, db, repo.ListWithOptions, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, own.ID, rows[0].ID)

	byEmail, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByEmails, []string{*own.Email, *foreign.Email})
	require.NoError(t, err)
	require.Len(t, byEmail, 1)
	assert.Equal(t, own.ID, byEmail[0].ID)
}
