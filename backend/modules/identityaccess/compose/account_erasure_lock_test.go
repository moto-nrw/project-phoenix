package compose_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The hourly erasure of unused accounts never waits for a lock for long
// (#5546). Writers lock an account's rows and the account in both orders:
// a session renewal locks the session row and then, inserting its
// successor, the account; a login locks the membership and then the
// account. The erasure used to lock every candidate account first and then
// delete its rows, so it waited for such a writer while holding the account
// the writer needed next. These tests hold each kind of lock from another
// transaction, in both orders against the erasure, and compare the deferred
// account with a snapshot.

// erasedQuickly is the bound a run that meets a lock in use returns within:
// deadlock_timeout (1 s). Each lock wait of the run ends after lock_timeout
// (100 ms); the rest is the work of the run.
const erasedQuickly = time.Second

type erasureFixture struct {
	db     *bun.DB
	owner  identityaccess.Owner
	base   context.Context
	school int64
}

func newErasureFixture(t *testing.T) erasureFixture {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)
	owner, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	f := erasureFixture{db: db, owner: owner, base: testpkg.WithTenantRuntime(t, t.Context(), db), school: school}
	// The clone may hold unused accounts of its own; the first run takes
	// them, so the counts below are the test's accounts only.
	f.eraseNow(t)
	return f
}

// unusedAccount is an account no school needs any more that still has every
// kind of row the erasure deletes and the writers below lock: an active
// membership with only the guardian role (a parent whose contact the school
// deleted), a live session, a profile and an avatar.
func (f erasureFixture) unusedAccount(t *testing.T, label string) int64 {
	t.Helper()
	ctx := t.Context()
	account := testpkg.CreateTestAccount(t, f.db, label)
	testpkg.UnclaimTestAccount(t, f.db, account.ID)
	for _, statement := range []string{
		`UPDATE auth.accounts SET avatar = '/uploads/avatars/global/lock-' || ?0 || '.png' WHERE id = ?0`,
		`INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at) VALUES (?0, ?1, 'active', NOW())`,
		`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) SELECT ?0, id, ?1 FROM auth.roles WHERE name = 'guardian'`,
		`INSERT INTO auth.tokens (account_id, token, expiry, tenant_id, portal_scope, family_id)
			VALUES (?0, encode(sha256(convert_to('lock-' || ?0::text, 'UTF8')), 'hex'), NOW() + INTERVAL '1 day', ?1, 'parent', 'family-' || ?0)`,
		`INSERT INTO users.profiles (tenant_id, account_id, bio, avatar, settings)
			VALUES (?1, ?0, 'Ich bin Eltern', '/uploads/avatars/' || ?1 || '/profile-' || ?0 || '.png', '{"theme": "dunkel"}'::jsonb)`,
	} {
		_, err := f.db.NewRaw(statement, account.ID, f.school).Exec(ctx)
		require.NoError(t, err, statement)
	}
	return account.ID
}

// erasureSnapshotRows selects, per table, every row of one account the
// erasure may change or write.
var erasureSnapshotRows = map[string]string{
	"auth.accounts":                    `FROM auth.accounts AS r WHERE r.id = ?`,
	"auth.account_tenants":             `FROM auth.account_tenants AS r WHERE r.account_id = ?`,
	"auth.account_roles":               `FROM auth.account_roles AS r WHERE r.account_id = ?`,
	"auth.tokens":                      `FROM auth.tokens AS r WHERE r.account_id = ?`,
	"users.profiles":                   `FROM users.profiles AS r WHERE r.account_id = ?`,
	"auth.account_avatar_file_cleanup": `FROM auth.account_avatar_file_cleanup AS r WHERE r.avatar LIKE '%-' || ? || '.png'`,
}

func (f erasureFixture) snapshot(t *testing.T, accountID int64) map[string]string {
	t.Helper()
	snapshot := make(map[string]string, len(erasureSnapshotRows))
	for table, rows := range erasureSnapshotRows {
		var content string
		require.NoError(t, f.db.NewRaw(`SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text), '[]'::jsonb)::text `+rows, accountID).
			Scan(t.Context(), &content), table)
		snapshot[table] = content
	}
	return snapshot
}

func (f erasureFixture) erased(t *testing.T, accountID int64) bool {
	t.Helper()
	var erased bool
	require.NoError(t, f.db.NewRaw(`SELECT personal_data_erased_at IS NOT NULL FROM auth.accounts WHERE id = ?`, accountID).Scan(t.Context(), &erased))
	return erased
}

