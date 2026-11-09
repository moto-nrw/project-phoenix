package behavior_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// The push cleanup after a school-scoped revocation (#5878, ADR 0161 step 6)
// deletes the subscriptions of every revoked family and the unbound ones at
// each school × portal in one administrative unit, in the order of the single
// units it replaces. It commits all of it or nothing: a failing position
// rolls back the deletions before it, and a failed start deletes nothing and
// is not retried. Each deletion stays its own statement, so no read waits for
// more than one position's row locks. Rows of another school or account are
// never touched.

const pushCleanupRevokeReason = "role_changed"

type pushCleanupAccount struct {
	accountID int64
	families  []string
	// own: one staff row per family, then the unbound staff row at the school.
	own []string
	// survivors: the unbound row at the other school, and another account's
	// bound and unbound rows at this school.
	survivors    []string
	otherAccount int64
}

// newPushCleanupAccount registers an account with families sessions at
// tenantID, a membership at otherTenantID, and the push rows the cleanup
// must delete or keep.
func newPushCleanupAccount(t *testing.T, db *bun.DB, service testAuthService, tenantID, otherTenantID int64, name string, families int) pushCleanupAccount {
	t.Helper()
	ctx := testpkg.TenantContext(tenantID)
	register := func(suffix string) int64 {
		email, username := uniqueTestCredentials(name + suffix)
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
		testpkg.MapAccountToTenant(t, db, account.ID, otherTenantID)
		for range families {
			_, _, err = service.Login(ctx, email, testPassword)
			require.NoError(t, err)
		}
		return account.ID
	}
	a := pushCleanupAccount{accountID: register(""), otherAccount: register("-other")}
	endpoint := func(accountID int64, label string) string {
		return fmt.Sprintf("https://fcm.googleapis.com/push-cleanup-%d-%s", accountID, label)
	}
	for i := range families {
		family := tokenFamilyID(t, db, a.accountID, i)
		a.families = append(a.families, family)
		a.own = append(a.own, endpoint(a.accountID, fmt.Sprintf("family-%d", i+1)))
		insertStaffPush(t, db, a.accountID, tenantID, a.own[i], family)
	}
	a.own = append(a.own, endpoint(a.accountID, "unbound-here"))
	insertStaffPush(t, db, a.accountID, tenantID, a.own[families], "")

	a.survivors = []string{endpoint(a.accountID, "unbound-other-school")}
	insertStaffPush(t, db, a.accountID, otherTenantID, a.survivors[0], "")
	otherBound, otherUnbound := endpoint(a.otherAccount, "bound"), endpoint(a.otherAccount, "unbound")
	insertStaffPush(t, db, a.otherAccount, tenantID, otherBound, tokenFamilyID(t, db, a.otherAccount, 0))
	insertStaffPush(t, db, a.otherAccount, tenantID, otherUnbound, "")
	a.survivors = append(a.survivors, otherBound, otherUnbound)
	return a
}

func (a pushCleanupAccount) revoke(t *testing.T, service testAuthService, tenantID int64) {
	t.Helper()
	require.NoError(t, service.RevokeAllTokensWithReason(testpkg.TenantContext(tenantID), a.accountID, pushCleanupRevokeReason))
}

// pushRowsLeft returns the endpoints of both accounts' rows still stored.
func (a pushCleanupAccount) pushRowsLeft(t *testing.T, db *bun.DB) []string {
	t.Helper()
	var endpoints []string
	require.NoError(t, db.NewSelect().
		ColumnExpr("endpoint").
		TableExpr("iot.push_subscriptions").
		Where("account_id IN (?, ?)", a.accountID, a.otherAccount).
		OrderExpr("endpoint").
		Scan(context.Background(), &endpoints))
	return endpoints
}

func sortedEndpoints(groups ...[]string) []string {
	var all []string
	for _, group := range groups {
		all = append(all, group...)
	}
	slices.Sort(all)
	return all
}

func liveTokensAt(t *testing.T, db *bun.DB, accountID, tenantID int64) int {
	t.Helper()
	count, err := db.NewSelect().TableExpr("auth.tokens").
		Where("account_id = ?", accountID).Where("tenant_id = ?", tenantID).
		Count(context.Background())
	require.NoError(t, err)
	return int(count)
}

// requireCleanConnection proves that the single pooled connection carries
// neither role nor school of the cleanup's unit into the next transaction.
func requireCleanConnection(t *testing.T, db *bun.DB) {
	t.Helper()
	var user, school string
	require.NoError(t, db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		return tx.NewRaw(`SELECT current_user, COALESCE(current_setting('app.current_tenant_id', true), '')`).Scan(ctx, &user, &school)
	}))
	require.NotEqual(t, "phoenix_admin", user)
	require.NotEqual(t, "phoenix_tenant", user)
	require.Empty(t, school)
}

func statementsStartingWith(queries []string, prefix string) int {
	count := 0
	for _, query := range queries {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), prefix) {
			count++
		}
	}
	return count
}

