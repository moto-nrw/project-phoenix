package migrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestTraegerMemberNameGoesWithTheErasedAccount pins the lifecycle of the
// name of #4693: the hourly erasure of an account that no Träger keeps any
// longer (a removed person) deletes its memberships and with them the name;
// a person still active at the Träger keeps both.
func TestTraegerMemberNameGoesWithTheErasedAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	// Remove unused template accounts before arranging the cases.
	eraseUnusedAccountsAsAdmin(t, db)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Name Löschung", Slug: fmt.Sprintf("traeger-name-loeschung-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	// The isolated database goes with the test, the erased account with it.
	account := func(name string) int64 {
		id := testpkg.CreateTestAccount(t, db, name).ID
		testpkg.UnclaimTestAccount(t, db, id)
		return id
	}
	removed, active := account("traeger-name-entfernt"), account("traeger-name-aktiv")
	_, err := db.ExecContext(ctx, `INSERT INTO platform.organization_accounts (organization_id, account_id, status, activated_at, deactivated_at, display_name)
		VALUES (?, ?, 'inactive', NOW(), NOW(), 'Ehemalige Kollegin')`, organization, removed)
	require.NoError(t, err)
	testpkg.CreateTestTraegerMember(t, db, organization, active)
	_, err = db.ExecContext(ctx, `UPDATE platform.organization_accounts SET display_name = 'Aktive Kollegin' WHERE organization_id = ? AND account_id = ?`, organization, active)
	require.NoError(t, err)

	erased := eraseUnusedAccountsAsAdmin(t, db)
	assert.Contains(t, erased, removed, "a removed person no Träger keeps is erased")
	assert.NotContains(t, erased, active, "an active member is kept")
	var names []string
	require.NoError(t, db.NewRaw(`SELECT display_name FROM platform.organization_accounts WHERE organization_id = ? ORDER BY account_id`, organization).Scan(ctx, &names))
	assert.Equal(t, []string{"Aktive Kollegin"}, names, "the erased account's name went with its membership")
}

// TestTraegerMemberNameRollsBack pins the rollback of #4693: Down forgets the
// names and the name changes of the ledger, Up brings the column, the action
// and the read of the Träger transaction back.
func TestTraegerMemberNameRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Name Rückbau", Slug: fmt.Sprintf("traeger-name-rueckbau-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	member := testpkg.CreateTestAccount(t, db, "traeger-name-rueckbau").ID
	testpkg.CreateTestTraegerMember(t, db, organization, member)
	_, err := db.ExecContext(ctx, `INSERT INTO audit.traeger_account_changes (organization_id, actor_account_id, action, target_account_id, target_name)
		VALUES (?, ?, 'name_changed', ?, 'Neuer Name')`, organization, member, member)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM audit.traeger_account_changes WHERE organization_id = ?`, organization)
		require.NoError(t, err)
	})
	column := func(table, name string) bool {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_schema || '.' || table_name = ? AND column_name = ?)`, table, name).Scan(ctx, &exists))
		return exists
	}

	require.NoError(t, traegerMemberDisplayNameDown(ctx, db))
	assert.False(t, column("platform.organization_accounts", "display_name"))
	assert.False(t, column("audit.traeger_account_changes", "target_name"))
	var changes int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM audit.traeger_account_changes WHERE organization_id = ?`, organization).Scan(ctx, &changes))
	assert.Zero(t, changes, "the name changes leave the ledger with the action")

	require.NoError(t, traegerMemberDisplayNameUp(ctx, db))
	assert.True(t, column("platform.organization_accounts", "display_name"))
	_, err = db.ExecContext(ctx, `INSERT INTO audit.traeger_account_changes (organization_id, actor_account_id, action, target_account_id, target_name)
		VALUES (?, ?, 'name_changed', ?, 'Neuer Name')`, organization, member, member)
	require.NoError(t, err, "Up accepts the action again")
	var granted bool
	require.NoError(t, db.NewRaw(`SELECT has_function_privilege('phoenix_tenant', 'platform.traeger_member_names(bigint, bigint[])', 'EXECUTE')
		AND NOT has_function_privilege('phoenix_admin', 'platform.traeger_member_names(bigint, bigint[])', 'EXECUTE')`).Scan(ctx, &granted))
	assert.True(t, granted, "only the request role of the Träger transaction calls the read")
}