// requireFullyErased checks that nothing of the account is left but its
// tombstone and the durable avatar deletion intents.
func (f erasureFixture) requireFullyErased(t *testing.T, accountID int64) {
	t.Helper()
	require.True(t, f.erased(t, accountID), "account %d is erased", accountID)
	snapshot := f.snapshot(t, accountID)
	for _, table := range []string{"auth.account_tenants", "auth.account_roles", "auth.tokens", "users.profiles"} {
		assert.Equal(t, "[]", snapshot[table], "%s keeps nothing of account %d", table, accountID)
	}
	var queued int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM auth.account_avatar_file_cleanup WHERE avatar LIKE '%-' || ? || '.png'`, accountID).
		Scan(t.Context(), &queued))
	assert.Equal(t, 2, queued, "both avatar files of account %d are queued", accountID)
}

type erasureOutcome struct {
	erased int
	err    error
}

func (f erasureFixture) startErasure() <-chan erasureOutcome {
	done := make(chan erasureOutcome, 1)
	go func() {
		var erased int
		err := testpkg.WithinAdminTransaction(f.base, func(ctx context.Context) error {
			var runErr error
			erased, runErr = f.owner.EraseUnusedAccounts(ctx)
			return runErr
		})
		done <- erasureOutcome{erased: erased, err: err}
	}()
	return done
}

// eraseNow runs the hourly erasure in its administrative transaction and
// fails the test when it does not return quickly.
func (f erasureFixture) eraseNow(t *testing.T) int {
	t.Helper()
	started := time.Now()
	select {
	case result := <-f.startErasure():
		require.NoError(t, result.err, "no lock error leaves the run")
		assert.Less(t, time.Since(started), erasedQuickly, "the run returns quickly")
		return result.erased
	case <-time.After(5 * time.Second):
		t.Fatal("the erasure waited for a lock another transaction holds")
		return 0
	}
}

// erasureWaits reports whether another session runs a statement containing
// marker and waits for a lock.
func erasureWaits(t *testing.T, db *bun.DB, marker string) bool {
	t.Helper()
	var waiting int
	err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
		WHERE datname = current_database() AND pid <> pg_backend_pid()
			AND wait_event_type = 'Lock' AND strpos(query, ?) > 0`, marker).Scan(t.Context(), &waiting)
	return err == nil && waiting > 0
}

type erasureStep func(ctx context.Context, tx bun.Tx) error

func erasureStatement(query string, args ...any) erasureStep {
	return func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewRaw(query, args...).Exec(ctx)
		return err
	}
}

// writerFirst: another transaction holds a row of the account (hold); the run
// returns quickly, leaves the account completely unchanged and erases the
// free account beside it. The writer then finishes (write) and commits.
func (f erasureFixture) writerFirst(t *testing.T, accountID int64, hold []erasureStep, write erasureStep) {
	t.Helper()
	ctx := t.Context()
	free := f.unusedAccount(t, "erasure-free")
	before := f.snapshot(t, accountID)
	writer, err := f.db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = writer.Rollback() }()
	for _, run := range hold {
		require.NoError(t, run(ctx, writer))
	}
	assert.Equal(t, 1, f.eraseNow(t), "only the free account is erased")
	assert.Equal(t, before, f.snapshot(t, accountID), "a deferred account is completely unchanged")
	f.requireFullyErased(t, free)
	require.NoError(t, write(ctx, writer), "the writer does not fail beside the run")
	require.NoError(t, writer.Commit(), "the writer does not fail beside the run")
}

// erasureFirst: the run has locked the account and waits for a row the writer
// holds (hold); meanwhile the writer needs the account (write, a statement
// containing marker). That is a lock cycle, which before #5546 ended in
// deadlock_detected for one side. The run must give up first: neither fails,
// and the account is deferred unchanged. The run waits at most lock_timeout,
// so the test repeats until it saw the writer waiting for the run.
func (f erasureFixture) erasureFirst(t *testing.T, accountID int64, hold []erasureStep, marker string, write erasureStep) {
	t.Helper()
	ctx := t.Context()
	before := f.snapshot(t, accountID)
	for attempt := 1; attempt <= 30; attempt++ {
		writer, err := f.db.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		for _, run := range hold {
			require.NoError(t, run(ctx, writer))
		}
		erasure := f.startErasure()
		var result erasureOutcome
		finished := false
		for !finished && !erasureWaits(t, f.db, "erase_unused_accounts") {
			select {
			case result = <-erasure:
				finished = true
			default:
			}
		}
		writerWaited := false
		var writeErr error
		if !finished {
			written := make(chan error, 1)
			go func() { written <- write(ctx, writer) }()
			wrote := false
			for !finished || !wrote {
				select {
				case result = <-erasure:
					finished = true
				case writeErr = <-written:
					wrote = true
				default:
					if !finished && !wrote && erasureWaits(t, f.db, marker) {
						writerWaited = true
					}
				}
			}
		}
		require.NoError(t, result.err, "the run does not fail beside the writer")
		require.NoError(t, writeErr, "the writer does not fail beside the run")
		assert.Zero(t, result.erased, "the account waits for the next run")
		require.NoError(t, writer.Rollback())
		require.Equal(t, before, f.snapshot(t, accountID), "a deferred account is completely unchanged")
		if writerWaited {
			t.Logf("the writer waited for the run in attempt %d", attempt)
			return
		}
	}
	t.Fatal("the writer never waited for the run; the cycle was not exercised")
}

