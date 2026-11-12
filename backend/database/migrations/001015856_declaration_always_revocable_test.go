package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func parentAnnouncementColumnExists(t *testing.T, ctx context.Context, db *bun.DB, column string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'users' AND table_name = 'parent_announcements' AND column_name = ?)`, column).Scan(ctx, &exists))
	return exists
}

// TestDeclarationAlwaysRevocableDropsTheSetting runs the migration of #6303:
// "Widerruf erlauben" leaves the table, the shape constraint still guards the
// declaration settings, and the down path restores every declaration as
// revocable.
func TestDeclarationAlwaysRevocableDropsTheSetting(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.False(t, parentAnnouncementColumnExists(t, ctx, db, "declaration_revocable"), "the migrated schema has no revocable setting")

	require.NoError(t, declarationAlwaysRevocableDown(ctx, db))
	require.True(t, parentAnnouncementColumnExists(t, ctx, db, "declaration_revocable"))
	require.NoError(t, declarationAlwaysRevocableUp(ctx, db))
	assert.False(t, parentAnnouncementColumnExists(t, ctx, db, "declaration_revocable"))
	require.NoError(t, declarationAlwaysRevocableUp(ctx, db), "a second Up changes nothing")

	var clause string
	require.NoError(t, db.NewRaw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conname = 'chk_parent_announcements_declaration_shape'`).Scan(ctx, &clause))
	assert.Contains(t, clause, "declaration_signers", "the shape constraint still guards the settings")
	assert.NotContains(t, clause, "declaration_revocable")
}
