package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPreserveStaffDeletionAuditMigration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	staffID, deletionID := deleteStaffWithLoggedDeletion(t, db, "Nachweis")

	var preservedStaffID int64
	require.NoError(t, db.NewRaw(`SELECT staff_id FROM audit.data_deletions WHERE id = ?`, deletionID).
		Scan(ctx, &preservedStaffID), "deleting a staff member must not delete the proof of their deletions")
	assert.Equal(t, staffID, preservedStaffID, "the row keeps naming the deleted staff member")
	assert.Empty(t, staffDeletionForeignKeyTarget(t, db), "no foreign key may cascade into the deletion log")
}

func TestPreserveStaffDeletionAuditDownRefusesWhilePreservedRowsExist(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	_, deletionID := deleteStaffWithLoggedDeletion(t, db, "Rückweg")

	require.ErrorContains(t, preserveStaffDeletionAuditDown(ctx, db),
		"1 preserved deletion audit records exist", "rolling back must not silently drop the proof of a deletion")

	_, err := db.ExecContext(ctx, `DELETE FROM audit.data_deletions WHERE id = ?`, deletionID)
	require.NoError(t, err)
	require.NoError(t, preserveStaffDeletionAuditDown(ctx, db))
	assert.Equal(t, "users.staff_school_memberships", staffDeletionForeignKeyTarget(t, db),
		"the down migration restores the cascading foreign key")
	require.NoError(t, preserveStaffDeletionAuditUp(ctx, db), "up is repeatable after a rollback")
}

// deleteStaffWithLoggedDeletion logs a deletion for a new staff member and
// then deletes the membership row itself. The delete is raw SQL on purpose:
// the foreign key's own referential action is what these tests observe.
func deleteStaffWithLoggedDeletion(t *testing.T, db *bun.DB, lastName string) (staffID, deletionID int64) {
	t.Helper()
	ctx := context.Background()
	tenantID := testpkg.Tenant(t)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Lösch", lastName)
	require.NoError(t, db.NewRaw(`
		INSERT INTO audit.data_deletions (tenant_id, staff_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, ?, 'time_tracking_retention', 1, 'system')
		RETURNING id`, tenantID, staff.ID).Scan(ctx, &deletionID))
	_, err := db.ExecContext(ctx, `DELETE FROM users.staff_school_memberships WHERE id = ?`, staff.ID)
	require.NoError(t, err)
	return staff.ID, deletionID
}

// staffDeletionForeignKeyTarget names the table fk_data_deletions_staff
// references, or "" when the constraint does not exist.
func staffDeletionForeignKeyTarget(t *testing.T, db *bun.DB) string {
	t.Helper()
	var target string
	require.NoError(t, db.NewRaw(`
		SELECT COALESCE((
			SELECT referenced.relnamespace::regnamespace::text || '.' || referenced.relname
			FROM pg_constraint JOIN pg_class AS referenced ON referenced.oid = confrelid
			WHERE conrelid = 'audit.data_deletions'::regclass
			  AND conname = 'fk_data_deletions_staff'
		), '')`).Scan(context.Background(), &target))
	return target
}
