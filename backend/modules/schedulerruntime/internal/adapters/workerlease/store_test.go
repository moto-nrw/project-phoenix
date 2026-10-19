package workerlease

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

const leaseName = "worker"

// leaseStore binds the store the way the Serve root does: statements run as
// the least-privilege phoenix_auth login, which switches to the
// administrative or tenant role per transaction.
func leaseStore(t *testing.T) (*Store, *bun.DB) {
	t.Helper()
	testpkg.SetupIsolatedTestDB(t)
	authDB := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, authDB.Close()) })
	store, err := NewStore(testpkg.TenantRuntime(t, authDB))
	require.NoError(t, err)
	return store, authDB
}

// assertAs asserts term inside a job transaction of the given role.
func assertAs(t *testing.T, db *bun.DB, role string, store *Store, term Term, margin time.Duration) error {
	t.Helper()
	assert := func(ctx context.Context) error { return store.Assert(ctx, term, margin) }
	if role == "phoenix_admin" {
		return testpkg.WithinAdminContext(t, context.Background(), db, assert)
	}
	return testpkg.WithinTenantContext(t, context.Background(), db, testpkg.Tenant(t), assert)
}

func withRole(ctx context.Context, db *bun.DB, role string, fn func(context.Context, bun.IDB) error) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SET LOCAL ROLE "+role); err != nil {
			return err
		}
		return fn(ctx, tx)
	})
}

func databaseNow(t *testing.T, db *bun.DB) time.Time {
	t.Helper()
	var now time.Time
	require.NoError(t, db.NewRaw("SELECT clock_timestamp()").Scan(context.Background(), &now))
	return now
}

func TestAcquireUsesDatabaseTimeAndRejectsASecondHolder(t *testing.T) {
	t.Parallel()
	store, db := leaseStore(t)
	ctx := context.Background()

	before := databaseNow(t, db)
	term, acquired, err := store.Acquire(ctx, leaseName, "worker-a", time.Minute)
	after := databaseNow(t, db)
	require.NoError(t, err)
	require.True(t, acquired)

	assert.EqualValues(t, 1, term.Token, "the first holder starts the token sequence")
	assert.Equal(t, "worker-a", term.Holder)
	assert.False(t, term.Until.Before(before.Add(time.Minute)), "the expiry is database time plus TTL, whatever the process clock says")
	assert.False(t, term.Until.After(after.Add(time.Minute)))

	_, acquired, err = store.Acquire(ctx, leaseName, "worker-b", time.Minute)
	require.NoError(t, err)
	assert.False(t, acquired, "a running term blocks every other holder")
}

func TestConcurrentAcquireGrantsExactlyOneTerm(t *testing.T) {
	t.Parallel()
	store, _ := leaseStore(t)

	const contenders = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	terms := make(chan Term, contenders)
	errs := make(chan error, contenders)
	for i := range contenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			term, acquired, err := store.Acquire(context.Background(), leaseName, fmt.Sprintf("worker-%d", i), time.Minute)
			if err != nil {
				errs <- err
				return
			}
			if acquired {
				terms <- term
			}
		}()
	}
	close(start)
	wg.Wait()
	close(terms)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	var granted []Term
	for term := range terms {
		granted = append(granted, term)
	}
	require.Len(t, granted, 1, "exactly one contender may lead")
	assert.EqualValues(t, 1, granted[0].Token)
}

func TestRenewExtendsOnlyTheRunningTerm(t *testing.T) {
	t.Parallel()
	store, _ := leaseStore(t)
	ctx := context.Background()
	term, acquired, err := store.Acquire(ctx, leaseName, "worker-a", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)

	renewed, ok, err := store.Renew(ctx, term, 2*time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, term.Token, renewed.Token)
	assert.True(t, renewed.Until.After(term.Until))

	stale := term
	stale.Token++
	_, ok, err = store.Renew(ctx, stale, time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "a token that never held the lease cannot renew it")

	other := term
	other.Holder = "worker-b"
	_, ok, err = store.Renew(ctx, other, time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "another holder cannot renew the term")
}

func TestExpiredLeasePassesToTheStandbyWithALargerToken(t *testing.T) {
	t.Parallel()
	store, db := leaseStore(t)
	ctx := context.Background()
	first, acquired, err := store.Acquire(ctx, leaseName, "worker-a", 300*time.Millisecond)
	require.NoError(t, err)
	require.True(t, acquired)

	require.Eventually(t, func() bool {
		return !databaseNow(t, db).Before(first.Until)
	}, 5*time.Second, 20*time.Millisecond)

	second, acquired, err := store.Acquire(ctx, leaseName, "worker-b", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired, "an expired lease passes to the standby")
	assert.Greater(t, second.Token, first.Token)

	_, ok, err := store.Renew(ctx, first, time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "the crashed holder cannot resume its old term")
}

func TestReleaseHandsTheLeaseOverAtOnce(t *testing.T) {
	t.Parallel()
	store, _ := leaseStore(t)
	ctx := context.Background()
	first, acquired, err := store.Acquire(ctx, leaseName, "worker-a", time.Hour)
	require.NoError(t, err)
	require.True(t, acquired)

	require.NoError(t, store.Release(ctx, first))
	second, acquired, err := store.Acquire(ctx, leaseName, "worker-b", time.Hour)
	require.NoError(t, err)
	require.True(t, acquired)
	assert.Equal(t, first.Token+1, second.Token)

	require.NoError(t, store.Release(ctx, first), "releasing an ended term changes nothing")
	_, acquired, err = store.Acquire(ctx, leaseName, "worker-c", time.Hour)
	require.NoError(t, err)
	assert.False(t, acquired, "the stale release left the new term running")
}

