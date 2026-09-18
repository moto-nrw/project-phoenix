// Class-list entry tenant isolation (#2382).
//
// Class-list-only entries are names of children — personal data — so the
// acceptance criteria demand proof that one school can never read or write
// another school's entries. The read side and the owner's own tenant stamping
// are proven against the School Membership capability that owns the rows
// (modules/schoolmembership/compose). What is left here is the direction no
// capability can reach at all: a raw INSERT carrying a foreign tenant_id,
// which only the RLS policy from migration 1.15.306 can refuse. It runs
// through a real phoenix_tenant transaction (tenant.WithTenantTx) — with a
// plain context it would pass even if the policy were missing.
package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

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
