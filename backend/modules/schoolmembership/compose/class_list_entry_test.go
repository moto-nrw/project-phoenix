package compose

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createEntry(t *testing.T, ctx context.Context, module *schoolmembership.Module, firstName, lastName, schoolClass string) schoolmembership.ClassListEntry {
	t.Helper()
	entry, err := module.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: firstName, LastName: lastName, SchoolClass: schoolClass},
	})
	require.NoError(t, err)
	return entry
}

func TestModuleRunsTheClassListEntryLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	actor := testpkg.CreateTestAccount(t, db, "cle-module-actor@test.local")

	created, err := module.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: " Zoe ", LastName: "Aalders", SchoolClass: "7z"},
		CreatedBy:            &actor.ID,
	})
	require.NoError(t, err)
	assert.Positive(t, created.ID)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)
	assert.Equal(t, "Zoe", created.FirstName, "the facade trims before persisting")
	require.NotNil(t, created.CreatedBy)
	assert.Equal(t, actor.ID, *created.CreatedBy)

	found, err := module.FindClassListEntry(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, found)

	updated, err := module.UpdateClassListEntry(ctx, schoolmembership.UpdateClassListEntry{ID: created.ID, ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "Zoe", LastName: "Aalders", SchoolClass: "7z-b",
	}})
	require.NoError(t, err)
	assert.Equal(t, "7z-b", updated.SchoolClass)
	assert.Equal(t, created.CreatedBy, updated.CreatedBy, "an update never touches the creating account")

	listed, err := module.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)

	require.NoError(t, module.DeleteClassListEntry(ctx, created.ID))
	_, err = module.FindClassListEntry(ctx, created.ID)
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryNotFound)
	require.ErrorIs(t, module.DeleteClassListEntry(ctx, created.ID), schoolmembership.ErrClassListEntryNotFound)
	_, err = module.UpdateClassListEntry(ctx, schoolmembership.UpdateClassListEntry{ID: created.ID, ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "Zoe", LastName: "Aalders", SchoolClass: "7z",
	}})
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryNotFound)
}

func TestModuleFiltersAndOrdersClassListEntries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	zoe := createEntry(t, ctx, module, "Zoe", "Aalders", "1a")
	ben := createEntry(t, ctx, module, "Ben", "aalders", "1a")
	anna := createEntry(t, ctx, module, "Anna", "Zander", " 1A ")
	other := createEntry(t, ctx, module, "Zoe", "Aalders", "2b")

	byClass, err := module.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{SchoolClass: "1a"})
	require.NoError(t, err)
	require.Len(t, byClass, 3, "the class match folds case and whitespace")
	assert.Equal(t, []int64{ben.ID, zoe.ID, anna.ID}, []int64{byClass[0].ID, byClass[1].ID, byClass[2].ID},
		"ordered by case-folded last name, then first name")

	byName, err := module.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{FirstName: " zoe", LastName: "AALDERS ", SchoolClass: "1a"})
	require.NoError(t, err)
	require.Len(t, byName, 1)
	assert.Equal(t, zoe.ID, byName[0].ID)

	byIDs, err := module.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{IDs: []int64{other.ID, anna.ID}})
	require.NoError(t, err)
	require.Len(t, byIDs, 2)

	none, err := module.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{IDs: []int64{}})
	require.NoError(t, err)
	assert.Empty(t, none, "an empty ID set lists nothing instead of everything")
}

func TestModuleReportsClassListEntryDuplicatesWithTheIndexCause(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	createEntry(t, ctx, module, "Zoe", "Aalders", "1a")
	_, err := module.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "zoe", LastName: "AALDERS", SchoolClass: " 1A "},
	})
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryDuplicate)
	assert.True(t, strings.Contains(err.Error(), "uniq_class_list_entries_name_class"),
		"the unique index stays visible in the chain: %v", err)
	assert.Equal(t, "class_list_entry_conflict", schoolmembership.ErrorCode(err))

	second := createEntry(t, ctx, module, "Zoe", "Aalders", "1b")
	_, err = module.UpdateClassListEntry(ctx, schoolmembership.UpdateClassListEntry{ID: second.ID, ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "Zoe", LastName: "Aalders", SchoolClass: "1a",
	}})
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryDuplicate)
}

func TestModuleTenantIsolationHidesAnotherTenantsClassListEntries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	entry := createEntry(t, ctx, module, "Ida", "Isolation", "iso1a")
	otherCtx, _ := otherTenantContext(t, db)

	// The same class name exists in both schools: the realistic collision.
	foreign := createEntry(t, otherCtx, module, "Ben", "Fremd", "iso1a")

	_, err := module.FindClassListEntry(otherCtx, entry.ID)
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryNotFound)

	listed, err := module.ListClassListEntries(otherCtx, schoolmembership.ClassListEntryFilter{SchoolClass: "iso1a"})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, foreign.ID, listed[0].ID)

	_, err = module.UpdateClassListEntry(otherCtx, schoolmembership.UpdateClassListEntry{ID: entry.ID, ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "Ida", LastName: "Isolation", SchoolClass: "iso9z",
	}})
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryNotFound)
	require.ErrorIs(t, module.DeleteClassListEntry(otherCtx, entry.ID), schoolmembership.ErrClassListEntryNotFound)

	stillThere, err := module.FindClassListEntry(ctx, entry.ID)
	require.NoError(t, err)
	assert.Equal(t, "iso1a", stillThere.SchoolClass)
	own, err := module.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{})
	require.NoError(t, err)
	require.Len(t, own, 1)
	assert.Equal(t, entry.ID, own[0].ID)
}

func TestModuleClassListEntryWriteRollsBackWithOuterTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	wantErr := errors.New("abort outer transaction")

	var createdID int64
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		created := createEntry(t, txCtx, module, "Rolled", "Back", "1a")
		createdID = created.ID
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	_, err = module.FindClassListEntry(ctx, createdID)
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryNotFound)
}

func TestModuleObservesClassListEntryOperations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var observations []Observation
	module := buildModule(t, db, func(observation Observation) {
		observations = append(observations, observation)
	})
	ctx := testpkg.Ctx(t)

	_, err := module.FindClassListEntry(ctx, 9_223_372_036_854_775_000)
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryNotFound)
	require.Len(t, observations, 1)
	assert.Equal(t, "find_class_list_entry", observations[0].Operation)
	assert.Equal(t, "not_found", schoolmembership.ErrorCode(observations[0].Err))
	assert.EqualValues(t, 1, observations[0].Stats.Queries)

	createEntry(t, ctx, module, "Observed", "Create", "1a")
	require.Len(t, observations, 2)
	assert.Equal(t, "create_class_list_entry", observations[1].Operation)
	assert.Equal(t, "none", schoolmembership.ErrorCode(observations[1].Err))
	assert.EqualValues(t, 1, observations[1].Stats.Rows)
	assert.Positive(t, observations[1].Stats.StatementDuration)
}
