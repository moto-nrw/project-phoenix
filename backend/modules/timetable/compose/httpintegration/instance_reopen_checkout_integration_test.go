package httpintegration_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestInstance_Reopen_SerializesSchoolCheckoutThroughRestoration(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s := buildLifecycle(t)
	ai := seedInstanceOn(t, s, calendar.TodayDate(), true, true)
	started, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	checkIn := time.Now().Add(-10 * time.Minute)
	device := testpkg.CreateTestDevice(t, s.db, "reopen-concurrent-checkout")
	testpkg.CreateTestAttendanceForDate(t, s.db, s.student1, s.staffID, device.ID, calendar.Date(ai.Date), checkIn, nil)
	visit := testpkg.CreateTestVisit(t, s.db, s.student1, started.ActiveGroupID, checkIn, nil)
	_, err = s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)
	mode, err := testpkg.TenantQuery0(t, s.ctx, s.db, s.active.GetPresenceMode)
	require.NoError(t, err)
	require.Equal(t, "detailed", mode)

	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	defer cancel()
	checkoutDone := make(chan error, 1)
	var checkoutFinished bool
	var checkoutErr error
	err = testpkg.WithTenantTx(t, ctx, s.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		var pid int
		if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid); err != nil {
			return err
		}
		if _, err := s.svc.Reopen(txCtx, ai.ID, 0, true); err != nil {
			return err
		}
		// Keep restoration uncommitted. Checkout must wait for its attendance
		// guard, then see and close the restored visit after the commit.
		go func() {
			_, err := s.active.CheckOutStudent(ctx, s.student1, s.staffID, true)
			checkoutDone <- err
		}()
		assert.Eventually(t, func() bool {
			select {
			case checkoutErr = <-checkoutDone:
				checkoutFinished = true
				return true
			default:
			}
			var blocked bool
			err := s.db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname = current_database() AND ? = ANY(pg_blocking_pids(pid)))", pid).Scan(ctx, &blocked)
			return err == nil && blocked
		}, 5*time.Second, 10*time.Millisecond, "checkout must reach the attendance lock")
		assert.False(t, checkoutFinished, "school checkout must not commit before restoration")
		return nil
	})
	require.NoError(t, err)
	if !checkoutFinished {
		select {
		case checkoutErr = <-checkoutDone:
		case <-ctx.Done():
			t.Fatal("checkout did not finish after restoration committed")
		}
	}
	require.NoError(t, checkoutErr)
	stays, err := s.presence.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{s.student1}, OpenOnly: true})
	require.NoError(t, err)
	assert.Empty(t, stays)
	assert.NotNil(t, testpkg.VisitExitTime(t, s.db, visit.ID), "a checked-out child must not retain a restored visit")
}
