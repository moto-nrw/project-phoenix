package peopledirectory_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func newCaregiverBindingLocker(db *bun.DB) testpkg.CaregiverBindingLocker {
	return testutil.NewPeopleRepositorySuiteCaregiverBindingLocker(db)
}

func TestStaffRepository_ReleasesCaregiverBindingLocksOnRollback(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	holderRepo := newCaregiverBindingLocker(db)

	holder, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	runtimeCtx := testpkg.ContextWithTenantRuntime(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))
	holderCtx := testpkg.ContextWithTransaction(runtimeCtx, &holder)
	require.NoError(t, holderRepo.LockCaregiverCapabilityBindings(holderCtx))

	waiter, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	waiterCtx, cancel := context.WithTimeout(testpkg.ContextWithTransaction(runtimeCtx, &waiter), 2*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- holderRepo.LockCaregiverCapabilityBindings(waiterCtx) }()

	select {
	case lockErr := <-result:
		t.Fatalf("competing table lock returned before rollback: %v", lockErr)
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(t, holder.Rollback())
	select {
	case lockErr := <-result:
		require.NoError(t, lockErr)
	case <-time.After(2 * time.Second):
		t.Fatal("competing table lock was not released by rollback")
	}
	require.NoError(t, waiter.Rollback())
}

func TestStaffRepository_ReturnsCaregiverBindingLockFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	repo := newCaregiverBindingLocker(db)
	require.NoError(t, db.Close())

	err := repo.LockCaregiverCapabilityBindings(testpkg.Ctx(t))
	require.Error(t, err)
}
