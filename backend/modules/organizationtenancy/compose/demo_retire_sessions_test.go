package compose

import (
	"database/sql"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The retirement ends the sessions of the school it hides, in the unit that
// hides it (#6305): every request checks the family its access token names,
// not the school, so a session the run leaves behind keeps the hidden school
// open. A session lock in use defers the whole unit, school and sessions,
// like every other lock of the run (#5540).

// demoSession writes a session of the account at the school, as a login
// leaves it, and returns its family.
func demoSession(t *testing.T, db *bun.DB, accountID, schoolID int64) string {
	t.Helper()
	var family string
	require.NoError(t, db.NewRaw(`INSERT INTO auth.tokens (account_id, token, expiry, tenant_id, portal_scope, family_id)
		VALUES (?0, encode(sha256(convert_to(gen_random_uuid()::text, 'UTF8')), 'hex'), NOW() + INTERVAL '1 day', ?1, 'tenant', gen_random_uuid()::text)
		RETURNING family_id`, accountID, schoolID).Scan(t.Context(), &family))
	return family
}

// schoolSessions lists the sessions of the school, as JSON for comparison.
func schoolSessions(t *testing.T, db *bun.DB, schoolID int64) string {
	t.Helper()
	var rows string
	require.NoError(t, db.NewRaw(`SELECT COALESCE(jsonb_agg(to_jsonb(token) ORDER BY token.id), '[]'::jsonb)::text
		FROM auth.tokens AS token WHERE token.tenant_id = ?`, schoolID).Scan(t.Context(), &rows))
	return rows
}

// colleague is an account the visitor added to the demo school: no visitor
// account, so the visitor erasure leaves it alone. It also works at the
// order's real school.
func (o demoOrder) colleague(t *testing.T) int64 {
	t.Helper()
	account := testpkg.CreateTestAccount(t, o.db, "demo-kollegin")
	testpkg.MapAccountToTenant(t, o.db, account.ID, o.schoolID)
	testpkg.MapAccountToTenant(t, o.db, account.ID, o.realID)
	demoSession(t, o.db, account.ID, o.schoolID)
	demoSession(t, o.db, account.ID, o.realID)
	return account.ID
}

// The sessions go with the school, not with the visitor erasure: a locked
// state row defers the erasure of the visitor, yet no session of the hidden
// school survives the run, whoever holds it. A session elsewhere stays.
func TestDemoRetirementEndsTheSessionsOfTheSchoolItHides(t *testing.T) {
	t.Parallel()
	order := newDemoOrder(t)
	colleague := order.colleague(t)
	ctx := t.Context()
	holder, err := order.db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	require.NoError(t, stateRow(order.slug)(ctx, holder))

	hidden, erased := retireNow(t, order.queue, nil)

	require.Equal(t, 1, hidden)
	assert.Zero(t, erased, "the visitor erasure waits for the state row")
	assert.Equal(t, "[]", schoolSessions(t, order.db, order.schoolID), "no session of the hidden school is left")
	var elsewhere int
	require.NoError(t, order.db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE account_id = ? AND tenant_id = ?`, colleague, order.realID).Scan(ctx, &elsewhere))
	assert.Equal(t, 1, elsewhere, "the colleague's session at the real school stays")
}

// A refresh in progress holds the account and its session. The run neither
// waits for it nor hides the school without its sessions: it defers the
// whole order, counted once and logged once, and the next run, told
// nothing, finds the order in the database and retires it.
func TestDemoRetirementDefersASchoolWhoseSessionIsInUse(t *testing.T) {
	t.Parallel()
	order := newDemoOrder(t)
	colleague := order.colleague(t)
	ctx := t.Context()
	before := schoolSessions(t, order.db, order.schoolID)
	holder, err := order.db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	require.NoError(t, statement(`SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, colleague)(ctx, holder))
	require.NoError(t, statement(`SELECT id FROM auth.tokens WHERE account_id = ? AND tenant_id = ? FOR UPDATE`, colleague, order.schoolID)(ctx, holder))

	hidden, erased := retireNow(t, order.queue, []string{order.slug})

	assert.Zero(t, hidden, "the school waits for the next run")
	assert.Zero(t, erased)
	assert.False(t, schoolHidden(t, order.db, order.schoolID), "no school is hidden without its sessions")
	assert.Equal(t, before, schoolSessions(t, order.db, order.schoolID), "a deferred order keeps every session")
	var deferrals int
	require.NoError(t, order.db.NewRaw(`SELECT erasure_deferrals FROM platform.demo_school_states WHERE name = ?`, order.slug).Scan(ctx, &deferrals))
	assert.Equal(t, 1, deferrals, "one deferral, one log line")

	require.NoError(t, holder.Rollback())
	hidden, _ = retireNow(t, order.queue, nil)
	assert.Equal(t, 1, hidden, "the next run finds the order without being told")
	assert.Equal(t, "[]", schoolSessions(t, order.db, order.schoolID))
}

