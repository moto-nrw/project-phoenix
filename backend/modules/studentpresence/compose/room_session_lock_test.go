package compose_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestRoomSessionLocker_AcquiresTransactionLock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	locker, composeErr := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, composeErr)
	activity := testpkg.CreateTestRoom(t, db, "room-session-lock")

	err := testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
		return locker.LockRoomSessionWrites(ctx, activity.ID)
	})
	require.NoError(t, err)
}

func TestRoomSessionLocker_ReleasesLockOnRollback(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	locker, composeErr := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, composeErr)
	activity := testpkg.CreateTestRoom(t, db, "room-session-lock-rollback")
	activityID := activity.ID

	holder, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	runtimeCtx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))
	holderCtx := tenant.WithTransactionForTest(runtimeCtx, &holder)
	// Contend with the exact key used by the legacy room-session writer.
	_, err = holder.ExecContext(holderCtx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("active-room-session:%d:%d", testpkg.Tenant(t), activityID))
	require.NoError(t, err)

	waiter, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	waiterCtx, cancel := context.WithTimeout(tenant.WithTransactionForTest(runtimeCtx, &waiter), 2*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- locker.LockRoomSessionWrites(waiterCtx, activityID) }()

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

func TestRoomSessionLocker_ReturnsDatabaseFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	locker, composeErr := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, composeErr)
	activity := testpkg.CreateTestRoom(t, db, "room-session-lock-failure")
	tx, err := db.BeginTx(testpkg.Ctx(t), nil)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	ctx := tenant.WithTransactionForTest(tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db)), &tx)

	err = locker.LockRoomSessionWrites(ctx, activity.ID)
	require.Error(t, err)
}

func TestRoomSessionLocker_RequiresTenantTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	locker, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	require.Error(t, locker.LockRoomSessionWrites(context.Background(), 1))
	require.ErrorContains(t, locker.LockRoomSessionWrites(tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db)), 1), "transaction is required")
	require.Error(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
		return locker.LockRoomSessionWrites(ctx, 0)
	}))
}
