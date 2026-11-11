package compose_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExclusiveWorkflowHoldsNoTransactionAndReleasesItsSession(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	db.SetMaxOpenConns(4)
	unit, err := compose.NewTenantRuntime(db)
	require.NoError(t, err)
	other, err := compose.NewTenantRuntime(db)
	require.NoError(t, err)
	ctx := tenant.WithUnitOfWork(t.Context(), unit)
	key := fmt.Sprintf("exclusive-workflow-test:%d", testpkg.Tenant(t))
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	var releaseOnce sync.Once
	releaseRun := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseRun()
	go func() {
		done <- tenant.WithinExclusive(ctx, key, func(ctx context.Context) error {
			if _, present := tenant.TransactionFromContext(ctx); present {
				return errors.New("mutex opened a transaction")
			}
			id, idErr := tenant.NewTenantID(testpkg.Tenant(t))
			if idErr != nil {
				return idErr
			}
			if txErr := tenant.WithinTenant(ctx, id, func(context.Context) error { return nil }); txErr != nil {
				return txErr
			}
			close(entered)
			<-release
			return nil
		})
	}()
	select {
	case <-entered:
	case err := <-done:
		require.NoError(t, err)
		t.Fatal("mutex callback did not run")
	case <-time.After(5 * time.Second):
		t.Fatal("mutex acquisition timed out")
	}
	probe, err := db.Conn(t.Context())
	require.NoError(t, err)
	defer func() { require.NoError(t, probe.Close()) }()
	var acquired bool
	require.NoError(t, probe.NewRaw("SELECT pg_try_advisory_lock(hashtextextended(?,0))", key).Scan(t.Context(), &acquired))
	assert.False(t, acquired, "a different connection cannot enter the workflow")
	waiting, cancel := context.WithCancel(tenant.WithUnitOfWork(t.Context(), other))
	defer cancel()
	blocked := make(chan error, 1)
	go func() {
		blocked <- tenant.WithinExclusive(waiting, key, func(context.Context) error { return errors.New("overlapping callback ran") })
	}()
	require.Eventually(t, func() bool { return db.Stats().InUse == 3 },
		5*time.Second, 10*time.Millisecond, "the other runtime holds a connection while waiting for the mutex")
	cancel()
	require.ErrorIs(t, <-blocked, context.Canceled)
	releaseRun()
	require.NoError(t, <-done)
	require.NoError(t, probe.NewRaw("SELECT pg_try_advisory_lock(hashtextextended(?,0))", key).Scan(t.Context(), &acquired))
	require.True(t, acquired, "success and cancellation leave no stranded session lock")
	_, err = probe.ExecContext(t.Context(), "SELECT pg_advisory_unlock(hashtextextended(?,0))", key)
	require.NoError(t, err)
	sentinel := errors.New("callback failed")
	require.ErrorIs(t, tenant.WithinExclusive(ctx, key, func(context.Context) error { return sentinel }), sentinel)
	require.NoError(t, probe.NewRaw("SELECT pg_try_advisory_lock(hashtextextended(?,0))", key).Scan(t.Context(), &acquired))
	require.True(t, acquired, "callback failure releases the lock")
	_, err = probe.ExecContext(t.Context(), "SELECT pg_advisory_unlock(hashtextextended(?,0))", key)
	require.NoError(t, err)
}
