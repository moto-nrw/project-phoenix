package behavior_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A role replacement removes the account's other staff roles under the
// account lock that every session mint and renewal takes first (#5882).
// These tests run both orders on two connections: a mint that holds the lock
// first is revoked by the replacement, and a renewal that waits for the
// replacement finds its session gone. Either way no session of this school
// outlives the replacement.

type fenceAccount struct {
	email                 string
	accountID             int64
	target, first, second int64
}

func newFenceAccount(t *testing.T, db *bun.DB, service testAuthService, name string) fenceAccount {
	t.Helper()
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials(name)
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	a := fenceAccount{email: email, accountID: account.ID}
	a.target = testpkg.CreateTestRole(t, db, name+"-target").ID
	a.first = testpkg.CreateTestRole(t, db, name+"-first").ID
	a.second = testpkg.CreateTestRole(t, db, name+"-second").ID
	for _, roleID := range []int64{a.target, a.first, a.second} {
		_, err := db.ExecContext(context.Background(), "INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			account.ID, roleID, testpkg.Tenant(t))
		require.NoError(t, err)
	}
	return a
}

func liveSessionsAtSchool(t *testing.T, db *bun.DB, accountID int64) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE account_id = ? AND tenant_id = ?`,
		accountID, testpkg.Tenant(t)).Scan(context.Background(), &count))
	return count
}

// waitForLockWaiters blocks until n backends of this isolated database wait
// for a lock.
func waitForLockWaiters(t *testing.T, db *bun.DB, n int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(context.Background(), &waiting)
		return err == nil && waiting >= n
	}, 10*time.Second, 10*time.Millisecond)
}

func awaitResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(15 * time.Second):
		t.Fatal("the waiting side did not finish after the lock holder ended")
		return nil
	}
}

// A mint that holds the account lock commits its session before the
// replacement reads the sessions to revoke: the replacement waits, then ends
// it with the others.
func TestReplaceAccountRoleRevokesASessionMintedBeforeItsLock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := setupAuthService(t, db)
	rbac := roleAdministrationOf(t, service)
	a := newFenceAccount(t, db, service, "fence-mint-first")
	ctx := context.Background()

	mint, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = mint.Rollback() }()
	_, err = mint.ExecContext(ctx, `SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, a.accountID)
	require.NoError(t, err)
	_, err = mint.ExecContext(ctx, `INSERT INTO auth.tokens (account_id, tenant_id, token, expiry, family_id, generation, portal_scope)
		VALUES (?, ?, ?, NOW() + INTERVAL '1 hour', ?, 0, 'tenant')`,
		a.accountID, testpkg.Tenant(t), sha256Hex(fmt.Sprintf("fence-minted-%d", a.accountID)), fmt.Sprintf("fence-family-%d", a.accountID))
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- rbac.ReplaceAccountRole(testpkg.Ctx(t), a.accountID, a.target) }()
	waitForLockWaiters(t, db, 1)
	require.NoError(t, mint.Commit())

	require.NoError(t, awaitResult(t, done))
	require.Zero(t, liveSessionsAtSchool(t, db, a.accountID), "the session minted before the replacement's lock is revoked")
}

// A renewal that arrives while the replacement holds the account lock waits
// for it and then finds its session revoked: it mints no successor.
func TestReplaceAccountRoleRefusesARenewalWaitingForItsLock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := setupAuthService(t, db)
	rbac := roleAdministrationOf(t, service)
	a := newFenceAccount(t, db, service, "fence-revoke-first")
	ctx := context.Background()
	_, refresh, err := service.Login(testpkg.Ctx(t), a.email, testPassword)
	require.NoError(t, err)
	require.Equal(t, 1, liveSessionsAtSchool(t, db, a.accountID))

	// Holding the second role's assignment keeps the replacement inside its
	// transaction, after it took the account lock.
	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	_, err = holder.ExecContext(ctx, `SELECT 1 FROM auth.account_roles WHERE account_id = ? AND role_id = ? FOR UPDATE`, a.accountID, a.second)
	require.NoError(t, err)

	replaced := make(chan error, 1)
	go func() { replaced <- rbac.ReplaceAccountRole(testpkg.Ctx(t), a.accountID, a.target) }()
	waitForLockWaiters(t, db, 1)
	renewed := make(chan error, 1)
	go func() {
		_, _, refreshErr := service.RefreshToken(testpkg.Ctx(t), refresh)
		renewed <- refreshErr
	}()
	waitForLockWaiters(t, db, 2)
	require.NoError(t, holder.Commit())

	require.NoError(t, awaitResult(t, replaced))
	require.Error(t, awaitResult(t, renewed), "the waiting renewal finds its session revoked")
	require.Zero(t, liveSessionsAtSchool(t, db, a.accountID), "no successor outlives the replacement")
}