func TestPushCleanupDeletesEveryFamilyAndSchoolInOneAdminUnit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	db.SetMaxOpenConns(1)
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	otherTenantID, _ := testpkg.CreateTestTenant(t, db)
	counter := testpkg.CaptureQueries(t, db)

	revokeFamilies := func(families int) []string {
		a := newPushCleanupAccount(t, db, service, tenantID, otherTenantID, fmt.Sprintf("push-unit-%d", families), families)
		counter.Reset()
		a.revoke(t, service, tenantID)
		statements := counter.Queries()

		require.Equal(t, sortedEndpoints(a.survivors), a.pushRowsLeft(t, db), "%d families: own rows at this school go, the rest stays", families)
		require.Zero(t, liveTokensAt(t, db, a.accountID, tenantID))
		require.Equal(t, 2, statementsStartingWith(statements, "BEGIN"), "%d families: the revocation and one cleanup unit", families)
		require.Equal(t, families+1, statementsStartingWith(statements, "DELETE FROM IOT.PUSH_SUBSCRIPTIONS"), "%d families: one statement per family and school × portal", families)
		requireCleanConnection(t, db)
		return statements
	}
	querybudgets.AssertQueryBudget(t, "identity.push_cleanup.role_changed.three_families", counter.Measurement(revokeFamilies(3)))
	querybudgets.AssertQueryBudget(t, "identity.push_cleanup.role_changed.five_families", counter.Measurement(revokeFamilies(5)))

	// Logout: one family at one school.
	email, username := uniqueTestCredentials("push-unit-logout")
	account, err := service.Register(testpkg.TenantContext(tenantID), email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	_, refresh, err := service.Login(testpkg.TenantContext(tenantID), email, testPassword)
	require.NoError(t, err)
	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/push-unit-logout-bound", tokenFamilyID(t, db, account.ID, 0))
	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/push-unit-logout-unbound", "")
	counter.Reset()
	require.NoError(t, service.LogoutWithAudit(testpkg.TenantContext(tenantID), refresh, "", ""))
	statements := counter.Queries()
	require.Zero(t, countStaffPush(t, db, account.ID, ""))
	querybudgets.AssertQueryBudget(t, "identity.push_cleanup.logout", counter.Measurement(statements))
	requireCleanConnection(t, db)
}

