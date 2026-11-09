package migrations

import (
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// 1.15.833 adds to the retirement of 1.15.734 only the end of the school's
// sessions before it is hidden, and one log line per retired school
// (#6305). Without these additions the function is that of 1.15.734, so
// every lock rule of #5540 stays as TestDemoVisitorErasureNeverWaitsForALock
// pins it.
func TestDemoRetirementRevokesSessionsAddsOnlyTheRevocation(t *testing.T) {
	t.Parallel()
	of734 := demoSchoolRetirementWithoutWaitSQL[strings.Index(demoSchoolRetirementWithoutWaitSQL,
		"CREATE OR REPLACE FUNCTION platform.retire_expired_demo_schools"):]
	assert.Equal(t, of734, strings.TrimLeft(demoRetirementOf734SQL, "\n\t"), "the down restores the retirement of 1.15.734")
	added := []string{
		"\t\t\thidden_school BIGINT;\n\t\t\tended INTEGER;\n",
		"\t\t\t\t\t-- The school's sessions end with it, in this subtransaction\n\t\t\t\t\t-- (#6305): every request checks the family an access token\n\t\t\t\t\t-- names, not the school. They end first, in the lock order of\n\t\t\t\t\t-- a sign-in; a session lock in use defers the order, its\n\t\t\t\t\t-- hiding included.\n\t\t\t\t\tSELECT school.id INTO hidden_school\n\t\t\t\t\t\tFROM platform.demo_school_states AS state\n\t\t\t\t\t\tJOIN platform.schools AS school ON school.id = state.tenant_id\n\t\t\t\t\t\tWHERE state.name = target_name\n\t\t\t\t\t\t\tAND (state.status = 'ready' OR (state.status = 'preparing' AND state.claimed_at IS NULL))\n\t\t\t\t\t\t\tAND school.deleted_at IS NULL;\n\t\t\t\t\tended := 0;\n\t\t\t\t\tIF hidden_school IS NOT NULL THEN\n\t\t\t\t\t\tSELECT count(*) INTO ended FROM auth.revoke_school_sessions(hidden_school);\n\t\t\t\t\tEND IF;\n",
		"\t\t\t\t\tIF changed > 0 THEN\n\t\t\t\t\t\tRAISE LOG 'demo school % of order % retired: % sessions ended', hidden_school, target_name, ended;\n\t\t\t\t\tEND IF;\n",
	}
	without := strings.TrimLeft(demoRetirementRevokesSessionsSQL, "\n\t")
	for _, addition := range added {
		require.Equal(t, 1, strings.Count(without, addition), addition)
		without = strings.Replace(without, addition, "", 1)
	}
	assert.Equal(t, of734, without)
	assert.Less(t, strings.Index(demoRetirementRevokesSessionsSQL, "auth.revoke_school_sessions(hidden_school)"),
		strings.Index(demoRetirementRevokesSessionsSQL, "UPDATE platform.schools AS school SET deleted_at = NOW()"),
		"the sessions end before the school is hidden: account before school")
}

// The revocation belongs to Identity & Access. Only the administrative
// transaction's role may call it (RevokeTokensByTenantID); the demo role
// reaches it only through the retirement, which runs as its owner, and gets
// no grant on auth.tokens for it.
func TestSchoolSessionRevocationIsTheAdministrativeRolesAlone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	for role, allowed := range map[string]bool{
		"phoenix_admin": true, "phoenix_demo": false, "phoenix_tenant": false, "phoenix_auth": false,
	} {
		var canCall bool
		require.NoError(t, db.NewRaw(`SELECT has_function_privilege(?, 'auth.revoke_school_sessions(bigint)', 'EXECUTE')`, role).Scan(ctx, &canCall))
		assert.Equal(t, allowed, canCall, role)
	}
	var canWrite bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_demo', 'auth.tokens', 'DELETE')
		OR has_table_privilege('phoenix_demo', 'auth.tokens', 'UPDATE')`).Scan(ctx, &canWrite))
	assert.False(t, canWrite, "phoenix_demo gets no write grant on auth.tokens")
}

func schoolSessionCount(t *testing.T, db bun.IDB, schoolID int64) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE tenant_id = ?`, schoolID).Scan(t.Context(), &count))
	return count
}

// The demo schools hidden before 1.15.833 lose their sessions with it. A
// school without a recorded demo origin, or one still open, keeps them.
func TestDemoRetiredSchoolSessionsBackfill(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	retiredDemo, _ := testpkg.CreateTestTenant(t, db)
	openDemo, _ := testpkg.CreateTestTenant(t, db)
	deletedSchool, _ := testpkg.CreateTestTenant(t, db)
	for slug, schoolID := range map[string]int64{"ogs-alt-k3m9xp": retiredDemo, "ogs-offen-a1b2c3": openDemo} {
		_, err := db.NewRaw(`INSERT INTO platform.demo_school_states (name, tenant_id, seed_state, status)
			VALUES (?, ?, '{}'::jsonb, 'ready')`, slug, schoolID).Exec(ctx)
		require.NoError(t, err)
	}
	for _, schoolID := range []int64{retiredDemo, openDemo, deletedSchool} {
		testpkg.CreateTestSessionFamily(t, db, testpkg.CreateTestAccount(t, db, "demo-backfill").ID, schoolID, "")
	}
	_, err := db.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id IN (?, ?)`, retiredDemo, deletedSchool).Exec(ctx)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, demoRetiredSchoolSessionsBackfillSQL)
	require.NoError(t, err)

	assert.Zero(t, schoolSessionCount(t, db, retiredDemo), "a demo school hidden before loses its sessions")
	assert.Equal(t, 1, schoolSessionCount(t, db, openDemo), "an open demo school keeps them")
	assert.Equal(t, 1, schoolSessionCount(t, db, deletedSchool), "a deleted school without demo origin is not touched")
}

// Down restores the retirement of 1.15.734 and drops the revocation, also a
// second time; Up then installs both again.
func TestDemoRetirementRevokesSessionsRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	revocationExists := func() bool {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT to_regprocedure('auth.revoke_school_sessions(bigint)') IS NOT NULL`).Scan(ctx, &exists))
		return exists
	}
	retirementRevokes := func() bool {
		var definition string
		require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.retire_expired_demo_schools(text[])'::regprocedure)`).Scan(ctx, &definition))
		return strings.Contains(definition, "auth.revoke_school_sessions")
	}
	require.True(t, revocationExists())
	require.True(t, retirementRevokes())

	for range 2 {
		require.NoError(t, demoRetirementRevokesSessionsDown(ctx, db))
		assert.False(t, revocationExists())
		assert.False(t, retirementRevokes())
	}
	require.NoError(t, demoRetirementRevokesSessionsUp(ctx, db))
	assert.True(t, revocationExists())
	assert.True(t, retirementRevokes())
}
