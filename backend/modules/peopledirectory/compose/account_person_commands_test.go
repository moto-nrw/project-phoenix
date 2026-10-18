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
