package schedule

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestLockStaffShiftWritesRequiresTenant(t *testing.T) {
	err := LockStaffShiftWrites(context.Background(), &bun.DB{}, 7)
	require.Error(t, err)
	assert.EqualError(t, err, "tenant id is required")
}

func TestLockStaffShiftWritesWrapsAcquireError(t *testing.T) {
	db := testpkg.SetupClosableTestDB(t)
	require.NoError(t, db.Close())

	ctx := tenant.WithTenantID(context.Background(), 1)
	err := LockStaffShiftWrites(ctx, db, 7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lock staff shift writes")
}

func TestLockStaffShiftWritesTakesAdvisoryLock(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	err := tenant.WithTenantTx(context.Background(), db, 1, func(ctx context.Context, _ bun.Tx) error {
		return LockStaffShiftWrites(ctx, db, 7)
	})
	require.NoError(t, err)
}
