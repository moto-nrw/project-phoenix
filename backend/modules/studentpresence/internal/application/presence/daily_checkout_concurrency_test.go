package presence_test

import (
	"context"
	"sync"
	"testing"
	"time"

	activeService "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestConfirmDailyCheckout_ConcurrentDestinationsReportRecordedCheckout(t *testing.T) {
	t.Parallel()
	for _, firstDestination := range []string{"zuhause", "unterricht"} {
		t.Run(firstDestination, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupClosableTestDB(t)
			t.Cleanup(func() { assert.NoError(t, db.Close()) })
			svc := setupActiveService(t, db)
			ctx := testpkg.Ctx(t)
			student := testpkg.CreateTestStudent(t, db, "Checkout", "Concurrent", "2a")
			staff := testpkg.CreateTestStaff(t, db, "Checkout", "Concurrent")
			device := testpkg.CreateTestDevice(t, db, "concurrent-checkout")
			testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, time.Now(), nil)

			// Keep the first checkout uncommitted. The other confirmation sees
			// the open attendance and then waits for its checkout lock.
			ready := make(chan int, 1)
			release := make(chan struct{})
			var releaseOnce sync.Once
			unlock := func() { releaseOnce.Do(func() { close(release) }) }
			done := make(chan error, 1)
			go func() {
				done <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
					var pid int
					if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid); err != nil {
						return err
					}
					if _, err := svc.ConfirmDailyCheckout(txCtx, student.ID, device.ID, firstDestination); err != nil {
						return err
					}
					ready <- pid
					<-release
					return nil
				})
			}()
			t.Cleanup(unlock)
			var pid int
			select {
			case pid = <-ready:
			case err := <-done:
				t.Fatalf("first checkout did not reach commit: %v", err)
			case <-time.After(10 * time.Second):
				t.Fatal("first checkout did not start")
			}

			secondDestination, expectedAction := "unterricht", "checked_out_daily"
			if firstDestination == "unterricht" {
				secondDestination, expectedAction = "zuhause", "checked_out_class"
			}
			secondDone := make(chan error, 1)
			var result *activeService.DailyCheckoutResult
			go func() {
				var err error
				result, err = svc.ConfirmDailyCheckout(ctx, student.ID, device.ID, secondDestination)
				secondDone <- err
			}()
			awaitBlockedOn(t, db, pid, 0)
			unlock()
			require.NoError(t, awaitResult(t, done))
			require.NoError(t, awaitResult(t, secondDone))
			require.NotNil(t, result)
			assert.Equal(t, expectedAction, result.Action)
			status, err := testpkg.TenantQuery1(t, ctx, db, svc.GetStudentAttendanceStatus, student.ID)
			require.NoError(t, err)
			assert.Equal(t, firstDestination == "unterricht", status.IsInClass())
		})
	}
}
