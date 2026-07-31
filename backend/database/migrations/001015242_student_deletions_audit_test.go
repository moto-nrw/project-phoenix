package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func studentDeletionAuditColumnExists(t *testing.T, db *bun.DB, column string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'audit'
			  AND table_name = 'student_deletions'
			  AND column_name = ?
		)
	`, column).Scan(context.Background(), &exists))
	return exists
}

func TestStudentDeletionsAuditMigration(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	// Exercise the migration from a clean relation and leave the fully migrated
	// schema behind for packages sharing the test database.
	require.NoError(t, studentDeletionsAuditDown(ctx, db))
	t.Cleanup(func() {
		require.NoError(t, studentDeletionsAuditUp(context.Background(), db))
	})
	require.NoError(t, studentDeletionsAuditUp(ctx, db))
	require.NoError(t, studentDeletionsAuditUp(ctx, db), "up must be idempotent")

	for _, column := range []string{
		"tenant_id",
		"student_id",
		"actor_account_id",
		"reason",
		"counts",
		"deleted_at",
	} {
		assert.True(t, studentDeletionAuditColumnExists(t, db, column), column)
	}

	var policyCount int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM pg_policies
		WHERE schemaname = 'audit'
		  AND tablename = 'student_deletions'
		  AND policyname = 'tenant_isolation_audit_student_deletions'
	`).Scan(ctx, &policyCount))
	assert.Equal(t, 1, policyCount)

	for privilege, want := range map[string]bool{
		"SELECT":   true,
		"INSERT":   true,
		"UPDATE":   false,
		"DELETE":   false,
		"TRUNCATE": false,
	} {
		var got bool
		require.NoError(t, db.NewRaw(
			`SELECT has_table_privilege('phoenix_tenant', 'audit.student_deletions', ?)`,
			privilege,
		).Scan(ctx, &got))
		assert.Equal(t, want, got, privilege)
	}

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("platform.schools").Where("id = ?", tenantID).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("platform.organizations").Where("id = ?", tenantID).Exec(context.Background())
	})
	_, err := db.NewRaw(`
		INSERT INTO audit.student_deletions
			(tenant_id, student_id, actor_account_id, reason, counts)
		VALUES (?, ?, ?, 'test_data', '{}'::jsonb)
	`, tenantID, tenantID+1, tenantID+2).Exec(ctx)
	require.NoError(t, err)
}
