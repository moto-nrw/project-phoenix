package users_test

import (
	"context"
	"testing"
	"time"

	repoUsers "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Deliberately NOT parallel: this test holds a table lock while a competing
// transaction waits for it, which can deadlock alongside other lock tests.
func TestStaffRepository_ReleasesCaregiverBindingLocksOnRollback(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	holderRepo := repoUsers.NewCaregiverBindingLocker(db)

	holder, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	runtimeCtx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	holderCtx := tenant.WithTransactionForTest(runtimeCtx, &holder)
	require.NoError(t, holderRepo.LockCaregiverCapabilityBindings(holderCtx))

	waiter, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	waiterCtx, cancel := context.WithTimeout(tenant.WithTransactionForTest(runtimeCtx, &waiter), 2*time.Second)
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
	repo := repoUsers.NewCaregiverBindingLocker(db)
	require.NoError(t, db.Close())

	err := repo.LockCaregiverCapabilityBindings(testpkg.Ctx(t))
	require.Error(t, err)
}