// installPushDeleteFault makes every DELETE of a push row whose endpoint ends
// in "-fail" raise, so the cleanup fails at exactly that position.
func installPushDeleteFault(t *testing.T, db *bun.DB) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		CREATE FUNCTION iot.push_cleanup_test_fault() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF OLD.endpoint LIKE '%-fail' THEN
				RAISE EXCEPTION 'injected push delete fault at %', OLD.endpoint;
			END IF;
			RETURN OLD;
		END $$;
		CREATE TRIGGER push_cleanup_test_fault BEFORE DELETE ON iot.push_subscriptions
			FOR EACH ROW EXECUTE FUNCTION iot.push_cleanup_test_fault();`)
	require.NoError(t, err)
}

func markPushRowFailing(t *testing.T, db *bun.DB, endpoint string) string {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `UPDATE iot.push_subscriptions SET endpoint = endpoint || '-fail' WHERE endpoint = ?`, endpoint)
	require.NoError(t, err)
	return endpoint + "-fail"
}

func TestPushCleanupRollsBackEveryDeletionWhenOnePositionFails(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	db.SetMaxOpenConns(1)
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	otherTenantID, _ := testpkg.CreateTestTenant(t, db)
	installPushDeleteFault(t, db)

	// Positions in cleanup order: families 1–3, then the unbound staff row at
	// the school.
	for position, label := range []string{"family 1", "family 2", "family 3", "unbound at the school"} {
		a := newPushCleanupAccount(t, db, service, tenantID, otherTenantID, fmt.Sprintf("push-fault-%d", position), 3)
		own := slices.Clone(a.own)
		own[position] = markPushRowFailing(t, db, own[position])

		a.revoke(t, service, tenantID)

		require.Equal(t, sortedEndpoints(own, a.survivors), a.pushRowsLeft(t, db), "fault at %s: no push row is deleted", label)
		require.Zero(t, liveTokensAt(t, db, a.accountID, tenantID), "fault at %s: the revocation itself stays committed", label)
		requireCleanConnection(t, db)
	}
}

// beginFault makes one BEGIN of its pool fail with a broken connection, after
// skipping a given number of BEGINs, and counts every BEGIN attempted.
type beginFault struct {
	skip   atomic.Int64
	armed  atomic.Bool
	begins atomic.Int64
}

func (f *beginFault) armAfter(skip int64) {
	f.skip.Store(skip)
	f.armed.Store(true)
}

type beginFaultConnector struct {
	inner driver.Connector
	fault *beginFault
}

func (c beginFaultConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &beginFaultConn{Conn: conn.(*pgdriver.Conn), fault: c.fault}, nil
}

func (c beginFaultConnector) Driver() driver.Driver { return c.inner.Driver() }

// beginFaultConn embeds the pgdriver connection, so database/sql sees every
// optional interface it implements; only BeginTx is intercepted.
type beginFaultConn struct {
	*pgdriver.Conn
	fault *beginFault
}

func (c *beginFaultConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.fault.begins.Add(1)
	if c.fault.armed.Load() && c.fault.skip.Add(-1) < 0 && c.fault.armed.CompareAndSwap(true, false) {
		return nil, driver.ErrBadConn
	}
	return c.Conn.BeginTx(ctx, opts)
}

func TestPushCleanupStartFailureDeletesNothingAndIsNotRetried(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	fault := &beginFault{}
	db := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(sql.OpenDB(beginFaultConnector{
		inner: pgdriver.NewConnector(pgdriver.WithDSN(testpkg.IsolatedTestDSN(t))), fault: fault,
	})))
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	otherTenantID, _ := testpkg.CreateTestTenant(t, db)
	a := newPushCleanupAccount(t, db, service, tenantID, otherTenantID, "push-start-fault", 3)

	before := fault.begins.Load()
	// The revocation's own unit starts; the cleanup's first start fails.
	fault.armAfter(1)
	a.revoke(t, service, tenantID)

	require.Equal(t, int64(2), fault.begins.Load()-before, "the revocation and the failed cleanup start, no retry")
	require.False(t, fault.armed.Load(), "the injected fault was consumed by the cleanup start")
	require.Equal(t, sortedEndpoints(a.own, a.survivors), a.pushRowsLeft(t, db), "a failed start deletes nothing")
	require.Zero(t, liveTokensAt(t, db, a.accountID, tenantID))
}

// pushRowLockedOrGone reports whether another transaction holds the row's
// lock, or the row is already deleted.
func pushRowLockedOrGone(t *testing.T, db *bun.DB, endpoint string) bool {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var ids []int64
	if err := tx.NewRaw(`SELECT id FROM iot.push_subscriptions WHERE endpoint = ? FOR UPDATE NOWAIT`, endpoint).Scan(ctx, &ids); err != nil {
		return true
	}
	return len(ids) == 0
}

func waitForPushLockWaiter(t *testing.T, db *bun.DB) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(context.Background(), &waiting)
		return err == nil && waiting >= 1
	}, 10*time.Second, 10*time.Millisecond)
}

// Two family rows held by other transactions are waited for one statement at
// a time. pgdriver gives up a read that waits longer than its read timeout
// (10 s by default, not overridden in production), and PostgreSQL sends
// nothing while a statement waits for a lock. Scaled down as in #6239: under
// a 1 s read timeout each row is held 0.7 s from the moment the cleanup is
// seen waiting for it. One statement per family reads within the timeout; a
// statement over both families would wait about 1.4 s in one read, time out
// and roll the cleanup back.
func TestPushCleanupWaitsForEachFamilyInItsOwnStatement(t *testing.T) {
	t.Parallel()
	const readTimeout, hold = time.Second, 700 * time.Millisecond
	db := testpkg.SetupIsolatedTestDB(t)
	dsn, err := url.Parse(testpkg.IsolatedTestDSN(t))
	require.NoError(t, err)
	query := dsn.Query()
	query.Set("read_timeout", readTimeout.String())
	dsn.RawQuery = query.Encode()
	pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(testpkg.OpenPostgresSQL(dsn.String())))
	t.Cleanup(func() { _ = pool.Close() })
	service := setupAuthService(t, pool)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	otherTenantID, _ := testpkg.CreateTestTenant(t, db)
	a := newPushCleanupAccount(t, db, service, tenantID, otherTenantID, "push-read-timeout", 2)
	ctx := context.Background()

	holdRow := func(endpoint string) bun.Tx {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = tx.Rollback() })
		_, err = tx.ExecContext(ctx, `SELECT 1 FROM iot.push_subscriptions WHERE endpoint = ? FOR UPDATE`, endpoint)
		require.NoError(t, err)
		return tx
	}
	first, second := holdRow(a.own[0]), holdRow(a.own[1])

	done := make(chan struct{})
	go func() {
		defer close(done)
		a.revoke(t, service, tenantID)
	}()
	waitForPushLockWaiter(t, db)
	waitStarted := time.Now()
	// The hold times are the subject of the test.
	time.Sleep(hold)
	require.NoError(t, first.Commit())
	require.Eventually(t, func() bool { return pushRowLockedOrGone(t, db, a.own[0]) },
		15*time.Second, 10*time.Millisecond, "the cleanup never deleted the first family's row")
	waitForPushLockWaiter(t, db)
	time.Sleep(hold)
	require.NoError(t, second.Commit())

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the revocation did not return")
	}
	require.Greater(t, time.Since(waitStarted), readTimeout, "the waits together exceed the read timeout")
	require.Equal(t, sortedEndpoints(a.survivors), a.pushRowsLeft(t, db), "each statement waited within the read timeout and the cleanup committed")
}
