// Class-list entry tenant isolation (#2382).
//
// Class-list-only entries are names of children — personal data — so the
// acceptance criteria demand proof that one school can never read or write
// another school's entries. What guarantees that is not a WHERE clause but the
// RLS policy from migration 1.15.304, so these tests run through real
// phoenix_tenant transactions (tenant.WithTenantTx): a repository-level test
// with a plain context would pass even if the policy were missing.
package test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repoUsers "github.com/moto-nrw/project-phoenix/database/repositories/users"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TestClassListEntryTenantIsolation pins the read side: a tenant transaction
// sees only its own school's entries, on every query shape the repository
// offers, including a point lookup by a foreign primary key.
func TestClassListEntryTenantIsolation(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	tenantA := UniqueTestTenantID(t)
	tenantB := UniqueTestTenantID(t)
	EnsureTestTenant(t, db, tenantA)
	EnsureTestTenant(t, db, tenantB)
	defer CleanupTenantTestData(t, db, tenantA, tenantB)

	// Same class name in both schools — the realistic collision: "1a" exists
	// everywhere, and a leak would surface exactly here.
	entryA := CreateTestClassListEntryForTenant(t, db, tenantA, "Ida", "IsolationA", "iso1a")
	entryB := CreateTestClassListEntryForTenant(t, db, tenantB, "Ben", "IsolationB", "iso1a")

	repo := repoUsers.NewClassListEntryRepository(db)

	assertSeesOnly := func(t *testing.T, ownTenant int64, own, foreign *userModels.ClassListEntry) {
		t.Helper()
		err := tenant.WithTenantTx(context.Background(), db, ownTenant, func(txCtx context.Context, _ bun.Tx) error {
			entries, err := repo.List(txCtx, nil)
			require.NoError(t, err)
			ids := make(map[int64]bool, len(entries))
			for _, entry := range entries {
				ids[entry.ID] = true
			}
			assert.True(t, ids[own.ID], "a school must see its own class-list entry")
			assert.False(t, ids[foreign.ID], "a foreign school's class-list entry leaked into List")

			byClass, err := repo.FindBySchoolClass(txCtx, "iso1a")
			require.NoError(t, err)
			for _, entry := range byClass {
				assert.NotEqual(t, foreign.ID, entry.ID,
					"a foreign school's entry leaked through the shared class name")
			}

			_, err = repo.FindByID(txCtx, foreign.ID)
			require.Error(t, err, "a foreign entry must not be fetchable by id")
			assert.ErrorIs(t, err, sql.ErrNoRows)
			return nil
		})
		require.NoError(t, err)
	}

	assertSeesOnly(t, tenantA, entryA, entryB)
	assertSeesOnly(t, tenantB, entryB, entryA)
}

// TestClassListEntryForeignTenantWriteRejected pins the fail-closed write
// direction: a row stamped with another school's tenant_id must be refused by
// the policy's WITH CHECK instead of silently landing in the foreign school.
func TestClassListEntryForeignTenantWriteRejected(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	tenantA := UniqueTestTenantID(t)
	tenantB := UniqueTestTenantID(t)
	EnsureTestTenant(t, db, tenantA)
	EnsureTestTenant(t, db, tenantB)
	defer CleanupTenantTestData(t, db, tenantA, tenantB)

	repo := repoUsers.NewClassListEntryRepository(db)
	smuggled := &userModels.ClassListEntry{
		FirstName:   "Fritz",
		LastName:    "Fremdschule",
		SchoolClass: "iso2b",
	}
	// EnsureTenantID only fills a zero tenant_id, so the preset foreign id
	// survives to the INSERT — exactly the write WITH CHECK must reject.
	smuggled.SetTenantID(tenantB)

	err := tenant.WithTenantTx(context.Background(), db, tenantA, func(txCtx context.Context, _ bun.Tx) error {
		return repo.Create(txCtx, smuggled)
	})
	require.Error(t, err, "the database must refuse an entry stamped with a foreign tenant_id")
	if smuggled.ID != 0 {
		CleanupClassListEntryFixtures(t, db, smuggled.ID)
	}
}
