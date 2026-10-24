package compose

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStaffWorkScheduleReplaceSharesBalanceLock(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	staff := testpkg.CreateTestStaff(t, db, "Schedule", "BalanceLock")

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
		require.FailNow(t, "adjustment writer did not acquire the balance lock")
	}

	writerDone := make(chan error, 1)
	go func() {
		writerDone <- testpkg.WithinTenantContext(t, context.Background(), db, staff.TenantID, func(ctx context.Context) error {
			return capability.ReplaceStaffSchedule(ctx, workforce.ReplaceStaffSchedule{
				StaffID: staff.ID,
				Entries: []workforce.StaffWorkScheduleEntry{{
					WeekIndex:      0,
					RotationLength: 1,
					DayOfWeek:      dayMonday,
					TargetMinutes:  480,
				}},
			})
		})
	}()

	select {
	case err := <-writerDone:
		close(releaseLock)
		require.NoError(t, <-holderDone)
		require.Failf(t, "schedule writer bypassed shared balance lock", "returned early: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseLock)
	require.NoError(t, <-holderDone)
	require.NoError(t, <-writerDone)
}
