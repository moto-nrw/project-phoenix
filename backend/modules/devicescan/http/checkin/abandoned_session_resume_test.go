package checkin_test

import (
	"context"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The abandoned-session sweep selects inactive sessions of offline devices
// without a lock. Between that selection and the session lock the device can
// come back: a committed scan, a heartbeat or a manual end. The sweep must
// judge the session again under its lock and keep a resumed session, its
// visits and its timetable mirror open (#6502).
func TestAbandonedSessionCleanupRechecksTheLockedSession(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, scenario := range []struct {
		name string
		// deviceContact commits a fresh heartbeat of the session's device
		// while the sweep waits for the session lock; scan queues a real kiosk
		// scan on the session ahead of the sweep.
		deviceContact, scan, ends bool
	}{
		{name: "unchanged_orphan", ends: true},
		{name: "committed_scan", scan: true},
		{name: "device_contact", deviceContact: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			f := newAbandonedSweepFixture(t)
			holder := holdRow(t, f.k.db, "SELECT id FROM active.groups WHERE id = ? FOR UPDATE", f.scanStay)
			var scanned <-chan *httptest.ResponseRecorder
			var swept <-chan error
			var count *int
			if scenario.scan {
				// The scan queues on the session first, so it commits its
				// activity before the sweep gets the lock.
				scanned = startLockOrderScan(t, f.k, f.tag, f.scanRoom, f.scanDevice, f.scanStaff)
				scanPID := awaitWaiterOf(t, f.k.db, holder.pid)
				swept, count = f.sweep(t)
				awaitWaiterOf(t, f.k.db, scanPID)
			} else {
				swept, count = f.sweep(t)
				awaitWaiterOf(t, f.k.db, holder.pid)
			}
			if scenario.deviceContact {
				_, err := f.k.db.NewRaw("UPDATE iot.devices SET last_seen = NOW() WHERE tenant_id = ? AND id = ?", testpkg.Tenant(t), f.device).Exec(testpkg.Ctx(t))
				require.NoError(t, err)
			}
			holder.commit(t)
			if scanned != nil {
				testutil.AssertSuccessResponse(t, awaitScan(t, scanned), 200)
			}
			require.NoError(t, awaitReattach(t, swept))
			if scenario.ends {
				require.Equal(t, 1, *count)
				f.assertEnded(t)
				return
			}
			require.Zero(t, *count, "a resumed session is no longer abandoned")
			f.assertOpen(t)
		})
	}
}

// A manual end that commits between selection and lock leaves the sweep an
// expected skip: it neither fails nor counts the session.
func TestAbandonedSessionCleanupSkipsAConcurrentManualEnd(t *testing.T) {
	t.Parallel()
	f := newAbandonedSweepFixture(t)
	holder := holdRow(t, f.k.db, "SELECT id FROM active.visits WHERE id = ? FOR UPDATE", f.held)
	workflow := api.NewActiveTestSessionEnd(testpkg.NewDatabase(t, f.k.db), testpkg.TenantRuntime(t, f.k.db)).SessionEnd
	ended := startLockOrderCommand(t, func(ctx context.Context) error {
		return testpkg.WithTenantTx(t, ctx, f.k.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			_, err := workflow.EndSession(txCtx, f.scanStay)
			return err
		})
	})
	endPID := awaitWaiterOf(t, f.k.db, holder.pid)
	swept, count := f.sweep(t)
	awaitWaiterOf(t, f.k.db, endPID)
	holder.commit(t)
	require.NoError(t, awaitReattach(t, ended))
	require.NoError(t, awaitReattach(t, swept))
	require.Zero(t, *count, "the manual end owns this session end")
	f.assertEnded(t)
}

type abandonedSweepFixture struct {
	bulkMoveScanFixture
	held     int64
	device   int64
	instance int64
}

// newAbandonedSweepFixture builds the kiosk session of an offline device that
// has been idle for two hours. It holds two open visits and mirrors a running
// timetable instance.
func newAbandonedSweepFixture(t *testing.T) abandonedSweepFixture {
	t.Helper()
	base, held := newSessionEndScanFixture(t)
	f := abandonedSweepFixture{bulkMoveScanFixture: base, held: held}
	ctx := testpkg.Ctx(t)
	require.NoError(t, f.k.db.NewRaw("SELECT device_id FROM active.groups WHERE tenant_id = ? AND id = ?", testpkg.Tenant(t), f.scanStay).Scan(ctx, &f.device))
	_, err := f.k.db.NewRaw("UPDATE iot.devices SET last_seen = NOW() - INTERVAL '2 hours' WHERE tenant_id = ? AND id = ?", testpkg.Tenant(t), f.device).Exec(ctx)
	require.NoError(t, err)
	f.instance = testpkg.CreateTestActivityInstance(t, f.k.db, testpkg.TodayDate(), f.scanRoom, testpkg.ActivityInstanceOpts{
		IsSpontaneous: true, StartHHMM: "00:00", EndHHMM: "23:59", ActiveGroupID: &f.scanStay,
	}).ID
	return f
}

// sweep runs the abandoned-session cleanup in one tenant unit, as the
// scheduler and the cleanup CLI do.
func (f abandonedSweepFixture) sweep(t *testing.T) (<-chan error, *int) {
	t.Helper()
	count := new(int)
	return startLockOrderCommand(t, func(ctx context.Context) error {
		return testpkg.WithTenantTx(t, ctx, f.k.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			var err error
			*count, err = f.active.CleanupAbandonedSessions(txCtx, time.Hour)
			return err
		})
	}), count
}

func (f abandonedSweepFixture) assertOpen(t *testing.T) {
	t.Helper()
	group, err := f.active.GetActiveGroup(testpkg.Ctx(t), f.scanStay)
	require.NoError(t, err)
	require.Nil(t, group.EndTime, "the resumed session stays open")
	open, err := f.k.presence(t).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{ActiveGroupIDs: []int64{f.scanStay}, OpenOnly: true})
	require.NoError(t, err)
	require.NotEmpty(t, open, "the visits of a resumed session stay open")
	require.True(t, slices.ContainsFunc(open, func(v studentpresence.Visit) bool { return v.ID == f.held }), "the untouched visit stays open")
	require.Equal(t, testpkg.ScheduleInstanceStatusActive, testpkg.InstanceStatus(t, f.k.db, f.instance), "the timetable mirror keeps running")
}

func (f abandonedSweepFixture) assertEnded(t *testing.T) {
	t.Helper()
	group, err := f.active.GetActiveGroup(testpkg.Ctx(t), f.scanStay)
	require.NoError(t, err)
	require.NotNil(t, group.EndTime)
	open, err := f.k.presence(t).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{ActiveGroupIDs: []int64{f.scanStay}, OpenOnly: true})
	require.NoError(t, err)
	require.Empty(t, open)
	require.Equal(t, testpkg.ScheduleInstanceStatusCompleted, testpkg.InstanceStatus(t, f.k.db, f.instance), "the timetable mirror ends with the session")
}
