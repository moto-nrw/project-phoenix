package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffSickNoteRetentionRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)
	functionExists := func() bool {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT to_regprocedure('users.delete_expired_staff_sick_notes(bigint, timestamptz)') IS NOT NULL`).
			Scan(ctx, &exists))
		return exists
	}
	logRow := func() error {
		_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
			VALUES (?, ?, 1, 'system')`, school, staffSickNoteRetentionKind)
		return err
	}

	require.NoError(t, staffSickNoteRetentionDown(ctx, db))
	assert.False(t, functionExists())
	require.ErrorContains(t, logRow(), "chk_data_deletions_subject", "before 1.15.453 the data kind needs a subject")
	_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, 'settled_enrollment_retention', 1, 'system')`, school)
	require.NoError(t, err, "the rollback keeps the data kinds of 1.15.452")

	require.NoError(t, staffSickNoteRetentionUp(ctx, db))
	assert.True(t, functionExists())
	require.NoError(t, logRow())
	assert.ErrorContains(t, staffSickNoteRetentionDown(ctx, db), "staff sick note retention rows exist",
		"the subject check cannot be restored over logged retention rows")
}
