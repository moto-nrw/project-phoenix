package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func insertStudentTombstone(ctx context.Context, db bun.IDB, tenantID int64, actor *int64, reason string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO audit.student_deletions (tenant_id, student_id, actor_account_id, reason, counts)
		VALUES (?, 4711, ?, ?, '{}'::jsonb)`, tenantID, actor, reason)
	return err
}

func TestStudentRetentionMigrationAllowsOnlyAutomaticTombstonesWithoutActor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.Tenant(t)
	actor := int64(42)

	require.NoError(t, studentRetentionUp(ctx, db), "up is repeatable")
	require.NoError(t, insertStudentTombstone(ctx, db, tenantID, nil, "retention_expired"), "the nightly run records no actor")
	require.NoError(t, insertStudentTombstone(ctx, db, tenantID, &actor, "retention_expired"), "a confirmed deletion keeps its actor")
	err := insertStudentTombstone(ctx, db, tenantID, nil, "privacy_request")
	require.ErrorContains(t, err, "chk_student_deletions_actor", "only the automatic deletion may go without an actor")
}

func TestStudentRetentionDownRefusesWhileAutomaticTombstonesExist(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.Tenant(t)
	require.NoError(t, insertStudentTombstone(ctx, db, tenantID, nil, "retention_expired"))

	require.ErrorContains(t, studentRetentionDown(ctx, db), "1 automatic deletions exist",
		"rolling back must not invent an actor for an automatic deletion")

	_, err := db.ExecContext(ctx, `DELETE FROM audit.student_deletions WHERE actor_account_id IS NULL`)
	require.NoError(t, err)
	require.NoError(t, studentRetentionDown(ctx, db))
	var nullable string
	require.NoError(t, db.NewRaw(`SELECT is_nullable FROM information_schema.columns
		WHERE table_schema = 'audit' AND table_name = 'student_deletions' AND column_name = 'actor_account_id'`).Scan(ctx, &nullable))
	assert.Equal(t, "NO", nullable, "down requires an actor again")
	require.NoError(t, studentRetentionUp(ctx, db), "up is repeatable after a rollback")
}