// requireErasedNext runs the next run once nothing holds a lock any more,
// without naming the account: the run finds deferred accounts in the
// database. It requires that the run erases the account completely.
func (f erasureFixture) requireErasedNext(t *testing.T, accountID int64) {
	t.Helper()
	assert.Equal(t, 1, f.eraseNow(t), "the next run erases the deferred account")
	f.requireFullyErased(t, accountID)
}

// A session renewal marks the session rotated, which locks its row, and then
// inserts the successor, whose foreign key locks the account.
func sessionRotation(accountID int64) erasureStep {
	return erasureStatement(`UPDATE auth.tokens SET rotated_at = NOW(),
		replacement_token = encode(sha256(convert_to('next-' || account_id::text, 'UTF8')), 'hex')
		WHERE account_id = ? AND rotated_at IS NULL`, accountID)
}

func sessionSuccessor(accountID, school int64) erasureStep {
	return erasureStatement(`INSERT INTO auth.tokens (account_id, token, expiry, tenant_id, portal_scope, family_id) /* session successor */
		VALUES (?0, encode(sha256(convert_to('next-' || ?0::text, 'UTF8')), 'hex'), NOW() + INTERVAL '1 day', ?1, 'parent', 'family-' || ?0)`,
		accountID, school)
}

// A login and a session check pin the membership with FOR SHARE
// (LockActiveTenantMappingShared) before they write a session or the
// membership.
func membershipPin(accountID, school int64) erasureStep {
	return erasureStatement(`SELECT 1 FROM auth.account_tenants
		WHERE account_id = ? AND tenant_id = ? AND status = 'active' FOR SHARE`, accountID, school)
}

// EnsureStaffCalendarFeedToken: an UPDATE of the membership locks the row,
// then its trigger auth.lock_live_account_reference the account.
func membershipUpdate(accountID, school int64) erasureStep {
	return erasureStatement(`UPDATE auth.account_tenants SET staff_calendar_feed_token = 'feed-' || account_id /* membership update */
		WHERE account_id = ? AND tenant_id = ?`, accountID, school)
}

func TestUnusedAccountErasureMeetsASessionRenewalWriterFirst(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(t)
	account := f.unusedAccount(t, "erasure-renewal")
	f.writerFirst(t, account, []erasureStep{sessionRotation(account)}, sessionSuccessor(account, f.school))
	f.requireErasedNext(t, account)
}

func TestUnusedAccountErasureMeetsASessionRenewalErasureFirst(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(t)
	account := f.unusedAccount(t, "erasure-renewal")
	f.erasureFirst(t, account, []erasureStep{sessionRotation(account)}, "session successor", sessionSuccessor(account, f.school))
	f.requireErasedNext(t, account)
}

func TestUnusedAccountErasureMeetsALoginWriterFirst(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(t)
	account := f.unusedAccount(t, "erasure-login")
	f.writerFirst(t, account, []erasureStep{membershipPin(account, f.school)}, sessionSuccessor(account, f.school))
	f.requireErasedNext(t, account)
}

func TestUnusedAccountErasureMeetsALoginErasureFirst(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(t)
	account := f.unusedAccount(t, "erasure-login")
	f.erasureFirst(t, account, []erasureStep{membershipPin(account, f.school)}, "session successor", sessionSuccessor(account, f.school))
	f.requireErasedNext(t, account)
}

func TestUnusedAccountErasureMeetsAMembershipUpdateWriterFirst(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(t)
	account := f.unusedAccount(t, "erasure-membership")
	f.writerFirst(t, account, []erasureStep{membershipPin(account, f.school)}, membershipUpdate(account, f.school))
	f.requireErasedNext(t, account)
}

func TestUnusedAccountErasureMeetsAMembershipUpdateErasureFirst(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(t)
	account := f.unusedAccount(t, "erasure-membership")
	f.erasureFirst(t, account, []erasureStep{membershipPin(account, f.school)}, "membership update", membershipUpdate(account, f.school))
	f.requireErasedNext(t, account)
}

