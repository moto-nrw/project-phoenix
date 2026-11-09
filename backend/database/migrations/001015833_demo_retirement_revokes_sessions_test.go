package migrations

import (
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// 1.15.833 changes only the hiding step of the retirement of 1.15.734: the
// hidden school's id is kept and its sessions end in the same unit (#6305).
// Every lock rule of #5540 stays as TestDemoVisitorErasureNeverWaitsForALock
// pins it.
func TestDemoRetirementRevokesSessionsChangesOnlyTheHiding(t *testing.T) {
	t.Parallel()
	declared := "\t\t\ttried INTEGER := 0;\n"
	hiding := "\t\t\t\t\t\t\tAND school.id = state.tenant_id AND school.deleted_at IS NULL;\n\t\t\t\t\tGET DIAGNOSTICS changed = ROW_COUNT;\n"
	of734 := demoSchoolRetirementWithoutWaitSQL[strings.Index(demoSchoolRetirementWithoutWaitSQL,
		"CREATE OR REPLACE FUNCTION platform.retire_expired_demo_schools"):]
	assert.Equal(t, of734, strings.TrimLeft(demoRetirementOf734SQL, "\n\t"), "the down restores the retirement of 1.15.734")
	require.Equal(t, 1, strings.Count(of734, declared))
	require.Equal(t, 1, strings.Count(of734, hiding))
	expected := strings.Replace(of734, declared, declared+"\t\t\thidden_school BIGINT;\n", 1)
	expected = strings.Replace(expected, hiding, strings.TrimSuffix(hiding, ";\n\t\t\t\t\tGET DIAGNOSTICS changed = ROW_COUNT;\n")+`
						RETURNING school.id INTO hidden_school;
					GET DIAGNOSTICS changed = ROW_COUNT;
					-- The school's sessions end with it, in this subtransaction
					-- (#6305): every request checks the family an access token
					-- names, not the school. A session lock in use defers the
					-- order, its hiding included.
					IF changed > 0 THEN
						PERFORM auth.revoke_school_sessions(hidden_school);
					END IF;
`, 1)
	assert.Equal(t, expected, strings.TrimLeft(demoRetirementRevokesSessionsSQL, "\n\t"))
}

// The revocation belongs to Identity & Access and no role may call it: the
// demo role reaches it only through the retirement, which runs as its owner,
// and gets no grant on auth.tokens for it.
func TestSchoolSessionRevocationIsNoRolesToCall(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	for _, role := range []string{"phoenix_demo", "phoenix_admin", "phoenix_tenant", "phoenix_auth"} {
		var canCall bool
		require.NoError(t, db.NewRaw(`SELECT has_function_privilege(?, 'auth.revoke_school_sessions(bigint)', 'EXECUTE')`, role).Scan(ctx, &canCall))
		assert.False(t, canCall, role)
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
