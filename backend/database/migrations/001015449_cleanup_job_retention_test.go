package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestDeclarationMarkersGoWithTheirRelationship(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)
	child := testpkg.CreateTestStudentForTenant(t, db, school, "Marker", "Kind", "1a")
	mother := testpkg.CreateTestGuardianProfileForTenant(t, db, school, "Marker", "Mutter", "marker-mutter@example.com")
	father := testpkg.CreateTestGuardianProfileForTenant(t, db, school, "Marker", "Vater", "marker-vater@example.com")
	kept := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, school, child.ID, mother.ID, "primary_guardian")
	orphaned := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, school, child.ID, father.ID, "co_guardian")

	markers := func() []int64 {
		var ids []int64
		require.NoError(t, db.NewRaw(`SELECT relationship_id FROM auth.parent_declaration_permission_grants
			WHERE tenant_id = ? ORDER BY relationship_id`, school).Scan(ctx, &ids))
		return ids
	}
	deleteRelationship := func(id int64) {
		_, err := db.ExecContext(ctx, `DELETE FROM users.student_guardian_relationships WHERE tenant_id = ? AND id = ?`, school, id)
		require.NoError(t, err)
	}

	// Before 1.15.449 a marker outlived its relationship.
	require.NoError(t, cleanupJobRetentionDown(ctx, db))
	_, err := db.ExecContext(ctx, `INSERT INTO auth.parent_declaration_permission_grants (tenant_id, relationship_id)
		VALUES (?, ?), (?, ?)`, school, kept.ID, school, orphaned.ID)
	require.NoError(t, err)
	deleteRelationship(orphaned.ID)
	require.Equal(t, []int64{kept.ID, orphaned.ID}, markers())

	require.NoError(t, cleanupJobRetentionUp(ctx, db))
	assert.Equal(t, []int64{kept.ID}, markers(), "a marker of a relationship already gone is removed")

	deleteRelationship(kept.ID)
	assert.Empty(t, markers(), "a marker goes with its relationship")
	assertCleanupFunctionsExist(t, db, true)
}

func TestCleanupJobRetentionRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	require.NoError(t, cleanupJobRetentionDown(ctx, db))
	assertCleanupFunctionsExist(t, db, false)
	require.NoError(t, cleanupJobRetentionUp(ctx, db))
	assertCleanupFunctionsExist(t, db, true)

	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)
	_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, 'file_cleanup_retention', 1, 'system')`, school)
	require.NoError(t, err)
	assert.ErrorContains(t, cleanupJobRetentionDown(ctx, db), "cleanup job retention rows exist",
		"the subject check cannot be restored over logged retention rows")
}

func assertCleanupFunctionsExist(t *testing.T, db *bun.DB, want bool) {
	t.Helper()
	for _, function := range finishedCleanupFunctions {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT to_regprocedure(? || '(bigint, timestamp with time zone)') IS NOT NULL`, function).
			Scan(context.Background(), &exists))
		assert.Equal(t, want, exists, function)
	}
}
