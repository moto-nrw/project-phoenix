package behavior_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// A login and the hourly erasure of unused accounts meet on the account row
// (#5546). Every account session is minted through one step that stamps the
// last login on the live account under its row lock, in the transaction that
// writes the session: an account the erasure anonymized meanwhile mints
// nothing. These tests stop each side at its lock and let the other one run.

type erasureLogin struct {
	db      *bun.DB
	service testAuthService
	owner   identityaccess.Owner
	base    context.Context
	email   string
	account int64
}

// newErasureLogin is a parent no school needs any more: an active membership
// with only the guardian role, no guardian contact. The parent login accepts
// the account, and the hourly erasure anonymizes it.
func newErasureLogin(t *testing.T) erasureLogin {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	owner := setupAuthFactory(t, db)
	service := newFixtureAuthService(t, db, owner)
	e := erasureLogin{db: db, service: service, owner: owner, base: testpkg.WithTenantRuntime(t, t.Context(), db)}
	// The clone may hold unused accounts of its own; this run takes them,
	// so the counts below are the test's account only.
	e.erase(t)
	// The fixture maps the account to the test's school; the guardian role
	// there is the only one it gets.
	email, _ := uniqueTestCredentials("erasure-login")
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	account := testpkg.CreateTestAccountWithPassword(t, db, email, testPassword)
	assignSeededRole(t, db, account.ID, tenantID, "guardian")
	var unused bool
	require.NoError(t, db.NewRaw(`SELECT auth.account_is_unused(id, email) FROM auth.accounts WHERE id = ?`, account.ID).Scan(t.Context(), &unused))
	require.True(t, unused, "the parent is an account no school needs")
	e.email, e.account = email, account.ID
	return e
}

func (e erasureLogin) erase(t *testing.T) int {
	t.Helper()
	var erased int
	require.NoError(t, testpkg.WithinAdminTransaction(e.base, func(ctx context.Context) error {
		var err error
		erased, err = e.owner.EraseUnusedAccounts(ctx)
		return err
	}))
	return erased
}

type loginResult struct {
	refresh string
	err     error
}

func (e erasureLogin) startLogin() <-chan loginResult {
	done := make(chan loginResult, 1)
	go func() {
		_, refresh, err := e.service.LoginParent(context.Background(), e.email, testPassword)
		done <- loginResult{refresh: refresh, err: err}
	}()
	return done
}

// waitFor waits until another session runs a statement containing marker
// and waits for a lock of the given kind ("" for any).
func (e erasureLogin) waitFor(t *testing.T, marker, event string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		require.NoError(t, e.db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND pid <> pg_backend_pid()
				AND wait_event_type = 'Lock' AND (? = '' OR wait_event = ?) AND strpos(query, ?) > 0`,
			event, event, marker).Scan(t.Context(), &waiting))
		if waiting > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no session waited on a lock running %q", marker)
}

// requireRefusedLikeWrongCredentials accepts the two refusals the login
// route renders alike, as wrong credentials (loginErrorResponse): an unknown
// address and an account gone by the time the session is minted.
func requireRefusedLikeWrongCredentials(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err, "the login refuses an erased account")
	assert.True(t, errors.Is(err, identityaccess.ErrInvalidCredentials) || errors.Is(err, identityaccess.ErrAccountNotFound),
		"the login refuses an erased account like wrong credentials: %v", err)
}

type accountState struct {
	Erased    bool
	LastLogin sql.NullTime
	Sessions  int
}

func (e erasureLogin) state(t *testing.T) accountState {
	t.Helper()
	var state accountState
	require.NoError(t, e.db.NewRaw(`SELECT account.personal_data_erased_at IS NOT NULL, account.last_login,
			(SELECT count(*) FROM auth.tokens AS token WHERE token.account_id = account.id)
		FROM auth.accounts AS account WHERE account.id = ?`, e.account).
		Scan(t.Context(), &state.Erased, &state.LastLogin, &state.Sessions))
	return state
}

// holdBefore makes every statement that names marker and writes
// auth.accounts (or auth.tokens) wait for the advisory lock the returned
// transaction holds. The barrier is a statement trigger: it runs before the
// statement takes any row lock. The erasure runs as one function call and
// never names marker.
func (e erasureLogin) holdBefore(t *testing.T, table, marker string) bun.Tx {
	t.Helper()
	ctx := t.Context()
	_, err := e.db.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION public.erasure_login_barrier() RETURNS trigger
		LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
		BEGIN
			IF strpos(current_query(), TG_ARGV[0]) > 0 THEN
				PERFORM pg_advisory_xact_lock_shared(5546);
			END IF;
			RETURN NULL;
		END;
		$$`)
	require.NoError(t, err)
	_, err = e.db.ExecContext(ctx, `CREATE TRIGGER erasure_login_barrier BEFORE INSERT OR UPDATE ON `+table+`
		FOR EACH STATEMENT EXECUTE FUNCTION public.erasure_login_barrier('`+marker+`')`)
	require.NoError(t, err)
	holder, err := e.db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = holder.Rollback() })
	_, err = holder.ExecContext(ctx, `SELECT pg_advisory_xact_lock(5546)`)
	require.NoError(t, err)
	return holder
}

