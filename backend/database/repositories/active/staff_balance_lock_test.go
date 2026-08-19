package active

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStaffBalanceWritersShareAdvisoryLock(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	staff := testpkg.CreateTestStaff(t, db, "Balance", "Lock")
	t.Cleanup(func() { testpkg.CleanupStaffFixtures(t, db, staff.ID) })

	adjustments := NewStaffBalanceAdjustmentRepository(db)
	workSessions := NewWorkSessionRepository(db)
	absences := NewStaffAbsenceRepository(db)

	tests := []struct {
		name string
		lock func(context.Context) error
	}{
		{
			name: "work session writer",
			lock: func(ctx context.Context) error {
				return workSessions.LockStaffBalanceWrites(ctx, staff.ID)
			},
		},
		{
			name: "absence writer",
			lock: func(ctx context.Context) error {
				return absences.LockStaffAbsenceWrites(ctx, staff.ID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
					return tt.lock(ctx)
				})
			}()

			select {
			case err := <-writerDone:
				close(releaseLock)
				require.NoError(t, <-holderDone)
				require.Failf(t, "writer bypassed shared balance lock", "returned early: %v", err)
			case <-time.After(150 * time.Millisecond):
			}

			close(releaseLock)
			require.NoError(t, <-holderDone)
			require.NoError(t, <-writerDone)
		})
	}
}
