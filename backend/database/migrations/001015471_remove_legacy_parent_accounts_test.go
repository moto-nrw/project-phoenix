package migrations

import (
	"testing"

	"github.com/uptrace/bun/migrate"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestLegacyParentRemovalPreflightSkipsTablesAwaitingInitialMigration(t *testing.T) {
	t.Parallel()
	parents := legacyParentMigration(t, AuthAccountsParentsVersion)
	links := legacyParentMigration(t, UsersPersonsGuardiansVersion)
	ctx := withPendingMigrations(t.Context(), migrate.MigrationSlice{
		{Name: parents.Name},
		{Name: links.Name},
	})

	// A fresh database has neither source table when preflight sees every
	// migration as pending. Passing nil proves no query is attempted.
	require.NoError(t, legacyParentAccountsPrecondition(ctx, nil))
}

func TestLegacyParentRemovalPreflightChecksExistingSourceTable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	runner := migrate.NewMigrator(db, Migrations)
	parents := legacyParentMigration(t, AuthAccountsParentsVersion)
	links := legacyParentMigration(t, UsersPersonsGuardiansVersion)
	require.NoError(t, parents.Up(ctx, runner, parents))
	_, err := db.ExecContext(ctx, `INSERT INTO auth.accounts_parents (email, username) VALUES ('legacy@example.test', 'legacy')`)
	require.NoError(t, err)

	ctx = withPendingMigrations(ctx, migrate.MigrationSlice{{Name: links.Name}})
	require.ErrorContains(t, legacyParentAccountsPrecondition(ctx, db), "auth.accounts_parents has 1 rows")
}

func TestLegacyParentRemovalRefusesDataAndUnexpectedDependencies(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	currentParent := testpkg.CreateTestAccount(t, db, "current-parent")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "current-guardian")
	_, err := db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = ?, has_account = TRUE WHERE id = ?`, currentParent.ID, guardian.ID)
	require.NoError(t, err)

	runner := migrate.NewMigrator(db, Migrations)
	removal := legacyParentMigration(t, "1.15.471")
	parents := legacyParentMigration(t, "1.0.9")
	linksMigration := legacyParentMigration(t, "1.2.6")
	// Restore the original legacy schema, including its real FK and trigger.
	require.NoError(t, parents.Up(ctx, runner, parents))
	require.NoError(t, linksMigration.Up(ctx, runner, linksMigration))
	var legacyID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.accounts_parents (email, username) VALUES ('legacy@example.test', 'legacy') RETURNING id`).Scan(ctx, &legacyID))
	require.ErrorContains(t, MigrationRegistry["1.15.471"].Precondition(ctx, db), "auth.accounts_parents has 1 rows")
	require.ErrorContains(t, removal.Up(ctx, runner, removal), "auth.accounts_parents has 1 rows")
	person := testpkg.CreateTestPerson(t, db, "Legacy", "Link")
	_, err = db.ExecContext(ctx, `INSERT INTO users.persons_guardians (person_id, guardian_account_id, relationship_type) VALUES (?, ?, 'parent')`, person.ID, legacyID)
	require.NoError(t, err)
	require.ErrorContains(t, MigrationRegistry["1.15.471"].Precondition(ctx, db), "users.persons_guardians has 1 rows")
	require.ErrorContains(t, removal.Up(ctx, runner, removal), "users.persons_guardians has 1 rows")
	var links int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM users.persons_guardians`).Scan(ctx, &links))
	require.Equal(t, 1, links, "refusal preserves the legacy data")
	_, err = db.ExecContext(ctx, `DELETE FROM auth.accounts_parents WHERE id = ?`, legacyID)
	require.NoError(t, err)
	require.NoError(t, MigrationRegistry["1.15.471"].Precondition(ctx, db))
	_, err = db.ExecContext(ctx, `CREATE VIEW auth.legacy_parent_dependency AS SELECT id FROM auth.accounts_parents`)
	require.NoError(t, err)
	require.Error(t, removal.Up(ctx, runner, removal), "unexpected dependencies must not be cascaded away")
	var tableExists bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.persons_guardians') IS NOT NULL`).Scan(ctx, &tableExists))
	require.True(t, tableExists, "all DDL rolls back when a later drop fails")
	_, err = db.ExecContext(ctx, `DROP VIEW auth.legacy_parent_dependency`)
	require.NoError(t, err)
	require.NoError(t, removal.Up(ctx, runner, removal))
	for _, table := range []string{"auth.accounts_parents", "users.persons_guardians"} {
		require.NoError(t, db.NewRaw(`SELECT to_regclass(?) IS NOT NULL`, table).Scan(ctx, &tableExists))
		require.False(t, tableExists, table)
	}
	var parentID int64
	require.NoError(t, db.NewRaw(`SELECT account_id FROM users.guardian_profiles WHERE id = ?`, guardian.ID).Scan(ctx, &parentID))
	require.Equal(t, currentParent.ID, parentID, "current parent accounts and their profile binding survive")
	require.ErrorContains(t, removal.Down(ctx, runner, removal), "restore the release database backup")
}

// Resolve the same registrations the public migration runner executes.
func legacyParentMigration(t *testing.T, version string) *migrate.Migration {
	t.Helper()
	for _, migration := range Migrations.Sorted() {
		metadata, ok := MigrationByBunName(migration.Name)
		if ok && metadata.Version == version {
			return &migration
		}
	}
	t.Fatalf("migration %s is not registered", version)
	return nil
}