func TestReacquireByTheSameHolderStartsANewTerm(t *testing.T) {
	t.Parallel()
	store, _ := leaseStore(t)
	ctx := context.Background()
	first, acquired, err := store.Acquire(ctx, leaseName, "worker-a", time.Hour)
	require.NoError(t, err)
	require.True(t, acquired)

	second, acquired, err := store.Acquire(ctx, leaseName, "worker-a", time.Hour)
	require.NoError(t, err)
	require.True(t, acquired)
	assert.Equal(t, first.Token+1, second.Token, "a holder that lost track of its term starts a fenced new one")

	_, ok, err := store.Renew(ctx, first, time.Hour)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestTakeoverWaitsForACommitThePredecessorAlreadyConfirmed(t *testing.T) {
	t.Parallel()
	store, db := leaseStore(t)
	ctx := context.Background()
	first, acquired, err := store.Acquire(ctx, leaseName, "worker-a", 500*time.Millisecond)
	require.NoError(t, err)
	require.True(t, acquired)

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(ctx, "SET LOCAL ROLE phoenix_tenant")
	require.NoError(t, err)
	require.NoError(t, store.Assert(testpkg.ContextWithTransaction(ctx, tx), first, 0), "the job confirms its term as its last statement")

	require.Eventually(t, func() bool {
		return !databaseNow(t, db).Before(first.Until)
	}, 5*time.Second, 20*time.Millisecond, "the term expires while the confirmed commit is still pending")

	type result struct {
		term     Term
		acquired bool
		err      error
	}
	takeover := make(chan result, 1)
	go func() {
		term, acquired, err := store.Acquire(ctx, leaseName, "worker-b", time.Hour)
		takeover <- result{term, acquired, err}
	}()
	select {
	case <-takeover:
		t.Fatal("the standby took over before the predecessor's confirmed commit finished")
	case <-time.After(300 * time.Millisecond):
	}

	require.NoError(t, tx.Commit())
	select {
	case got := <-takeover:
		require.NoError(t, got.err)
		require.True(t, got.acquired)
		assert.Greater(t, got.term.Token, first.Token)
	case <-time.After(5 * time.Second):
		t.Fatal("the takeover did not proceed after the commit")
	}
}

func TestAssertFencesStaleTermsInsideTenantAndAdminTransactions(t *testing.T) {
	t.Parallel()
	store, db := leaseStore(t)
	ctx := context.Background()
	first, acquired, err := store.Acquire(ctx, leaseName, "worker-a", time.Hour)
	require.NoError(t, err)
	require.True(t, acquired)

	for _, role := range []string{"phoenix_tenant", "phoenix_admin"} {
		require.NoError(t, assertAs(t, db, role, store, first, time.Second), "the running term may commit as %s", role)
	}

	err = assertAs(t, db, "phoenix_tenant", store, first, 2*time.Hour)
	require.ErrorIs(t, err, ErrNotHeld, "a term ending within the margin may not commit")

	require.NoError(t, store.Release(ctx, first))
	second, acquired, err := store.Acquire(ctx, leaseName, "worker-b", time.Hour)
	require.NoError(t, err)
	require.True(t, acquired)

	for _, role := range []string{"phoenix_tenant", "phoenix_admin"} {
		err := assertAs(t, db, role, store, first, 0)
		require.ErrorIs(t, err, ErrNotHeld, "a stale token is rejected at finalize as %s", role)
	}
	require.NoError(t, assertAs(t, db, "phoenix_tenant", store, second, 0))
}

func TestTenantRoleCannotReachTheLeaseTable(t *testing.T) {
	t.Parallel()
	store, db := leaseStore(t)
	ctx := context.Background()
	_, acquired, err := store.Acquire(ctx, leaseName, "worker-a", time.Hour)
	require.NoError(t, err)
	require.True(t, acquired)

	statements := []string{
		"SELECT holder_id FROM platform.worker_leases",
		"UPDATE platform.worker_leases SET lease_until = clock_timestamp()",
		"INSERT INTO platform.worker_leases (lease_name, holder_id, fencing_token, lease_until) VALUES ('other', 'x', 1, clock_timestamp())",
		"DELETE FROM platform.worker_leases",
	}
	for _, statement := range statements {
		err := withRole(ctx, db, "phoenix_tenant", func(ctx context.Context, tx bun.IDB) error {
			_, err := tx.ExecContext(ctx, statement)
			return err
		})
		require.ErrorContains(t, err, "permission denied", statement)
	}

	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, "SELECT holder_id FROM platform.worker_leases")
		return err
	})
	require.ErrorContains(t, err, "permission denied", "the login role reaches the lease only through the administrative role")

	var holder string
	err = withRole(ctx, db, "phoenix_admin", func(ctx context.Context, tx bun.IDB) error {
		return tx.NewRaw("SELECT holder_id FROM platform.worker_leases WHERE lease_name = ?", leaseName).Scan(ctx, &holder)
	})
	require.NoError(t, err)
	assert.Equal(t, "worker-a", holder)
}
