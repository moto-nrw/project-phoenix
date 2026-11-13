//go:build testhooks

package account_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The role replacement removes every other staff role at the school as one
// set (#5882). These tests pin what the removal ends and records, at the
// request boundary, on a healthy database and at every statement that can
// fail inside the request transaction.

type roleSetAccount struct {
	accountID, otherTenantID int64
	target, first, second    int64
	guardian                 int64
	familyA1, familyA2, famB string
	pushA1, pushAUnbound, pB string
}

type roleSetState struct {
	Roles        []int64
	TokensHere   int
	TokensOther  int
	RevokedAudit []string
	PushHere     []string
	PushOther    []string
}

// newRoleSetAccount seeds an account active at this school and at a second
// one. At this school it holds the target (when targetHeld), two staff roles
// and a guardian-tier role, two refresh families and two push subscriptions;
// the second school keeps one family with one subscription.
func newRoleSetAccount(t *testing.T, f *rbacBudgetFixture, name string, targetHeld bool) roleSetAccount {
	t.Helper()
	ctx := context.Background()
	tenantID := testpkg.Tenant(t)
	account := testpkg.CreateTestAccount(t, f.db, name)
	testpkg.EnsureAccountTenant(t, f.db, account.ID, tenantID)
	otherID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, f.db, otherID)
	testpkg.EnsureAccountTenant(t, f.db, account.ID, otherID)

	s := roleSetAccount{accountID: account.ID, otherTenantID: otherID}
	s.target = testpkg.CreateTestRole(t, f.db, name+"-target").ID
	s.first = testpkg.CreateTestRole(t, f.db, name+"-first").ID
	s.second = testpkg.CreateTestRole(t, f.db, name+"-second").ID
	s.guardian = testpkg.CreateTestRole(t, f.db, name+"-guardian").ID
	_, err := f.db.ExecContext(ctx, "UPDATE auth.roles SET base_role = 'guardian' WHERE id = ?", s.guardian)
	require.NoError(t, err)
	held := []int64{s.first, s.guardian, s.second}
	if targetHeld {
		held = append([]int64{s.target}, held...)
	}
	for _, roleID := range held {
		_, err := f.db.ExecContext(ctx, "INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)", account.ID, roleID, tenantID)
		require.NoError(t, err)
	}

	s.familyA1 = testpkg.CreateTestToken(t, f.db, account.ID, "refresh").FamilyID
	s.familyA2 = testpkg.CreateTestToken(t, f.db, account.ID, "refresh").FamilyID
	s.famB = testpkg.CreateTestTokenForTenant(t, f.db, otherID, account.ID).FamilyID
	s.pushA1 = "https://fcm.googleapis.com/" + name + "-a1"
	s.pushAUnbound = "https://fcm.googleapis.com/" + name + "-a-unbound"
	s.pB = "https://fcm.googleapis.com/" + name + "-b"
	insertRoleSetPush(t, f.db, account.ID, tenantID, s.pushA1, s.familyA1)
	insertRoleSetPush(t, f.db, account.ID, tenantID, s.pushAUnbound, "")
	insertRoleSetPush(t, f.db, account.ID, otherID, s.pB, s.famB)
	return s
}

func insertRoleSetPush(t *testing.T, db *bun.DB, accountID, tenantID int64, endpoint, familyID string) {
	t.Helper()
	_, err := db.NewRaw(`INSERT INTO iot.push_subscriptions
		(tenant_id, account_id, portal, endpoint, p256dh, auth, token_family_id)
		VALUES (?, ?, 'staff', ?, 'p256dh-key', 'auth-key', ?)`,
		tenantID, accountID, endpoint, familyID).Exec(context.Background())
	require.NoError(t, err)
}

