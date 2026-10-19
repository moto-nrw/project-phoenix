package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeletionLogAcceptsSchoolContentRetentionRowsWithoutASubject(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)

	for _, kind := range schoolContentRetentionKinds {
		_, err := db.ExecContext(context.Background(), `
			INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
			VALUES (?, ?, 1, 'system')`, school, kind)
		assert.NoError(t, err, "%s logs a data kind without a subject", kind)
	}
}

func TestSchoolContentRetentionRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)
	functionExists := func() bool {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT to_regprocedure('documents.delete_removed_files(bigint, timestamptz)') IS NOT NULL`).Scan(ctx, &exists))
		return exists
	}
	logRow := func() error {
		_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
			VALUES (?, 'appointment_retention', 1, 'system')`, school)
		return err
	}

	require.NoError(t, schoolContentRetentionDown(ctx, db))
	assert.False(t, functionExists())
	require.ErrorContains(t, logRow(), "chk_data_deletions_subject", "before 1.15.465 the data kind needs a subject")
	_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, 'guardian_retention', 1, 'system')`, school)
	require.NoError(t, err, "the rollback keeps the data kinds of 1.15.464")

	require.NoError(t, schoolContentRetentionUp(ctx, db))
	assert.True(t, functionExists())
	require.NoError(t, logRow())
	assert.ErrorContains(t, schoolContentRetentionDown(ctx, db), "school content retention rows exist",
		"the subject check cannot be restored over logged retention rows")
}
