package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A profile's first edit creates the account's person at the school, the
// names trimmed; a later edit renames only what it names.
func TestAccountPersonIsCreatedAndRenamedAtTheSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "profile-person@example.test")

	require.NoError(t, module.CreateAccountPerson(ctx, account.ID, " Lea ", " Kraft "))
	created, err := module.FindPersonByAccount(ctx, account.ID)
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)
	assert.Equal(t, "Lea", created.FirstName)
	assert.Equal(t, "Kraft", created.LastName)

	renamed := " Leonie "
	require.NoError(t, module.RenamePerson(ctx, created.ID, &renamed, nil))
	stored, err := module.FindPerson(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Leonie", stored.FirstName)
	assert.Equal(t, "Kraft", stored.LastName, "a name the edit leaves out is kept")
	assert.Equal(t, created.AccountID, stored.AccountID)
	assert.False(t, stored.UpdatedAt.Before(created.UpdatedAt))
}

// A person of another school cannot be renamed through this school.
func TestRenamePersonStaysInTheSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	foreign := testpkg.CreateTestPersonForTenant(t, db, otherTenant, "Ina", "Fremd")

	name := "Umbenannt"
	err := module.RenamePerson(ctx, foreign.ID, &name, nil)
	require.ErrorIs(t, err, peopledirectory.ErrPersonNotFound)

	var firstName string
	require.NoError(t, db.NewRaw(`SELECT first_name FROM users.persons WHERE id = ?`, foreign.ID).Scan(ctx, &firstName))
	assert.Equal(t, "Ina", firstName)
}

// A staff record or Stammdaten write replaces the names and the birthday,
// clears an omitted birthday and keeps the person's tag and account.
func TestSetPersonDetailsReplacesNamesAndBirthday(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	person, account := testpkg.CreateTestPersonWithAccount(t, db, "Lea", "Kraft")

	require.NoError(t, module.SetPersonDetails(ctx, person.ID, peopledirectory.PersonDetails{FirstName: "Leonie", LastName: "Kraft-Berg", Birthday: "2015-04-01"}))
	stored, err := module.FindPerson(ctx, person.ID)
	require.NoError(t, err)
	assert.Equal(t, "Leonie", stored.FirstName)
	assert.Equal(t, "Kraft-Berg", stored.LastName)
	assert.Equal(t, "2015-04-01", stored.Birthday)
	require.NotNil(t, stored.AccountID)
	assert.Equal(t, account.ID, *stored.AccountID)

	require.NoError(t, module.SetPersonDetails(ctx, person.ID, peopledirectory.PersonDetails{FirstName: "Leonie", LastName: "Kraft-Berg"}))
	stored, err = module.FindPerson(ctx, person.ID)
	require.NoError(t, err)
	assert.Empty(t, stored.Birthday)

	err = module.SetPersonDetails(ctx, person.ID, peopledirectory.PersonDetails{FirstName: "Leonie", LastName: "Kraft", Birthday: "2015-02-30"})
	require.Error(t, err, "a date that does not exist is refused")
}

// A person of another school is out of reach.
func TestSetPersonDetailsStaysInTheSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	foreign := testpkg.CreateTestPersonForTenant(t, db, otherTenant, "Ina", "Fremd")

	err := module.SetPersonDetails(ctx, foreign.ID, peopledirectory.PersonDetails{FirstName: "X", LastName: "Y"})
	require.ErrorIs(t, err, peopledirectory.ErrPersonNotFound)
}

// An operator's deletion leaves the deletion placeholders and a tombstone;
// a person that is gone is no error.
func TestAnonymizeAndSoftDeletePersonLeavesTheTombstone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	person := testpkg.CreateTestPerson(t, db, "Lea", "Kraft")
	require.NoError(t, module.SetPersonDetails(ctx, person.ID, peopledirectory.PersonDetails{FirstName: "Lea", LastName: "Kraft", Birthday: "2015-04-01"}))

	require.NoError(t, module.AnonymizeAndSoftDeletePerson(ctx, person.ID))
	var row struct {
		FirstName string  `bun:"first_name"`
		LastName  string  `bun:"last_name"`
		Birthday  *string `bun:"birthday"`
		Deleted   bool    `bun:"deleted"`
	}
	require.NoError(t, db.NewRaw(`SELECT first_name, last_name, birthday::text AS birthday, deleted_at IS NOT NULL AS deleted FROM users.persons WHERE id = ?`, person.ID).Scan(ctx, &row))
	assert.Equal(t, "Gelöscht", row.FirstName)
	assert.Equal(t, "Benutzer", row.LastName)
	assert.Nil(t, row.Birthday)
	assert.True(t, row.Deleted)
	_, err := module.FindPerson(ctx, person.ID)
	require.ErrorIs(t, err, peopledirectory.ErrPersonNotFound)

	require.NoError(t, module.AnonymizeAndSoftDeletePerson(ctx, person.ID), "a repeated deletion is no error")
}
