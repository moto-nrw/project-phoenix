package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestGroupSupervisionLocksRequireTransactionAndReleaseOnRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	staff := testpkg.CreateTestStaff(t, db, "Lock", "Supervisor")
	supervision := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	date := supervision.StartDate.String()
	filter := studentpresence.GroupSupervisionFilter{GroupIDs: []int64{group.ID}, ActiveOn: &date, ForUpdate: true}
	_, err = module.QueryGroupSupervisions(ctx, filter)
	require.ErrorContains(t, err, "transaction is required")
	holder, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	rows, err := module.QueryGroupSupervisions(tenant.WithTransactionForTest(ctx, &holder), filter)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, supervision.ID, rows[0].ID)
	waiter, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer func() { _ = waiter.Rollback() }()
	waitCtx, cancel := context.WithTimeout(tenant.WithTransactionForTest(ctx, &waiter), 3*time.Second)
	defer cancel()
	completed := make(chan error, 1)
	go func() {
		_, readErr := module.QueryGroupSupervisions(waitCtx, filter)
		completed <- readErr
	}()
	select {
	case err := <-completed:
		t.Fatalf("competing row lock returned before rollback: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, holder.Rollback())
	select {
	case err := <-completed:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("supervision lock was not released by rollback")
	}
}
