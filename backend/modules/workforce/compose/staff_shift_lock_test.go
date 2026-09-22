package compose

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStaffShiftLockRequiresTenant(t *testing.T) {
	t.Parallel()

	err := NewStaffShiftLock(&bun.DB{})(context.Background(), 7)
	require.Error(t, err)
	assert.EqualError(t, err, "tenant id is required")
}

func TestStaffShiftLockWrapsAcquireError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupClosableTestDB(t)
	require.NoError(t, db.Close())

	ctx := testpkg.Ctx(t)
	err := NewStaffShiftLock(db)(ctx, 7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lock staff shift writes")
}

func TestStaffShiftLockTakesAdvisoryLock(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	err := testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		return NewStaffShiftLock(db)(ctx, 7)
	})
	require.NoError(t, err)
}