// Writers that lock the account first (login and role removal lock it FOR
// UPDATE, upserts and inserts through auth.lock_live_account_reference or a
// foreign key FOR KEY SHARE) make the erasure pass the account over at no
// cost; it erases the free account and the held one in the next run.
func TestUnusedAccountErasurePassesOverAnAccountAWriterHolds(t *testing.T) {
	t.Parallel()
	for name, lock := range map[string]string{
		"account update": `SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`,
		"account share":  `SELECT id FROM auth.accounts WHERE id = ? FOR KEY SHARE`,
		"role upsert": `INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
			SELECT account_id, (SELECT id FROM auth.roles WHERE name = 'guardian'), tenant_id
			FROM auth.account_tenants WHERE account_id = ? ON CONFLICT DO NOTHING`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newErasureFixture(t)
			account := f.unusedAccount(t, "erasure-held")
			f.writerFirst(t, account, []erasureStep{erasureStatement(lock, account)}, erasureStatement(`SELECT 1`))
			f.requireErasedNext(t, account)
		})
	}
}

// A table the run reads or writes held exclusively: the run returns quickly
// without an error and leaves the account as it was, or erases it
// completely. The next run, told nothing, erases it.
func TestUnusedAccountErasureMeetsALockedTable(t *testing.T) {
	t.Parallel()
	for _, table := range []string{"auth.accounts", "auth.account_tenants", "auth.account_roles", "auth.tokens",
		"users.profiles", "auth.account_avatar_file_cleanup", "auth.invitation_tokens"} {
		t.Run(table, func(t *testing.T) {
			f := newErasureFixture(t)
			account := f.unusedAccount(t, "erasure-table")
			before := f.snapshot(t, account)
			ctx := t.Context()
			holder, err := f.db.BeginTx(ctx, &sql.TxOptions{})
			require.NoError(t, err)
			defer func() { _ = holder.Rollback() }()
			_, err = holder.ExecContext(ctx, `LOCK TABLE `+table+` IN ACCESS EXCLUSIVE MODE`)
			require.NoError(t, err)
			assert.Zero(t, f.eraseNow(t), "nothing is erased while %s is locked", table)
			require.NoError(t, holder.Rollback())
			assert.Equal(t, before, f.snapshot(t, account), "a deferred account is completely unchanged")
			f.requireErasedNext(t, account)
		})
	}
}

// Accounts whose rows stay locked do not starve the accounts behind them: a
// deferral costs the run at most one lock_timeout, and an account a writer
// holds costs it nothing, so a free account behind both is erased in the
// first run and the held ones, without being named, once they are free.
func TestUnusedAccountErasureReachesTheAccountsBehindLockedOnes(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(t)
	ctx := t.Context()
	rowLocked := make([]int64, 0, 3)
	for i := range 3 {
		rowLocked = append(rowLocked, f.unusedAccount(t, fmt.Sprintf("erasure-row-%d", i)))
	}
	accountLocked := make([]int64, 0, 60)
	for i := range 60 {
		accountLocked = append(accountLocked, f.unusedAccount(t, fmt.Sprintf("erasure-held-%d", i)))
	}
	free := f.unusedAccount(t, "erasure-behind")
	holder, err := f.db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	for _, account := range rowLocked {
		require.NoError(t, sessionRotation(account)(ctx, holder))
	}
	_, err = holder.NewRaw(`SELECT id FROM auth.accounts WHERE id IN (?) FOR KEY SHARE`, bun.List(accountLocked)).Exec(ctx)
	require.NoError(t, err)

	assert.Equal(t, 1, f.eraseNow(t), "the first run erases the free account")
	f.requireFullyErased(t, free)
	require.NoError(t, holder.Rollback())
	for run := 0; run < 3; run++ {
		f.eraseNow(t)
	}
	for _, account := range append(rowLocked, accountLocked...) {
		assert.True(t, f.erased(t, account), "account %d is erased once it is free", account)
	}
}

// A run erases at most 200 accounts; the next run takes the rest.
func TestUnusedAccountErasureKeepsItsBatch(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(t)
	_, err := f.db.NewRaw(`INSERT INTO auth.accounts (email, active)
		SELECT 'erasure-batch-' || n || '-' || ? || '@example.invalid', TRUE FROM generate_series(1, 205) AS n`, f.school).Exec(t.Context())
	require.NoError(t, err)
	var open int
	count := func() int {
		require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM auth.accounts
			WHERE email LIKE 'erasure-batch-%' AND personal_data_erased_at IS NULL`).Scan(t.Context(), &open))
		return open
	}
	require.Equal(t, 205, count())
	assert.Equal(t, 200, f.eraseNow(t), "one run erases at most 200 accounts")
	assert.Equal(t, 5, count())
	assert.Equal(t, 5, f.eraseNow(t), "the next run takes the rest")
	assert.Zero(t, count())
}
