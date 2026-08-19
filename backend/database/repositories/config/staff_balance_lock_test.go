package config_test

import (
	"context"
	"testing"
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStaffWorkScheduleReplaceSharesBalanceLock(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	staff := testpkg.CreateTestStaff(t, db, "Schedule", "BalanceLock")
	t.Cleanup(func() {
		cleanupStaffWorkSchedules(t, db, staff.ID)
		testpkg.CleanupStaffFixtures(t, db, staff.ID)
	})

	adjustments := activeRepo.NewStaffBalanceAdjustmentRepository(db)
	schedules := configRepo.NewStaffWorkScheduleRepository(db)

	lockHeld := make(chan struct{})
	releaseLock := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- tenant.WithTenantTx(context.Background(), db, staff.TenantID, func(ctx context.Context, _ bun.Tx) error {
			if err := adjustments.LockStaffBalanceWrites(ctx, staff.ID); err != nil {
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
		writerDone <- tenant.WithTenantTx(context.Background(), db, staff.TenantID, func(ctx context.Context, _ bun.Tx) error {
			return schedules.ReplaceSchedule(ctx, staff.ID, []*configModel.StaffWorkSchedule{{
				WeekIndex:      0,
				RotationLength: 1,
				DayOfWeek:      configModel.DayMonday,
				TargetMinutes:  480,
			}}, timezone.Date{})
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
