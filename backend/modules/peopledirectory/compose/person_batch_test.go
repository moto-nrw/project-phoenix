package compose

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// CreatePersons is CreatePerson for many persons (#5552): normalized like a
// single create, index-aligned, every person or none, in the caller's
// transaction only.
func TestModuleCreatesPersonsInOneCommand(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	batches, ok := module.(peopledirectory.PersonBatches)
	require.True(t, ok, "the composed owner offers the batch slice")
	ctx := testpkg.Ctx(t)
	card := testpkg.CreateTestRFIDCard(t, db, "BATCH")
	other := testpkg.CreateTestRFIDCard(t, db, "OTHER")
	account := testpkg.CreateTestAccount(t, db, "person-batch@example.com")

	empty, err := batches.CreatePersons(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
	_, err = batches.CreatePersons(ctx, []peopledirectory.CreatePerson{{FirstName: "Ohne", LastName: "Transaktion"}})
	require.Error(t, err, "a batch command joins its caller's transaction and opens none")

	createPersons := func(inputs []peopledirectory.CreatePerson) (created []peopledirectory.Person, err error) {
		txErr := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			created, err = batches.CreatePersons(txCtx, inputs)
			return err
		})
		if err == nil {
			err = txErr
		}
		return created, err
	}
	created, err := createPersons([]peopledirectory.CreatePerson{
		{FirstName: " Mia ", LastName: "Batch", Birthday: "2017-04-05", TagID: &card.ID},
		{FirstName: "Ben", LastName: "Batch"},
		{FirstName: "Cem", LastName: "Batch", AccountID: &account.ID},
	})
	require.NoError(t, err)
	require.Len(t, created, 3)
	assert.Equal(t, []string{"Mia", "Ben", "Cem"}, []string{created[0].FirstName, created[1].FirstName, created[2].FirstName}, "index-aligned and normalized")
	assert.Equal(t, "2017-04-05", created[0].Birthday)
	assert.Equal(t, card.ID, *created[0].TagID)
	assert.Equal(t, account.ID, *created[2].AccountID)
	assert.Empty(t, created[1].Birthday)
	assert.Nil(t, created[1].TagID)
	assert.Nil(t, created[1].AccountID)
	for _, person := range created {
		assert.Equal(t, testpkg.Tenant(t), person.TenantID)
		found, err := module.FindPerson(ctx, person.ID)
		require.NoError(t, err)
		assert.Equal(t, person.FirstName, found.FirstName, "the ID belongs to its input")
	}

	_, err = createPersons([]peopledirectory.CreatePerson{
		{FirstName: "Dora", LastName: "Batch", TagID: &other.ID},
		{FirstName: "Eva", LastName: "Batch", TagID: &card.ID},
	})
	require.ErrorIs(t, err, peopledirectory.ErrTagConflict)
	dora, err := module.SearchPersons(ctx, peopledirectory.PersonFilter{FirstNameEquals: "Dora", LastNameEquals: "Batch"})
	require.NoError(t, err)
	assert.Empty(t, dora, "every person or none")

	_, err = createPersons([]peopledirectory.CreatePerson{{FirstName: "Ok", LastName: "Batch"}, {FirstName: " ", LastName: "Batch"}})
	require.Error(t, err, "an input a single create rejects fails the whole call")
}
