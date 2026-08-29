package schedule

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestLockStaffShiftWritesRequiresTenant(t *testing.T) {
	t.Parallel()

	err := LockStaffShiftWrites(context.Background(), &bun.DB{}, 7)
	require.Error(t, err)
	assert.EqualError(t, err, "tenant id is required")
}

func TestLockStaffShiftWritesWrapsAcquireError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupClosableTestDB(t)
	require.NoError(t, db.Close())

	ctx := testpkg.Ctx(t)
	err := LockStaffShiftWrites(ctx, db, 7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lock staff shift writes")
}

func TestLockStaffShiftWritesTakesAdvisoryLock(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	err := testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		return LockStaffShiftWrites(ctx, db, 7)
	})
	require.NoError(t, err)
}
