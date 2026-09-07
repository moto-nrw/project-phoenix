package compose

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// An effective absence changes the Stundenkonto, so the absence writer must
// queue behind whoever holds the shared staff balance lock (balance
// adjustments, work sessions, schedule rewrites) instead of interleaving.
func TestStaffAbsenceWriterSharesBalanceLock(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "Absence", "BalanceLock")

	runtime := testpkg.ConfigRuntime(db)
	capability := buildWorkforce(t, db)

	lockHeld := make(chan struct{})
	releaseLock := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- testpkg.WithinTenantContext(t, context.Background(), db, staff.TenantID, func(ctx context.Context) error {
			if err := runtime.LockStaffBalance(ctx, staff.ID); err != nil {
				return err
			}
			close(lockHeld)
			<-releaseLock
			return nil
		})
	}()

	select {
	case <-lockHeld:
	case <-time.After(5 * time.Second):
		close(releaseLock)
		require.FailNow(t, "balance writer did not acquire the balance lock")
	}

	writerDone := make(chan error, 1)
	go func() {
		writerDone <- testpkg.WithinTenantContext(t, context.Background(), db, staff.TenantID, func(ctx context.Context) error {
			return capability.LockStaffAbsenceWrites(ctx, staff.ID)
		})
	}()

	select {
	case err := <-writerDone:
		close(releaseLock)
		require.NoError(t, <-holderDone)
		require.Failf(t, "absence writer bypassed shared balance lock", "returned early: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseLock)
	require.NoError(t, <-holderDone)
	require.NoError(t, <-writerDone)
}