// The login has checked the credentials and the guardian role and stops
// right before it stamps the last login; the erasure anonymizes the account
// and commits; the login goes on. It must mint nothing: no session, no last
// login on the tombstone, and the answer of an unknown account, which the
// login route renders like wrong credentials.
func TestParentLoginMintsNothingForAnAccountErasedMeanwhile(t *testing.T) {
	t.Parallel()
	e := newErasureLogin(t)
	before := e.state(t)
	holder := e.holdBefore(t, "auth.accounts", "last_login")
	login := e.startLogin()
	e.waitFor(t, "last_login", "advisory")

	assert.Equal(t, 1, e.erase(t), "the erasure does not wait for the stopped login")
	require.NoError(t, holder.Rollback())
	result := <-login

	requireRefusedLikeWrongCredentials(t, result.err)
	assert.Empty(t, result.refresh)
	after := e.state(t)
	assert.True(t, after.Erased)
	assert.Equal(t, before.LastLogin, after.LastLogin, "the tombstone records no login")
	assert.Zero(t, after.Sessions, "no session exists for the erased account")
}

// The erasure has anonymized the account and holds it until it commits; the
// login reaches the account meanwhile. It waits no longer than the erasure's
// transaction and then refuses the account, minting nothing.
func TestParentLoginWaitsForAnErasureAndMintsNothing(t *testing.T) {
	t.Parallel()
	e := newErasureLogin(t)
	before := e.state(t)
	held := make(chan struct{})
	release := make(chan struct{})
	erased := make(chan int, 1)
	go func() {
		var count int
		err := testpkg.WithinAdminTransaction(e.base, func(ctx context.Context) error {
			var runErr error
			count, runErr = e.owner.EraseUnusedAccounts(ctx)
			close(held)
			<-release
			return runErr
		})
		if err != nil {
			count = -1
		}
		erased <- count
	}()
	<-held
	login := e.startLogin()
	e.waitFor(t, "auth", "")
	close(release)
	require.Equal(t, 1, <-erased)

	select {
	case result := <-login:
		requireRefusedLikeWrongCredentials(t, result.err)
		assert.Empty(t, result.refresh)
	case <-time.After(5 * time.Second):
		t.Fatal("the login kept waiting after the erasure committed")
	}
	after := e.state(t)
	assert.True(t, after.Erased)
	assert.Equal(t, before.LastLogin, after.LastLogin, "the tombstone records no login")
	assert.Zero(t, after.Sessions, "no session exists for the erased account")
}

// The login has stamped the last login, holds the account and stops before
// it writes the session; the erasure passes the account over without
// waiting. The login then completes, and the next run, once the account is
// free, anonymizes it with its new session.
func TestErasurePassesOverAnAccountALoginHolds(t *testing.T) {
	t.Parallel()
	e := newErasureLogin(t)
	holder := e.holdBefore(t, "auth.tokens", "INSERT INTO")
	login := e.startLogin()
	e.waitFor(t, "INSERT INTO", "advisory")

	started := time.Now()
	assert.Zero(t, e.erase(t), "the erasure passes over the account the login holds")
	assert.Less(t, time.Since(started), time.Second, "the erasure does not wait for the login")
	require.NoError(t, holder.Rollback())
	result := <-login
	require.NoError(t, result.err, "the login does not fail beside the erasure")
	require.NotEmpty(t, result.refresh)
	state := e.state(t)
	assert.False(t, state.Erased)
	assert.True(t, state.LastLogin.Valid)
	assert.Equal(t, 1, state.Sessions)

	assert.Equal(t, 1, e.erase(t), "the next run erases the account")
	state = e.state(t)
	assert.True(t, state.Erased)
	assert.Zero(t, state.Sessions)
}
