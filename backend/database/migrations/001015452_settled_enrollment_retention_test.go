package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettledEnrollmentRetentionRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)
	logRow := func() error {
		_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
			VALUES (?, ?, 1, 'system')`, school, settledEnrollmentRetentionKind)
		return err
	}

	require.NoError(t, settledEnrollmentRetentionDown(ctx, db))
	require.ErrorContains(t, logRow(), "chk_data_deletions_subject", "before 1.15.452 the data kind needs a subject")
	_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, 'submission_rate_limit_retention', 1, 'system')`, school)
	require.NoError(t, err, "the rollback keeps the data kinds of 1.15.449")

	require.NoError(t, settledEnrollmentRetentionUp(ctx, db))
	require.NoError(t, logRow())
	assert.ErrorContains(t, settledEnrollmentRetentionDown(ctx, db), "settled enrollment retention rows exist",
		"the subject check cannot be restored over logged retention rows")
}
