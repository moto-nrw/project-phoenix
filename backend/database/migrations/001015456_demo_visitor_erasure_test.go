package migrations

import (
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemoVisitorErasureBackfillsAbandonedReplacementSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	currentID, _ := testpkg.CreateTestTenant(t, db)
	const slug = "ogs-legacy-abc123"

	var abandonedID int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.schools (organization_id, name, slug, subdomain, deleted_at)
		SELECT organization_id, 'Aufgegebene Demo', 'aufgegeben', 'aufgegeben', NOW()
		FROM platform.schools WHERE id = ? RETURNING id`, currentID).Scan(ctx, &abandonedID))
	retiredSubdomain := fmt.Sprintf("x%d-%s", abandonedID, slug)
	_, err := db.NewRaw(`UPDATE platform.schools SET slug = ?, subdomain = ? WHERE id = ?`,
		retiredSubdomain, retiredSubdomain, abandonedID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO platform.demo_school_states (name, tenant_id, seed_state, status)
		VALUES (?, ?, '{}'::jsonb, 'ready')`, slug, currentID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, demoVisitorErasureUp(ctx, db))
	var recovered bool
	require.NoError(t, db.NewRaw(`SELECT ? = ANY(visitor_school_ids) FROM platform.demo_school_states WHERE name = ?`,
		abandonedID, slug).Scan(ctx, &recovered))
	assert.True(t, recovered, "the pre-migration abandoned school is retained for visitor erasure")
}

func TestDemoVisitorErasureBackfillsTheSchoolsOfALegacyFailedOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	currentID, _ := testpkg.CreateTestTenant(t, db)
	const slug = "ogs-legacy-failed-abc123"

	_, err := db.NewRaw(`UPDATE platform.schools SET slug = ?, subdomain = ?, deleted_at = NOW() WHERE id = ?`, slug, slug, currentID).Exec(ctx)
	require.NoError(t, err)
	var abandonedID int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.schools (organization_id, name, slug, subdomain, deleted_at)
		SELECT organization_id, 'Aufgegebene Demo', 'aufgegeben', 'aufgegeben', NOW()
		FROM platform.schools WHERE id = ? RETURNING id`, currentID).Scan(ctx, &abandonedID))
	retiredSubdomain := fmt.Sprintf("x%d-%s", abandonedID, slug)
	_, err = db.NewRaw(`UPDATE platform.schools SET slug = ?, subdomain = ? WHERE id = ?`, retiredSubdomain, retiredSubdomain, abandonedID).Exec(ctx)
	require.NoError(t, err)
	// Before 1.15.456, Fail cleared tenant_id and the visitor identifiers.
	_, err = db.NewRaw(`INSERT INTO platform.demo_school_states (name, seed_state, status, attempts)
		VALUES (?, '{}'::jsonb, 'failed', 3)`, slug).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, demoVisitorErasureUp(ctx, db))
	for _, schoolID := range []int64{currentID, abandonedID} {
		var recovered bool
		require.NoError(t, db.NewRaw(`SELECT ? = ANY(visitor_school_ids) FROM platform.demo_school_states WHERE name = ?`,
			schoolID, slug).Scan(ctx, &recovered))
		assert.True(t, recovered, "the legacy failed order keeps school %d for visitor erasure", schoolID)
	}
}

// Only the demo process erases demo visitors (#4048). The function writes
// tables of other owners as their owner, so no serving role may call it,
// and the demo role reaches it without any write grant on those tables.
func TestDemoVisitorErasureIsTheDemoRolesAlone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()

	for _, function := range []string{"platform.erase_closed_demo_visitors()", "platform.retire_expired_demo_schools(text[])"} {
		for role, allowed := range map[string]bool{
			"phoenix_demo": true, "phoenix_admin": false, "phoenix_tenant": false, "phoenix_auth": false,
		} {
			var canCall bool
			require.NoError(t, db.NewRaw(`SELECT has_function_privilege(?, ?, 'EXECUTE')`, role, function).Scan(ctx, &canCall))
			assert.Equal(t, allowed, canCall, function+" / "+role)
		}
	}
	for _, table := range []string{"auth.guardian_student_access", "users.persons", "users.guardian_profiles", "enrollment.requests", "audit.guardian_changes",
		"users.parent_messages", "users.parent_announcement_declaration_submissions", "platform.email_delivery", "platform.email_outbox"} {
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
	var retired struct{ Hidden, Erased int }
	require.NoError(t, tx.NewRaw(`SELECT hidden, erased FROM platform.retire_expired_demo_schools('{}'::text[])`).Scan(ctx, &retired))
}
