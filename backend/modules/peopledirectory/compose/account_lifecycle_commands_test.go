package compose

import (
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSchoolIdentityPersonKeepsLongNamesAndTheCardID(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	card := testpkg.CreateTestRFIDCard(t, db, "LIFECYCLE")
	longName := strings.Repeat("A", peopledirectory.MaxNameLength+50)

	id, err := module.CreateSchoolIdentityPerson(ctx, peopledirectory.SchoolIdentityPerson{
		FirstName: "  " + longName + " ", LastName: " Kraft ", TagID: &card.ID,
	})
	require.NoError(t, err)
	require.Positive(t, id)

	stored, err := module.FindPerson(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), stored.TenantID)
	assert.Equal(t, longName, stored.FirstName, "the account flows never capped a name")
	assert.Equal(t, "Kraft", stored.LastName)
	require.NotNil(t, stored.TagID)
	assert.Equal(t, card.ID, *stored.TagID)
	assert.Nil(t, stored.AccountID, "the lifecycle links the account in a separate step")

	_, err = module.CreatePerson(ctx, peopledirectory.CreatePerson{FirstName: longName, LastName: "Kraft"})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidPerson, "the directory's own create keeps its cap")
}

func TestCreateSchoolIdentityPersonRequiresBothNames(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	for _, person := range []peopledirectory.SchoolIdentityPerson{
		{FirstName: "  ", LastName: "Kraft"},
		{FirstName: "Lea", LastName: ""},
	} {
		_, err := module.CreateSchoolIdentityPerson(ctx, person)
		require.ErrorIs(t, err, peopledirectory.ErrInvalidPerson)
	}
}

func TestCreateInvitedGuardianPrefersEmailInGerman(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	id, err := module.CreateInvitedGuardian(ctx, peopledirectory.InvitedGuardian{
		FirstName: " Ina ", LastName: " Eltern ", Email: "  Ina.Eltern@Example.DE ",
	})
	require.NoError(t, err)
	require.Positive(t, id)

	stored := readGuardianProfile(t, db, id)
	assert.Equal(t, testpkg.Tenant(t), stored.TenantID)
	assert.Equal(t, "Ina", stored.FirstName)
	assert.Equal(t, "Eltern", stored.LastName)
	require.NotNil(t, stored.Email)
	assert.Equal(t, "ina.eltern@example.de", *stored.Email)
	assert.Equal(t, "email", stored.PreferredContactMethod)
	assert.Equal(t, "de", stored.LanguagePreference)
	assert.False(t, stored.HasAccount)
	assert.Nil(t, stored.AccountID)

	_, err = module.CreateInvitedGuardian(ctx, peopledirectory.InvitedGuardian{Email: "INA.ELTERN@example.de"})
	require.ErrorIs(t, err, peopledirectory.ErrGuardianEmailTaken)

	_, err = module.CreateInvitedGuardian(ctx, peopledirectory.InvitedGuardian{Email: "not an address"})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian)
}
