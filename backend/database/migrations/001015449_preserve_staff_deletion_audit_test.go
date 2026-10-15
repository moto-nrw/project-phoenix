package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreserveStaffDeletionAuditMigration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)

	staff := testpkg.CreateTestStaffForTenant(t, db, school, "Lösch", "Nachweis")
	var deletionID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO audit.data_deletions (tenant_id, staff_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, ?, 'time_tracking_retention', 1, 'system')
		RETURNING id`, school, staff.ID).Scan(ctx, &deletionID))

	_, err := db.ExecContext(ctx, `DELETE FROM users.staff_school_memberships WHERE id = ?`, staff.ID)
	require.NoError(t, err)

	var preservedStaffID int64
	require.NoError(t, db.NewRaw(`SELECT staff_id FROM audit.data_deletions WHERE id = ?`, deletionID).
		Scan(ctx, &preservedStaffID), "deleting a staff member must not delete the proof of their deletions")
	assert.Equal(t, staff.ID, preservedStaffID, "the row keeps naming the deleted staff member")

	var foreignKeyExists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint
			WHERE conrelid = 'audit.data_deletions'::regclass
			  AND conname = 'fk_data_deletions_staff'
		)`).Scan(ctx, &foreignKeyExists))
	assert.False(t, foreignKeyExists)
}

func TestPreserveStaffDeletionAuditDownRefusesWhilePreservedRowsExist(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)

	staff := testpkg.CreateTestStaffForTenant(t, db, school, "Lösch", "Rückweg")
	var deletionID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO audit.data_deletions (tenant_id, staff_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, ?, 'time_tracking_retention', 1, 'system')
		RETURNING id`, school, staff.ID).Scan(ctx, &deletionID))
	_, err := db.ExecContext(ctx, `DELETE FROM users.staff_school_memberships WHERE id = ?`, staff.ID)
	require.NoError(t, err)

	require.ErrorContains(t, preserveStaffDeletionAuditDown(ctx, db),
		"1 preserved deletion audit records exist", "rolling back must not silently drop the proof of a deletion")

	_, err = db.ExecContext(ctx, `DELETE FROM audit.data_deletions WHERE id = ?`, deletionID)
	require.NoError(t, err)
	require.NoError(t, preserveStaffDeletionAuditDown(ctx, db))

	var foreignKeyExists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint
			WHERE conrelid = 'audit.data_deletions'::regclass
			  AND conname = 'fk_data_deletions_staff'
			  AND confrelid = 'users.staff_school_memberships'::regclass
		)`).Scan(ctx, &foreignKeyExists))
	assert.True(t, foreignKeyExists, "the down migration restores the cascading foreign key")
	require.NoError(t, preserveStaffDeletionAuditUp(ctx, db), "up is repeatable after a rollback")
}
