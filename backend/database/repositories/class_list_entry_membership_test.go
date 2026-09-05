package repositories_test

// The class-list entry repository is an adapter over the School Membership
// owner (#2668). The tests below pin what the composition keeps of the legacy
// contract: the not-found shape callers classify on, idempotent deletes, and
// the class-scoped lookups with their order. The write path (created row,
// unique-index collision by index name, audit trail) is covered by the
// services/users tests that drive this repository through the service.

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassListEntryAdapter_KeepsTheLegacyErrorContracts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)

	existing := testpkg.CreateTestClassListEntry(t, db, "Zoe", "Aalders", "1a")
	absent := missingID(existing.ID)

	t.Run("FindByID not found", func(t *testing.T) {
		_, err := factory.ClassListEntry.FindByID(ctx, absent)
		require.Error(t, err)
		assert.True(t, testpkg.IsNotFoundError(err), "callers classify entry lookups with sql.ErrNoRows")
	})

	t.Run("FindByID returns the persisted row", func(t *testing.T) {
		found, err := factory.ClassListEntry.FindByID(ctx, existing.ID)
		require.NoError(t, err)
		assert.Equal(t, existing.ID, found.ID)
		assert.Equal(t, testpkg.Tenant(t), found.GetTenantID())
		assert.Equal(t, "Aalders", found.LastName)
		assert.False(t, found.CreatedAt.IsZero())
	})

	t.Run("Delete is idempotent", func(t *testing.T) {
		require.NoError(t, factory.ClassListEntry.Delete(ctx, absent))
	})
}

func TestClassListEntryAdapter_ClassLookupsMatchCaseInsensitivelyAndOrderByName(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)

	zander := testpkg.CreateTestClassListEntry(t, db, "Anna", "Zander", "3c")
	zoe := testpkg.CreateTestClassListEntry(t, db, "Zoe", "Aalders", "3c")
	ben := testpkg.CreateTestClassListEntry(t, db, "Ben", "aalders", " 3C ")
	_ = testpkg.CreateTestClassListEntry(t, db, "Zoe", "Aalders", "4d")

	byClass, err := factory.ClassListEntry.FindBySchoolClass(ctx, "3c")
	require.NoError(t, err)
	require.Len(t, byClass, 3)
	assert.Equal(t, []int64{ben.ID, zoe.ID, zander.ID}, []int64{byClass[0].ID, byClass[1].ID, byClass[2].ID})

	byName, err := factory.ClassListEntry.FindByNameAndClass(ctx, " zoe ", "AALDERS", "3c")
	require.NoError(t, err)
	require.Len(t, byName, 1)
	assert.Equal(t, zoe.ID, byName[0].ID)

	all, err := factory.ClassListEntry.List(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, all, 4)

	_, err = factory.ClassListEntry.List(ctx, map[string]any{"first_name": "Zoe"})
	require.Error(t, err, "an unsupported filter is an explicit error, never silently ignored")
}