// A refresh that commits its successor while the run waits for the account
// lock loses the successor too: the run takes the lock refresh takes first
// and deletes in a statement after it, so it sees what the refresh wrote.
// The run waits at most lock_timeout, so the test repeats until the refresh
// committed within that wait.
func TestDemoRetirementEndsASessionRotatedWhileItWaits(t *testing.T) {
	t.Parallel()
	order := newDemoOrder(t)
	colleague := order.colleague(t)
	ctx := t.Context()
	type outcome struct {
		hidden int
		err    error
	}
	for attempt := 1; attempt <= 30; attempt++ {
		refresh, err := order.db.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		for _, run := range []step{
			statement(`SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, colleague),
			statement(`INSERT INTO auth.tokens (account_id, token, expiry, tenant_id, portal_scope, family_id)
				SELECT account_id, encode(sha256(convert_to(gen_random_uuid()::text, 'UTF8')), 'hex'), expiry, tenant_id, portal_scope, family_id
				FROM auth.tokens WHERE account_id = ? AND tenant_id = ? AND rotated_at IS NULL`, colleague, order.schoolID),
			statement(`UPDATE auth.tokens SET rotated_at = NOW(), replacement_token = encode(sha256(convert_to(gen_random_uuid()::text, 'UTF8')), 'hex') WHERE account_id = ? AND tenant_id = ? AND rotated_at IS NULL
				AND id < (SELECT max(id) FROM auth.tokens WHERE account_id = ? AND tenant_id = ?)`, colleague, order.schoolID, colleague, order.schoolID),
		} {
			require.NoError(t, run(ctx, refresh))
		}
		done := make(chan outcome, 1)
		go func() {
			hidden, _, retireErr := order.queue.RetireDemoSchools(ctx, []string{order.slug})
			done <- outcome{hidden: hidden, err: retireErr}
		}()
		var result outcome
		finished := false
		for !finished && !lockWaits(t, order.db, "retire_expired_demo_schools") {
			select {
			case result = <-done:
				finished = true
			default:
			}
		}
		require.NoError(t, refresh.Commit())
		if !finished {
			result = <-done
		}
		require.NoError(t, result.err)
		if result.hidden == 0 {
			continue // the wait ended before the refresh committed: deferred
		}
		require.False(t, finished, "the run hid the school while the refresh held the account")
		t.Logf("the refresh committed while the run waited, attempt %d", attempt)
		assert.Equal(t, "[]", schoolSessions(t, order.db, order.schoolID), "the successor the refresh committed is gone too")
		return
	}
	t.Fatal("the refresh never committed while the run waited")
}

// The demo process runs the retirement as phoenix_demo, which has no grant
// on auth.tokens: the retirement ends the sessions as its owner.
func TestDemoRoleEndsTheSessionsWithoutAGrant(t *testing.T) {
	t.Parallel()
	order := newDemoOrder(t)
	order.colleague(t)
	ctx := t.Context()
	tx, err := order.db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_demo`)
	require.NoError(t, err)
	var result struct{ Hidden, Erased int }
	require.NoError(t, tx.NewRaw(`SELECT hidden, erased FROM platform.retire_expired_demo_schools(ARRAY[?]::text[])`, order.slug).Scan(ctx, &result))
	require.Equal(t, 1, result.Hidden)
	_, err = tx.ExecContext(ctx, `RESET ROLE`)
	require.NoError(t, err)
	var left int
	require.NoError(t, tx.NewRaw(`SELECT count(*) FROM auth.tokens WHERE tenant_id = ?`, order.schoolID).Scan(ctx, &left))
	assert.Zero(t, left, "no session of the hidden school is left")
}
