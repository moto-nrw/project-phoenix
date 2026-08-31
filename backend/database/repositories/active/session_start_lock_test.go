package active_test

import (
	"context"
	"testing"
	"time"

	repoActive "github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSessionStartLocker_AcquiresTransactionLock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	locker := repoActive.NewSessionStartLocker(db)
	activity := testpkg.CreateTestActivityGroup(t, db, "session-start-lock")

	err := testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
		return locker.LockSessionStart(ctx, testpkg.Tenant(t), activity.ID)
	})
	require.NoError(t, err)
}

func TestSessionStartLocker_ReleasesLockOnRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	locker := repoActive.NewSessionStartLocker(db)
	tenantID, activityID := testpkg.Tenant(t), int64(73)

	holder, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	runtimeCtx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	holderCtx := tenant.WithTransactionForTest(runtimeCtx, &holder)
	require.NoError(t, locker.LockSessionStart(holderCtx, tenantID, activityID))

	waiter, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	waiterCtx, cancel := context.WithTimeout(tenant.WithTransactionForTest(runtimeCtx, &waiter), 2*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- locker.LockSessionStart(waiterCtx, tenantID, activityID) }()

	select {
	case lockErr := <-result:
		t.Fatalf("competing lock returned before rollback: %v", lockErr)
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(t, holder.Rollback())
	select {
	case lockErr := <-result:
		require.NoError(t, lockErr)
	case <-time.After(2 * time.Second):
		t.Fatal("competing lock was not released by rollback")
	}
	require.NoError(t, waiter.Rollback())
}

func TestSessionStartLocker_ReturnsDatabaseFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	locker := repoActive.NewSessionStartLocker(db)
	activity := testpkg.CreateTestActivityGroup(t, db, "session-start-lock-failure")
	require.NoError(t, db.Close())

	err := locker.LockSessionStart(testpkg.Ctx(t), testpkg.Tenant(t), activity.ID)
	require.Error(t, err)
}
