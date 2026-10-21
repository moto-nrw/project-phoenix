package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestLegacyParentRemovalRefusesDataAndUnexpectedDependencies(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	currentParent := testpkg.CreateTestAccount(t, db, "current-parent")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "current-guardian")
	_, err := db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = ?, has_account = TRUE WHERE id = ?`, currentParent.ID, guardian.ID)
	require.NoError(t, err)

	// Restore the original legacy schema, including its real FK and trigger.
	require.NoError(t, createAuthAccountsParentsTable(ctx, db))
	require.NoError(t, usersPersonsGuardiansUp(ctx, db))
	var legacyID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.accounts_parents (email, username) VALUES ('legacy@example.test', 'legacy') RETURNING id`).Scan(ctx, &legacyID))
	require.ErrorContains(t, legacyParentAccountsPrecondition(ctx, db), "auth.accounts_parents has 1 rows")
	require.ErrorContains(t, removeLegacyParentAccountsUp(ctx, db), "auth.accounts_parents has 1 rows")
	person := testpkg.CreateTestPerson(t, db, "Legacy", "Link")
	_, err = db.ExecContext(ctx, `INSERT INTO users.persons_guardians (person_id, guardian_account_id, relationship_type) VALUES (?, ?, 'parent')`, person.ID, legacyID)
	require.NoError(t, err)
	require.ErrorContains(t, legacyParentAccountsPrecondition(ctx, db), "users.persons_guardians has 1 rows")
	require.ErrorContains(t, removeLegacyParentAccountsUp(ctx, db), "users.persons_guardians has 1 rows")
	var links int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM users.persons_guardians`).Scan(ctx, &links))
	require.Equal(t, 1, links, "refusal preserves the legacy data")
	_, err = db.ExecContext(ctx, `DELETE FROM auth.accounts_parents WHERE id = ?`, legacyID)
	require.NoError(t, err)
	require.NoError(t, legacyParentAccountsPrecondition(ctx, db))
	_, err = db.ExecContext(ctx, `CREATE VIEW auth.legacy_parent_dependency AS SELECT id FROM auth.accounts_parents`)
	require.NoError(t, err)
	require.Error(t, removeLegacyParentAccountsUp(ctx, db), "unexpected dependencies must not be cascaded away")
	var tableExists bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.persons_guardians') IS NOT NULL`).Scan(ctx, &tableExists))
	require.True(t, tableExists, "all DDL rolls back when a later drop fails")
	_, err = db.ExecContext(ctx, `DROP VIEW auth.legacy_parent_dependency`)
	require.NoError(t, err)
	require.NoError(t, removeLegacyParentAccountsUp(ctx, db))
	for _, table := range []string{"auth.accounts_parents", "users.persons_guardians"} {
		require.NoError(t, db.NewRaw(`SELECT to_regclass(?) IS NOT NULL`, table).Scan(ctx, &tableExists))
		require.False(t, tableExists, table)
	}
	var parentID int64
	require.NoError(t, db.NewRaw(`SELECT account_id FROM users.guardian_profiles WHERE id = ?`, guardian.ID).Scan(ctx, &parentID))
	require.Equal(t, currentParent.ID, parentID, "current parent accounts and their profile binding survive")
	require.ErrorContains(t, removeLegacyParentAccountsDown(ctx, db), "restore the release database backup")
}