func (s roleSetAccount) state(t *testing.T, db *bun.DB) roleSetState {
	t.Helper()
	ctx := context.Background()
	tenantID := testpkg.Tenant(t)
	var state roleSetState
	require.NoError(t, db.NewRaw(`SELECT role_id FROM auth.account_roles WHERE account_id = ? AND tenant_id = ? ORDER BY role_id`,
		s.accountID, tenantID).Scan(ctx, &state.Roles))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE account_id = ? AND tenant_id = ?`, s.accountID, tenantID).Scan(ctx, &state.TokensHere))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE account_id = ? AND tenant_id = ?`, s.accountID, s.otherTenantID).Scan(ctx, &state.TokensOther))
	require.NoError(t, db.NewRaw(`
		SELECT concat_ws('|', tenant_id, event_type, success, metadata->>'reason', metadata->>'portal_scope',
			metadata->>'revoked_token_count', (metadata->>'family_fingerprint') IS NOT NULL)
		FROM audit.auth_events WHERE account_id = ? AND event_type = 'token_revoked' ORDER BY id`, s.accountID).Scan(ctx, &state.RevokedAudit))
	require.NoError(t, db.NewRaw(`SELECT endpoint FROM iot.push_subscriptions WHERE account_id = ? AND tenant_id = ? ORDER BY endpoint`,
		s.accountID, tenantID).Scan(ctx, &state.PushHere))
	require.NoError(t, db.NewRaw(`SELECT endpoint FROM iot.push_subscriptions WHERE account_id = ? AND tenant_id = ? ORDER BY endpoint`,
		s.accountID, s.otherTenantID).Scan(ctx, &state.PushOther))
	return state
}

func (s roleSetAccount) replace(t *testing.T, f *rbacBudgetFixture, token string) int {
	t.Helper()
	req := testutil.NewJSONRequest(t, http.MethodPut, fmt.Sprintf("/auth/accounts/%d/roles/", s.accountID),
		map[string]string{"role_id": fmt.Sprint(s.target)})
	req.Header.Set("Authorization", "Bearer "+token)
	rr := testutil.ExecuteRequestForTest(t, f.router, req)
	return rr.Code
}

