package integration

import (
	"context"
	"errors"
	"testing"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestRequestLocksRequireAmbientTransactionAndReleaseOnRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := enrollmentCompose.New()
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	ctx := testpkg.Ctx(t)
	for _, tc := range []struct {
		name          string
		first, second int32
		acquire       func(context.Context) error
	}{
		{"email", int32(phase.ID & 0x7fffffff), 123, func(ctx context.Context) error { return module.AcquireSubmissionDedupLock(ctx, phase.ID, 123) }},
		{"student", 0x656e726c, int32(phase.ID), func(ctx context.Context) error { return module.AcquireExistingStudentMatchLock(ctx, phase.ID) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorContains(t, tc.acquire(context.Background()), "transaction is required")
			failure := errors.New("abort submission")
			err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
				require.NoError(t, tc.acquire(txCtx))
				err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(otherCtx context.Context, tx bun.Tx) error {
					var acquired bool
					err := tx.NewRaw("SELECT pg_try_advisory_xact_lock(?, ?)", tc.first, tc.second).Scan(otherCtx, &acquired)
					require.NoError(t, err)
					require.False(t, acquired, "the command must retain its lock in the caller transaction")
					return nil
				})
				require.NoError(t, err)
				return failure
			})
			require.ErrorIs(t, err, failure)
			err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
				var acquired bool
				err := tx.NewRaw("SELECT pg_try_advisory_xact_lock(?, ?)", tc.first, tc.second).Scan(txCtx, &acquired)
				require.NoError(t, err)
				require.True(t, acquired, "rollback must release the lock for retry")
				return nil
			})
			require.NoError(t, err)
		})
	}
}
