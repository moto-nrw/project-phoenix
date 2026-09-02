package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func buildModule(t *testing.T, db *bun.DB, observations ...func(Observation)) *peopledirectory.Module {
	t.Helper()
	observe := func(Observation) {}
	if len(observations) > 0 {
		observe = observations[0]
	}
	module, err := New(Dependencies{DB: db, Observe: observe})
	require.NoError(t, err)
	return module
}

func otherTenantContext(t *testing.T, db *bun.DB) (context.Context, int64) {
	t.Helper()
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	return tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), otherTenantID), otherTenantID
}

func TestModuleCreatesReadsUpdatesAndSoftDeletesOneTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	created, err := module.CreatePerson(ctx, peopledirectory.CreatePerson{FirstName: " Mia ", LastName: "Directory"})
	require.NoError(t, err)
	assert.Positive(t, created.ID)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)
	assert.Equal(t, "Mia", created.FirstName)

	found, err := module.FindPerson(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Directory", found.LastName)

	updated, err := module.UpdatePerson(ctx, peopledirectory.UpdatePerson{ID: created.ID, FirstName: "Mia", LastName: "Renamed"})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", updated.LastName)

	listed, err := module.SearchPersons(ctx, peopledirectory.PersonFilter{LastNamePrefix: "ren"})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)

	require.NoError(t, module.DeletePerson(ctx, created.ID))
	_, err = module.FindPerson(ctx, created.ID)
	require.ErrorIs(t, err, peopledirectory.ErrPersonNotFound)
	require.ErrorIs(t, module.DeletePerson(ctx, created.ID), peopledirectory.ErrPersonNotFound)

	byIDs, err := module.ListPersonsByID(ctx, []int64{created.ID})
	require.NoError(t, err)
	assert.Empty(t, byIDs, "soft-deleted persons stay out of every listing")
}

func TestModuleTenantIsolationHidesAnotherTenantsPersons(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	person := testpkg.CreateTestPerson(t, db, "Isolated", "TenantA")

	otherCtx, _ := otherTenantContext(t, db)
	_, err := module.FindPerson(otherCtx, person.ID)
	require.ErrorIs(t, err, peopledirectory.ErrPersonNotFound)

	listed, err := module.ListPersonsByID(otherCtx, []int64{person.ID})
	require.NoError(t, err)
	assert.Empty(t, listed)

	searched, err := module.SearchPersons(otherCtx, peopledirectory.PersonFilter{LastNamePrefix: "TenantA"})
	require.NoError(t, err)
	assert.Empty(t, searched)

	_, err = module.UpdatePerson(otherCtx, peopledirectory.UpdatePerson{ID: person.ID, FirstName: "Hijacked", LastName: "Name"})
	require.ErrorIs(t, err, peopledirectory.ErrPersonNotFound)
	require.ErrorIs(t, module.DeletePerson(otherCtx, person.ID), peopledirectory.ErrPersonNotFound)

	// Visiting-student names are the one deliberate way past the boundary,
	// and they resolve even from inside the other tenant's transaction.
	err = tenant.WithinCurrentTenant(otherCtx, func(txCtx context.Context) error {
		across, err := module.ListPersonsAcrossTenantsByID(txCtx, []int64{person.ID})
		require.NoError(t, err)
		require.Len(t, across, 1)
		assert.Equal(t, testpkg.Tenant(t), across[0].TenantID)
		assert.Equal(t, "Isolated", across[0].FirstName)
		return nil
	})
	require.NoError(t, err)
}

func TestModuleAdminTransactionReadsAcrossTenantsAndCountsPerTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	person := testpkg.CreateTestPerson(t, db, "Counted", "Person")
	otherCtx, otherTenantID := otherTenantContext(t, db)
	other, err := module.CreatePerson(otherCtx, peopledirectory.CreatePerson{FirstName: "Other", LastName: "Tenant"})
	require.NoError(t, err)

	err = tenant.WithinAdmin(testpkg.Ctx(t), func(adminCtx context.Context) error {
		listed, err := module.ListPersonsByID(adminCtx, []int64{person.ID, other.ID})
		require.NoError(t, err)
		assert.Len(t, listed, 2)

		counts, err := module.CountPersonsByTenant(adminCtx)
		require.NoError(t, err)
		assert.Equal(t, 1, counts[testpkg.Tenant(t)])
		assert.Equal(t, 1, counts[otherTenantID])
		return nil
	})
	require.NoError(t, err)
}

func TestModuleWriteRollsBackWithOuterTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	wantErr := errors.New("abort outer transaction")

	var createdID int64
	err := tenant.WithinCurrentTenant(testpkg.Ctx(t), func(txCtx context.Context) error {
		created, createErr := module.CreatePerson(txCtx, peopledirectory.CreatePerson{FirstName: "Rolled", LastName: "Back"})
		require.NoError(t, createErr)
		createdID = created.ID
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	_, err = module.FindPerson(testpkg.Ctx(t), createdID)
	require.ErrorIs(t, err, peopledirectory.ErrPersonNotFound)
}

func TestModuleLinksAccountsAndRejectsSecondHolder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "directory-link@example.com")
	first := testpkg.CreateTestPerson(t, db, "First", "Holder")
	second := testpkg.CreateTestPerson(t, db, "Second", "Holder")

	require.NoError(t, module.LinkAccount(ctx, first.ID, account.ID))
	byAccount, err := module.FindPersonByAccount(ctx, account.ID)
	require.NoError(t, err)
	assert.Equal(t, first.ID, byAccount.ID)

	require.ErrorIs(t, module.LinkAccount(ctx, second.ID, account.ID), peopledirectory.ErrAccountConflict)

	require.NoError(t, module.UnlinkAccount(ctx, first.ID))
	_, err = module.FindPersonByAccount(ctx, account.ID)
	require.ErrorIs(t, err, peopledirectory.ErrPersonNotFound)

	byAccounts, err := module.ListPersonsByAccount(ctx, []int64{account.ID})
	require.NoError(t, err)
	assert.Empty(t, byAccounts)
}

func TestModuleReleasesAndRestoresTagsIdempotently(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	card := testpkg.CreateTestRFIDCard(t, db, "AABBCC")
	holder := testpkg.CreateTestPerson(t, db, "Tag", "Holder")
	untagged := testpkg.CreateTestPerson(t, db, "No", "Tag")

	require.NoError(t, module.LinkTag(ctx, holder.ID, card.ID))
	byTag, err := module.FindPersonByTag(ctx, card.ID)
	require.NoError(t, err)
	assert.Equal(t, holder.ID, byTag.ID)
	require.ErrorIs(t, module.LinkTag(ctx, untagged.ID, card.ID), peopledirectory.ErrTagConflict)

	released, err := module.ReleaseTags(ctx, []int64{holder.ID, untagged.ID})
	require.NoError(t, err)
	require.Len(t, released, 1)
	assert.Equal(t, holder.ID, released[0].PersonID)
	assert.Equal(t, card.ID, released[0].TagID)

	again, err := module.ReleaseTags(ctx, []int64{holder.ID})
	require.NoError(t, err)
	assert.Empty(t, again, "a second release finds nothing to release")

	restored, err := module.RestoreTag(ctx, holder.ID, card.ID)
	require.NoError(t, err)
	assert.True(t, restored)
	restoredAgain, err := module.RestoreTag(ctx, untagged.ID, card.ID)
	require.NoError(t, err)
	assert.False(t, restoredAgain, "the current holder wins")

	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		restored, err := module.RestoreTag(txCtx, untagged.ID, card.ID)
		require.NoError(t, err)
		assert.False(t, restored)
		// The refused restore left the outer transaction usable.
		_, err = module.FindPerson(txCtx, untagged.ID)
		return err
	})
	require.NoError(t, err)
}

func TestModuleObservesStableErrorCodes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var observations []Observation
	module := buildModule(t, db, func(observation Observation) {
		observations = append(observations, observation)
	})

	_, err := module.FindPerson(testpkg.Ctx(t), 9_223_372_036_854_775_000)
	require.ErrorIs(t, err, peopledirectory.ErrPersonNotFound)
	require.Len(t, observations, 1)
	assert.Equal(t, "find_person", observations[0].Operation)
	assert.Equal(t, "not_found", peopledirectory.ErrorCode(observations[0].Err))
	assert.EqualValues(t, 1, observations[0].Stats.Queries)
	assert.Positive(t, observations[0].Stats.StatementDuration)
}

func TestModuleKeepsPersistenceErrorsVisible(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)

	missingTenantID := testpkg.UniqueTestTenantID(t)
	missingTenantCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), missingTenantID)
	_, err := module.CreatePerson(missingTenantCtx, peopledirectory.CreatePerson{FirstName: "Ghost", LastName: "Tenant"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, peopledirectory.ErrInvalidPerson)
	assert.NotErrorIs(t, err, peopledirectory.ErrPersonNotFound)
}
