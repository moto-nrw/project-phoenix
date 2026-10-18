package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeletionLogAcceptsAttendanceRetentionRowsWithoutASubject(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)

	_, err := db.ExecContext(context.Background(), `
		INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, ?, 1, 'system')`, school, attendanceRetentionKind)
	assert.NoError(t, err, "the attendance retention logs a data kind without a subject")
}

func TestAttendanceRetentionRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)
	logRow := func() error {
		_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
			VALUES (?, ?, 1, 'system')`, school, attendanceRetentionKind)
		return err
	}

	require.NoError(t, attendanceRetentionDown(ctx, db))
	require.ErrorContains(t, logRow(), "chk_data_deletions_subject", "before 1.15.460 the data kind needs a subject")
	_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, 'setting_audit_retention', 1, 'system')`, school)
	require.NoError(t, err, "the rollback keeps the data kinds of 1.15.454")

	require.NoError(t, attendanceRetentionUp(ctx, db))
	require.NoError(t, logRow())
	assert.ErrorContains(t, attendanceRetentionDown(ctx, db), "attendance retention rows exist",
		"the subject check cannot be restored over logged retention rows")
}