func sortedIDs(ids ...int64) []int64 {
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// revokedAt is the audit row one revoked fixture family leaves at a school.
func revokedAt(tenantID int64) string {
	return fmt.Sprintf("%d|token_revoked|t|role_changed|unknown|1|t", tenantID)
}

// TestReplaceAccountRoleEndsEveryStaffGrantAsOneSet pins the healthy path:
// both staff roles go, the guardian-tier role stays, every session of this
// school ends with one audit row per family, its push subscriptions go after
// commit, and the second school keeps its session and subscription. Whether
// the assignment of a new target or the first removal revokes, the evidence
// is the same.
func TestReplaceAccountRoleEndsEveryStaffGrantAsOneSet(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixtureOn(t, testpkg.SetupTestDB(t))
	token, _ := f.accountToken(t, "role-set-admin", "users:manage")
	for _, targetHeld := range []bool{true, false} {
		s := newRoleSetAccount(t, f, fmt.Sprintf("role-set-healthy-%t", targetHeld), targetHeld)
		require.Equal(t, http.StatusNoContent, s.replace(t, f, token))
		got := s.state(t, f.db)
		require.Equal(t, sortedIDs(s.target, s.guardian), got.Roles, "target held: %t", targetHeld)
		require.Zero(t, got.TokensHere)
		require.Equal(t, 1, got.TokensOther, "the other school keeps its session")
		require.ElementsMatch(t, []string{revokedAt(testpkg.Tenant(t)), revokedAt(testpkg.Tenant(t))}, got.RevokedAudit,
			"one audit row per revoked family of this school, none for the other school")
		require.Empty(t, got.PushHere, "the revoked families' and this school's unbound subscriptions go after commit")
		require.Equal(t, []string{s.pB}, got.PushOther)
	}
}

// TestReplaceAccountRoleFailsWholeAtEveryStatement injects a failure into
// each statement the removal runs inside the request transaction. Every one
// answers 500 and leaves roles, sessions, audit and push subscriptions as
// they were: nothing is partly revoked. A failing push cleanup runs after the
// commit and is only logged: the request succeeds, the revocation stands,
// and the cleanup stops at its first failing delete.
func TestReplaceAccountRoleFailsWholeAtEveryStatement(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := newRBACBudgetFixtureOn(t, db)
	token, _ := f.accountToken(t, "role-set-fault-admin", "users:manage")
	for _, tc := range []struct {
		name    string
		trigger func(s roleSetAccount) (table, when string)
		status  int
	}{
		{"second role delete", func(s roleSetAccount) (string, string) {
			return "auth.account_roles", fmt.Sprintf("BEFORE DELETE ON auth.account_roles FOR EACH ROW WHEN (OLD.role_id = %d)", s.second)
		}, http.StatusInternalServerError},
		{"first role delete", func(s roleSetAccount) (string, string) {
			return "auth.account_roles", fmt.Sprintf("BEFORE DELETE ON auth.account_roles FOR EACH ROW WHEN (OLD.role_id = %d)", s.first)
		}, http.StatusInternalServerError},
		{"session revocation", func(s roleSetAccount) (string, string) {
			return "auth.tokens", fmt.Sprintf("BEFORE DELETE ON auth.tokens FOR EACH ROW WHEN (OLD.family_id = '%s')", s.familyA2)
		}, http.StatusInternalServerError},
		{"revocation audit", func(s roleSetAccount) (string, string) {
			return "audit.auth_events", fmt.Sprintf("BEFORE INSERT ON audit.auth_events FOR EACH ROW WHEN (NEW.account_id = %d)", s.accountID)
		}, http.StatusInternalServerError},
		{"push cleanup after commit", func(s roleSetAccount) (string, string) {
			return "iot.push_subscriptions", fmt.Sprintf("BEFORE DELETE ON iot.push_subscriptions FOR EACH ROW WHEN (OLD.account_id = %d)", s.accountID)
		}, http.StatusNoContent},
	} {
		s := newRoleSetAccount(t, f, fmt.Sprintf("role-set-fault-%d", testpkg.UniqueTestTenantID(t)), true)
		before := s.state(t, db)
		table, when := tc.trigger(s)
		installFailingTrigger(t, db, table, when)
		require.Equal(t, tc.status, s.replace(t, f, token), tc.name)
		dropFailingTrigger(t, db, table)
		got := s.state(t, db)
		if tc.status != http.StatusNoContent {
			require.Equal(t, before, got, "%s: the failed replacement leaves every effect as it was", tc.name)
			continue
		}
		require.Equal(t, sortedIDs(s.target, s.guardian), got.Roles, tc.name)
		require.Zero(t, got.TokensHere, tc.name)
		require.Len(t, got.RevokedAudit, 2, tc.name)
		require.Equal(t, before.PushHere, got.PushHere, "%s: the logged cleanup failure keeps the subscriptions", tc.name)
	}
}

func installFailingTrigger(t *testing.T, db *bun.DB, table, when string) {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `CREATE OR REPLACE FUNCTION public.role_set_fault() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'injected role-set fault'; END $$`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE TRIGGER role_set_fault %s EXECUTE FUNCTION public.role_set_fault()`, when))
	require.NoError(t, err, table)
}

func dropFailingTrigger(t *testing.T, db *bun.DB, table string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), fmt.Sprintf(`DROP TRIGGER role_set_fault ON %s`, table))
	require.NoError(t, err)
}

// replaceAsync sends the replacement from a goroutine and reports its status.
func (s roleSetAccount) replaceAsync(t *testing.T, f *rbacBudgetFixture, token string) <-chan int {
	t.Helper()
	req := testutil.NewJSONRequest(t, http.MethodPut, fmt.Sprintf("/auth/accounts/%d/roles/", s.accountID),
		map[string]string{"role_id": fmt.Sprint(s.target)})
	req.Header.Set("Authorization", "Bearer "+token)
	status := make(chan int, 1)
	go func() { status <- testutil.ExecuteRequestForTest(t, f.router, req).Code }()
	return status
}

// waitForRoleSetLockWaiters blocks until n backends of this isolated database
// wait for a lock.
func waitForRoleSetLockWaiters(t *testing.T, db *bun.DB, n int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(context.Background(), &waiting)
		return err == nil && waiting >= n
	}, 10*time.Second, 10*time.Millisecond)
}

func awaitRoleSetStatus(t *testing.T, status <-chan int) int {
	t.Helper()
	select {
	case code := <-status:
		return code
	case <-time.After(30 * time.Second):
		t.Fatal("the replacement did not answer")
		return 0
	}
}

// A role deletion that removes the observed staff roles while the
// replacement waits for their rows must not spare the sessions: the
// replacement still revokes them with their audit, as the single removals
// did after their assignment check had seen the role (#5882 review P1).
func TestReplaceAccountRoleRevokesWhenARoleDeletionWinsTheRow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := newRBACBudgetFixtureOn(t, db)
	token, _ := f.accountToken(t, "role-set-race-admin", "users:manage")
	s := newRoleSetAccount(t, f, "role-set-race", true)
	ctx := context.Background()

	// Stands in for DeleteRole's DeleteRoleAssignments of both staff roles.
	deletion, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = deletion.Rollback() }()
	_, err = deletion.ExecContext(ctx, `DELETE FROM auth.account_roles WHERE account_id = ? AND role_id IN (?, ?)`,
		s.accountID, s.first, s.second)
	require.NoError(t, err)

	status := s.replaceAsync(t, f, token)
	waitForRoleSetLockWaiters(t, db, 1)
	require.NoError(t, deletion.Commit())

	require.Equal(t, http.StatusNoContent, awaitRoleSetStatus(t, status))
	got := s.state(t, db)
	require.Equal(t, sortedIDs(s.target, s.guardian), got.Roles)
	require.Zero(t, got.TokensHere, "the observed role set ends every session of this school")
	require.ElementsMatch(t, []string{revokedAt(testpkg.Tenant(t)), revokedAt(testpkg.Tenant(t))}, got.RevokedAudit)
	require.Equal(t, 1, got.TokensOther)
}

// Two role rows held by different transactions are waited for one statement
// at a time, so no single read waits longer than for one of them (#5882
// review P2). The driver gives up a read that waits longer than its read
// timeout (pgdriver: 10 s by default, not overridden in production), and
// PostgreSQL sends nothing while a statement waits for a lock. Scaled down
// as in #6239: under a 1 s read timeout each row is held 0.7 s from the moment
// the replacement is seen waiting for it, so one statement per row reads
// within the timeout while a statement over both rows would wait about 1.4 s
// in one read and answer 500.
func TestReplaceAccountRoleWaitsForEachRoleRowSeparately(t *testing.T) {
	t.Parallel()
	const readTimeout, hold = time.Second, 700 * time.Millisecond
	db := testpkg.SetupIsolatedTestDB(t)
	f := newRBACBudgetFixtureOn(t, db)
	token, _ := f.accountToken(t, "role-set-wait-admin", "users:manage")
	s := newRoleSetAccount(t, f, "role-set-wait", true)
	// Only the request transaction needs the artificial read timeout.
	// Fixture writes and final readback use the normal pool so shared-server
	// I/O does not consume the lock test's deliberately narrow read budget.
	pool := shortReadTimeoutPool(t, readTimeout)
	f.router = testutil.NewTenantRouter(pool)
	f.router.Mount("/auth", f.resource.Router(api.TenantSessionChain))
	ctx := context.Background()

	holdRow := func(roleID int64) bun.Tx {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = tx.Rollback() })
		_, err = tx.ExecContext(ctx, `SELECT 1 FROM auth.account_roles WHERE account_id = ? AND role_id = ? FOR UPDATE`, s.accountID, roleID)
		require.NoError(t, err)
		return tx
	}
	first, second := holdRow(s.first), holdRow(s.second)

	status := s.replaceAsync(t, f, token)
	waitForRoleSetLockWaiters(t, db, 1)
	waitStarted := time.Now()
	// The hold times are the subject of the test.
	time.Sleep(hold)
	require.NoError(t, first.Commit())
	require.Eventually(t, func() bool { return roleRowLockedElsewhere(t, db, s.accountID, s.first) },
		15*time.Second, 10*time.Millisecond, "the replacement never took the first role row")
	waitForRoleSetLockWaiters(t, db, 1)
	time.Sleep(hold)
	require.NoError(t, second.Commit())

	require.Equal(t, http.StatusNoContent, awaitRoleSetStatus(t, status), "each statement waits for one row within the read timeout")
	require.Greater(t, time.Since(waitStarted), readTimeout, "the waits together exceed the read timeout")
	got := s.state(t, db)
	require.Equal(t, sortedIDs(s.target, s.guardian), got.Roles)
	require.Zero(t, got.TokensHere)
}

// shortReadTimeoutPool opens a second pool on the test's isolated clone whose
// driver gives up a read after readTimeout.
func shortReadTimeoutPool(t *testing.T, readTimeout time.Duration) *bun.DB {
	t.Helper()
	dsn, err := url.Parse(testpkg.IsolatedTestDSN(t))
	require.NoError(t, err)
	query := dsn.Query()
	query.Set("read_timeout", readTimeout.String())
	dsn.RawQuery = query.Encode()
	pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(testpkg.OpenPostgresSQL(dsn.String())))
	t.Cleanup(func() { _ = pool.Close() })
	return pool
}

// roleRowLockedElsewhere reports whether another transaction holds the
// assignment row's lock.
func roleRowLockedElsewhere(t *testing.T, db *bun.DB, accountID, roleID int64) bool {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `SELECT 1 FROM auth.account_roles WHERE account_id = ? AND role_id = ? FOR UPDATE NOWAIT`, accountID, roleID)
	return err != nil
}

// Every other staff assignment disappears after the replacement listed it:
// one under a row lock the replacement waits for, the other committed before
// the replacement reaches it. Before #5882 a removal whose assignment check
// came after such a commit skipped the revocation. Fail-safe by decision of
// 09.11.2026 (#5882): the observed set is revoked all the same, at this
// school only; the other school's session and subscription stay (#5882
// review R2).
func TestReplaceAccountRoleRevokesWhenEveryListedRoleVanishes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := newRBACBudgetFixtureOn(t, db)
	token, _ := f.accountToken(t, "role-set-vanish-admin", "users:manage")
	s := newRoleSetAccount(t, f, "role-set-vanish", true)
	ctx := context.Background()

	// Stands in for DeleteRole of the first staff role: it holds the row.
	firstDeletion, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = firstDeletion.Rollback() }()
	_, err = firstDeletion.ExecContext(ctx, `SELECT 1 FROM auth.account_roles WHERE account_id = ? AND role_id = ? FOR UPDATE`, s.accountID, s.first)
	require.NoError(t, err)

	status := s.replaceAsync(t, f, token)
	waitForRoleSetLockWaiters(t, db, 1)
	// DeleteRole of the second staff role commits after the listing.
	_, err = db.ExecContext(ctx, `DELETE FROM auth.account_roles WHERE account_id = ? AND role_id = ?`, s.accountID, s.second)
	require.NoError(t, err)
	_, err = firstDeletion.ExecContext(ctx, `DELETE FROM auth.account_roles WHERE account_id = ? AND role_id = ?`, s.accountID, s.first)
	require.NoError(t, err)
	require.NoError(t, firstDeletion.Commit())

	require.Equal(t, http.StatusNoContent, awaitRoleSetStatus(t, status))
	got := s.state(t, db)
	require.Equal(t, sortedIDs(s.target, s.guardian), got.Roles)
	require.Zero(t, got.TokensHere, "the listed role set ends every session of this school")
	require.ElementsMatch(t, []string{revokedAt(testpkg.Tenant(t)), revokedAt(testpkg.Tenant(t))}, got.RevokedAudit,
		"one audit row per revoked family of this school")
	require.Empty(t, got.PushHere, "the revoked families' and this school's unbound subscriptions go after commit")
	require.Equal(t, 1, got.TokensOther, "the other school keeps its session")
	require.Equal(t, []string{s.pB}, got.PushOther, "the other school keeps its subscription")
}
