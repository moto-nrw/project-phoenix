package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Only the demo process erases demo visitors (#4048). The function writes
// tables of other owners as their owner, so no serving role may call it,
// and the demo role reaches it without any write grant on those tables.
func TestDemoVisitorErasureIsTheDemoRolesAlone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()

	for role, allowed := range map[string]bool{
		"phoenix_demo": true, "phoenix_admin": false, "phoenix_tenant": false, "phoenix_auth": false,
	} {
		var canCall bool
		require.NoError(t, db.NewRaw(`SELECT has_function_privilege(?, 'platform.erase_closed_demo_visitors()', 'EXECUTE')`, role).Scan(ctx, &canCall))
		assert.Equal(t, allowed, canCall, role)
	}
	for _, table := range []string{"users.persons", "users.guardian_profiles", "enrollment.requests", "audit.guardian_changes",
		"users.parent_messages", "users.parent_announcement_declaration_submissions", "platform.email_outbox"} {
		var canWrite bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_demo', ?, 'UPDATE') OR has_table_privilege('phoenix_demo', ?, 'DELETE')`,
			table, table).Scan(ctx, &canWrite))
		assert.False(t, canWrite, "phoenix_demo gets no write grant on %s", table)
	}

	// The demo role runs it; the rollback keeps the shared database as it was.
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_demo`)
	require.NoError(t, err)
	var erased int
	require.NoError(t, tx.NewRaw(`SELECT platform.erase_closed_demo_visitors()`).Scan(ctx, &erased))
}
