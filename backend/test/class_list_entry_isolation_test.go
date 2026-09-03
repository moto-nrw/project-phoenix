// Class-list entry tenant isolation (#2382).
//
// Class-list-only entries are names of children — personal data — so the
// acceptance criteria demand proof that one school can never read or write
// another school's entries. What guarantees that is not a WHERE clause but the
// RLS policy from migration 1.15.306, so these tests run through real
// phoenix_tenant transactions (tenant.WithTenantTx): a repository-level test
// with a plain context would pass even if the policy were missing.
//
// Since #2668 the repository is an adapter over the School Membership owner,
// so the read side drives the factory-bound repository and the write side
// inserts the smuggled row directly: the owner stamps the transaction's
// tenant on every insert, so only a raw statement can still carry a foreign
// tenant_id to the policy's WITH CHECK.
package test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// TestClassListEntryTenantIsolation pins the read side: a tenant transaction
// sees only its own school's entries, on every query shape the repository
// offers, including a point lookup by a foreign primary key.
func TestClassListEntryTenantIsolation(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)

	tenantA := UniqueTestTenantID(t)
	tenantB := UniqueTestTenantID(t)
	EnsureTestTenant(t, db, tenantA)
	EnsureTestTenant(t, db, tenantB)

	// Same class name in both schools — the realistic collision: "1a" exists
	// everywhere, and a leak would surface exactly here.
	entryA := CreateTestClassListEntryForTenant(t, db, tenantA, "Ida", "IsolationA", "iso1a")
	entryB := CreateTestClassListEntryForTenant(t, db, tenantB, "Ben", "IsolationB", "iso1a")

	repo := repositories.NewFactory(db).ClassListEntry

	assertSeesOnly := func(t *testing.T, ownTenant int64, own, foreign *userModels.ClassListEntry) {
		t.Helper()
		err := WithTenantTx(t, context.Background(), db, ownTenant, func(txCtx context.Context, _ bun.Tx) error {
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
	t.Parallel()

	db := SetupTestDB(t)

	tenantA := UniqueTestTenantID(t)
	tenantB := UniqueTestTenantID(t)
	EnsureTestTenant(t, db, tenantA)
	EnsureTestTenant(t, db, tenantB)

	smuggled := &userModels.ClassListEntry{
		FirstName:   "Fritz",
		LastName:    "Fremdschule",
		SchoolClass: "iso2b",
	}
	// The preset foreign id travels straight to the INSERT — exactly the
	// write the policy's WITH CHECK must reject.
	smuggled.SetTenantID(tenantB)

	err := WithTenantTx(t, context.Background(), db, tenantA, func(txCtx context.Context, tx bun.Tx) error {
		_, err := tx.NewInsert().Model(smuggled).ModelTableExpr(`users.class_list_entries AS "class_list_entry"`).Exec(txCtx)
		return err
	})
	require.Error(t, err, "the database must refuse an entry stamped with a foreign tenant_id")
}
