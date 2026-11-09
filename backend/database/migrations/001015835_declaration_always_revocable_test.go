package migrations

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"

	testpkg "github.com/moto-nrw/project-phoenix/test"
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

// TestDeclarationAlwaysRevocableRunsAfterHigherVersions proves that the runner
// applies this migration on a database that already holds later versions:
// 1.15.835 was added after development had 1.15.836 and 1.15.837, so staging
// and production run it out of number order. The runner applies every
// registered migration not marked applied (bun's Unapplied), whatever the
// highest applied version is.
func TestDeclarationAlwaysRevocableRunsAfterHigherVersions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	migrator := newMigrator(db, nil)

	status, err := migrator.MigrationsWithStatus(ctx)
	require.NoError(t, err)
	target, laterApplied := declarationAlwaysRevocableStatus(status)
	require.NotNil(t, target)
	require.True(t, target.IsApplied())
	require.NotEmpty(t, laterApplied, "the database already holds versions above 1.15.835")

	// The state of a database migrated before this migration existed.
	require.NoError(t, declarationAlwaysRevocableDown(ctx, db))
	require.NoError(t, migrator.MarkUnapplied(ctx, target))
	require.True(t, parentAnnouncementColumnExists(t, ctx, db, "declaration_revocable"))

	require.NoError(t, migrateWithLogger(ctx, db, slog.New(slog.NewTextHandler(io.Discard, nil))))

	assert.False(t, parentAnnouncementColumnExists(t, ctx, db, "declaration_revocable"), "the lower version ran")
	after, err := migrator.MigrationsWithStatus(ctx)
	require.NoError(t, err)
	applied, stillLater := declarationAlwaysRevocableStatus(after)
	require.NotNil(t, applied)
	assert.True(t, applied.IsApplied(), "the lower version is recorded as applied")
	assert.Equal(t, laterApplied, stillLater, "the later versions stay applied and are not run again")
	assert.Empty(t, after.Unapplied())
}

// declarationAlwaysRevocableStatus returns this migration's status and the
// names of the applied migrations with a higher version.
func declarationAlwaysRevocableStatus(status migrate.MigrationSlice) (*migrate.Migration, []string) {
	var target *migrate.Migration
	later := []string{}
	for i := range status {
		version, _ := migrationIdentity(&status[i])
		switch {
		case version == declarationAlwaysRevocableVersion:
			target = &status[i]
		case compareVersionsSemantic(declarationAlwaysRevocableVersion, version) && status[i].IsApplied():
			later = append(later, status[i].Name)
		}
	}
	return target, later
}
