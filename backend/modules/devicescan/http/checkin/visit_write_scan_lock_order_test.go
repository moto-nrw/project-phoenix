package checkin_test

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	checkinAPI "github.com/moto-nrw/project-phoenix/modules/devicescan/http/checkin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// An admin transfer holds the target session while its capacity check waits
// for the room. The concurrent scan must not form a visit/session cycle.
func TestAdminTransferAndScanDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := newBulkMoveScanFixture(t)
	visit, err := f.k.presence(t).FindVisit(testpkg.Ctx(t), f.visit)
	require.NoError(t, err)
	visit.ActiveGroupID = f.scanStay
	holder := holdRow(t, f.k.db, "SELECT id FROM facilities.rooms WHERE id = ? FOR UPDATE", f.scanRoom)
	operations := api.NewActiveTestPresence(testpkg.NewDatabase(t, f.k.db), testpkg.TenantRuntime(t, f.k.db)).PresenceOperations()
	transferred := startLockOrderCommand(t, func(ctx context.Context) error { return operations.AmendVisit(ctx, *visit) })
	transferPID := awaitWaiterOf(t, f.k.db, holder.pid)
	scan := startLockOrderScan(t, f.k, f.tag, f.scanRoom, f.scanDevice, f.scanStaff)
	awaitWaiterOf(t, f.k.db, transferPID)
	holder.commit(t)
	require.NoError(t, awaitReattach(t, transferred), "admin transfer must not deadlock against the scan")
	response := awaitScan(t, scan)
	testutil.AssertSuccessResponse(t, response, 200)
	require.Equal(t, "checked_out", responseData(t, response.Body.Bytes())["action"])
	open, err := f.k.presence(t).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{StudentIDs: []int64{f.student}, OpenOnly: true})
	require.NoError(t, err)
	require.Empty(t, open, "the scan sees the committed move into its room and checks the child out")
	var attendance int
	require.NoError(t, f.k.db.NewRaw("SELECT count(*) FROM active.attendance WHERE tenant_id = ? AND student_id = ? AND check_out_time IS NULL", testpkg.Tenant(t), f.student).Scan(testpkg.Ctx(t), &attendance))
	require.Equal(t, 1, attendance)
}

// A second visit, earlier in ID order, holds the lifecycle writer between
// its first and second visit. The real scan closes the second visit and then
// touches the same device session. This exposes session -> visit against
// visit -> session without relying on which waiter PostgreSQL chooses first.
func newSessionEndScanFixture(t *testing.T) (bulkMoveScanFixture, int64) {
	t.Helper()
	k := reattachKiosk(t)
	f := bulkMoveScanFixture{k: k}
	_, _, _, active, _, _, _, buildErr := api.NewTestTimetable(testpkg.NewDatabase(t, k.db), slog.Default())
	require.NoError(t, buildErr)
	f.active = active
	var device int64
	f.scanDevice, device = k.deviceRow(t, "lifecycle-scan-kiosk")
	f.scanStaff, _, _ = k.staff(t, "Lifecycle", "Scan")
	f.scanRoom, _, f.scanStay = k.roomWithSession(t, "Lifecycle Scan")
	testpkg.LinkDeviceToActiveGroup(t, k.db, f.scanStay, device)
	_, err := k.db.NewUpdate().Table("active.groups").Set("last_activity = NOW() - INTERVAL '2 hours'").Where("id = ?", f.scanStay).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	first := testpkg.CreateTestStudent(t, k.db, "First", "Visit", "3a")
	held := testpkg.CreateTestVisit(t, k.db, first.ID, f.scanStay, time.Now().Add(-time.Hour), nil).ID
	f.tag, f.student, _ = k.studentCard(t, "Scanned", "Visit", "3a")
	f.attendance = testpkg.CreateTestAttendance(t, k.db, f.student, 0, device, time.Now().Add(-2*time.Hour), nil).ID
	f.visit = testpkg.CreateTestVisit(t, k.db, f.student, f.scanStay, time.Now().Add(-time.Hour), nil).ID
	return f, held
}

type sessionEndScanCase struct {
	name string
	end  func(context.Context, *testing.T, bulkMoveScanFixture) error
}

func sessionEndScanCases() []sessionEndScanCase {
	return []sessionEndScanCase{
		{name: "timetable_start"},
		{name: "timetable_complete"},
		{name: "timetable_cancel"},
		{"active_group_end", func(ctx context.Context, _ *testing.T, f bulkMoveScanFixture) error {
			return f.active.EndActiveGroupSession(ctx, f.scanStay)
		}},
		{"presence_end", func(ctx context.Context, _ *testing.T, f bulkMoveScanFixture) error {
			return f.active.EndActivitySession(ctx, f.scanStay)
		}},
		{"timeout", func(ctx context.Context, t *testing.T, f bulkMoveScanFixture) error {
			var id int64
			if err := f.k.db.NewRaw("SELECT device_id FROM active.groups WHERE tenant_id = ? AND id = ?", testpkg.Tenant(t), f.scanStay).Scan(ctx, &id); err != nil {
				return err
			}
			_, err := f.active.ProcessSessionTimeout(ctx, id)
			return err
		}},
		{"scheduler_timeout", func(ctx context.Context, _ *testing.T, f bulkMoveScanFixture) error {
			_, err := f.active.CleanupAbandonedSessions(ctx, time.Minute)
			return err
		}},
		{"nightly_end", func(ctx context.Context, _ *testing.T, f bulkMoveScanFixture) error {
			_, err := f.active.EndDailySessions(ctx)
			return err
		}},
		{"workflow_end", func(ctx context.Context, t *testing.T, f bulkMoveScanFixture) error {
			workflow := api.NewActiveTestSessionEnd(testpkg.NewDatabase(t, f.k.db), testpkg.TenantRuntime(t, f.k.db)).SessionEnd
			_, err := workflow.EndSession(ctx, f.scanStay)
			return err
		}},
	}
}

func TestSessionEndPathsAndScanDoNotDeadlock(t *testing.T) {
	t.Parallel()
	for _, scenario := range sessionEndScanCases() {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			f, held := newSessionEndScanFixture(t)
			scenario := prepareTimetableScanCase(t, scenario, f)
			queries := testpkg.CaptureQueries(t, f.k.db)
			holder := holdRow(t, f.k.db, "SELECT id FROM active.visits WHERE id = ? FOR UPDATE", held)
			ending := startLockOrderCommand(t, func(ctx context.Context) error {
				return testpkg.WithTenantTx(t, ctx, f.k.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error { return scenario.end(txCtx, t, f) })
			})
			endPID := awaitWaiterOf(t, f.k.db, holder.pid)
			scan := startLockOrderScan(t, f.k, f.tag, f.scanRoom, f.scanDevice, f.scanStaff)
			scanned := make(chan *httptest.ResponseRecorder, 1)
			scanDone := make(chan error, 1)
			go func() { scanned <- <-scan; scanDone <- nil }()
			_, waitErr := awaitDoneOrWaiting(t, f.k.db, scanDone, endPID)
			require.NoError(t, waitErr)
			holder.commit(t)
			require.NoError(t, awaitReattach(t, ending))
			testutil.AssertSuccessResponse(t, awaitScan(t, scanned), 200)
			assertNoLockFailureRollback(t, queries)
			group, err := f.active.GetActiveGroup(testpkg.Ctx(t), f.scanStay)
			require.NoError(t, err)
			require.NotNil(t, group.EndTime)
			open, err := f.k.presence(t).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{ActiveGroupIDs: []int64{f.scanStay}, OpenOnly: true})
			require.NoError(t, err)
			require.Empty(t, open)
			var openAttendance int
			require.NoError(t, f.k.db.NewRaw("SELECT count(*) FROM active.attendance WHERE tenant_id = ? AND student_id = ? AND check_out_time IS NULL", testpkg.Tenant(t), f.student).Scan(testpkg.Ctx(t), &openAttendance))
			require.Equal(t, 1, openAttendance, "ending a room session must not end the school attendance")
		})
	}
}

func TestNewSessionStartAndScanDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := newReattachScanFixture(t)
	// Another activity must create a session, not join the existing room stay.
	f.activity = testpkg.CreateTestActivityGroup(t, f.k.db, "New Session Activity").ID
	holder := holdRow(t, f.k.db, "SELECT id FROM active.groups WHERE id = ? FOR UPDATE", f.stay)
	scan := startLockOrderScan(t, f.k, f.tag, f.room, f.scanDevice, f.scanStaff)
	scanPID := awaitWaiterOf(t, f.k.db, holder.pid)
	started := startLockOrderCommand(t, func(ctx context.Context) error {
		_, err := f.active.StartActivitySessionWithSupervisors(ctx, f.activity, f.device, []int64{f.supervisor}, &f.room)
		return err
	})
	awaitWaiterOf(t, f.k.db, scanPID)
	holder.commit(t)
	testutil.AssertSuccessResponse(t, awaitScan(t, scan), 200)
	require.NoError(t, awaitReattach(t, started))
	session, err := f.active.GetDeviceCurrentSession(testpkg.Ctx(t), f.device)
	require.NoError(t, err)
	require.NotEqual(t, f.stay, session.ID)
	visits, err := f.k.presence(t).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{StudentIDs: []int64{f.scanned, f.takenOver}, OpenOnly: true})
	require.NoError(t, err)
	require.Len(t, visits, 2)
	for _, visit := range visits {
		if visit.StudentID == f.scanned {
			require.Equal(t, f.stay, visit.ActiveGroupID)
		} else {
			require.Equal(t, session.ID, visit.ActiveGroupID)
		}
	}
}

func TestForceStartAndScanDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := newForceStartScanFixture(t)
	ctx := testpkg.Ctx(t)
	holder := holdRow(t, f.k.db, "SELECT id FROM active.groups WHERE id = ? FOR UPDATE", f.stay)
	started := startLockOrderCommand(t, func(ctx context.Context) error {
		_, err := f.active.ForceStartActivitySessionWithSupervisors(ctx, f.activity, f.device, []int64{f.supervisor}, &f.room)
		return err
	})
	startPID := awaitWaiterOf(t, f.k.db, holder.pid)
	scan := startLockOrderScan(t, f.k, f.tag, f.room, f.scanDevice, f.scanStaff)
	awaitWaiterOf(t, f.k.db, startPID)
	holder.commit(t)
	require.NoError(t, awaitReattach(t, started))
	testutil.AssertSuccessResponse(t, awaitScan(t, scan), 200)
	session, err := f.active.GetDeviceCurrentSession(ctx, f.device)
	require.NoError(t, err)
	visits, err := f.k.presence(t).ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{f.scanned, f.takenOver}, OpenOnly: true})
	require.NoError(t, err)
	require.Len(t, visits, 2)
	for _, visit := range visits {
		require.Equal(t, session.ID, visit.ActiveGroupID)
	}
}

func TestScanAndSessionEndPathsDoNotDeadlock(t *testing.T) {
	t.Parallel()
	for _, scenario := range sessionEndScanCases() {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			f, _ := newSessionEndScanFixture(t)
			scenario := prepareTimetableScanCase(t, scenario, f)
			queries := testpkg.CaptureQueries(t, f.k.db)
			holder := holdRow(t, f.k.db, "SELECT id FROM active.groups WHERE id = ? FOR UPDATE", f.scanStay)
			scan := startLockOrderScan(t, f.k, f.tag, f.scanRoom, f.scanDevice, f.scanStaff)
			scanPID := awaitWaiterOf(t, f.k.db, holder.pid)
			ending := startLockOrderCommand(t, func(ctx context.Context) error {
				return testpkg.WithTenantTx(t, ctx, f.k.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error { return scenario.end(txCtx, t, f) })
			})
			// The helper waits for the visit already owned by the scan, before it
			// can take a session row. The old order waits on the holder instead.
			finished, err := awaitDoneOrWaiting(t, f.k.db, ending, scanPID)
			require.False(t, finished, "the end must wait for the uncommitted scan: %v", err)
			holder.commit(t)
			testutil.AssertSuccessResponse(t, awaitScan(t, scan), 200)
			require.NoError(t, awaitReattach(t, ending))
			assertNoLockFailureRollback(t, queries)
			group, err := f.active.GetActiveGroup(testpkg.Ctx(t), f.scanStay)
			require.NoError(t, err)
			require.NotNil(t, group.EndTime)
			open, err := f.k.presence(t).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{ActiveGroupIDs: []int64{f.scanStay}, OpenOnly: true})
			require.NoError(t, err)
			require.Empty(t, open)
			var openAttendance int
			require.NoError(t, f.k.db.NewRaw("SELECT count(*) FROM active.attendance WHERE tenant_id = ? AND student_id = ? AND check_out_time IS NULL", testpkg.Tenant(t), f.student).Scan(testpkg.Ctx(t), &openAttendance))
			require.Equal(t, 1, openAttendance, "ending a room session must not end the school attendance")
		})
	}
}

func prepareTimetableScanCase(t *testing.T, scenario sessionEndScanCase, f bulkMoveScanFixture) sessionEndScanCase {
	t.Helper()
	if scenario.end != nil {
		return scenario
	}
	options := testpkg.ActivityInstanceOpts{IsSpontaneous: true, StartHHMM: "00:00", EndHHMM: "23:59"}
	if scenario.name == "timetable_complete" || scenario.name == "timetable_cancel" {
		options.ActiveGroupID = &f.scanStay
	}
	instance := testpkg.CreateTestActivityInstance(t, f.k.db, testpkg.TodayDate(), f.scanRoom, options)
	module, _ := api.NewActiveTestTimetable(testpkg.NewDatabase(t, f.k.db), testpkg.TenantRuntime(t, f.k.db))
	switch scenario.name {
	case "timetable_start":
		staff := testpkg.CreateTestStaff(t, f.k.db, "Start", "Timetable")
		scenario.end = func(ctx context.Context, _ *testing.T, _ bulkMoveScanFixture) error {
			_, err := module.Instance.Start(ctx, instance.ID, staff.ID)
			return err
		}
	case "timetable_cancel":
		scenario.end = func(ctx context.Context, _ *testing.T, _ bulkMoveScanFixture) error {
			_, err := module.Instance.Cancel(ctx, instance.ID, nil, nil)
			return err
		}
	default:
		scenario.end = func(ctx context.Context, _ *testing.T, _ bulkMoveScanFixture) error {
			_, err := module.Instance.Complete(ctx, instance.ID)
			return err
		}
	}
	return scenario
}

func TestScanAndAdminReopenDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f, _ := newSessionEndScanFixture(t)
	_, _, target := f.k.roomWithSession(t, "Admin Reopen Target")
	visit, err := f.k.presence(t).FindVisit(testpkg.Ctx(t), f.visit)
	require.NoError(t, err)
	visit.ActiveGroupID = target
	operations := api.NewActiveTestPresence(testpkg.NewDatabase(t, f.k.db), testpkg.TenantRuntime(t, f.k.db)).PresenceOperations()
	holder := holdRow(t, f.k.db, "SELECT id FROM active.groups WHERE id = ? FOR UPDATE", f.scanStay)
	scan := startLockOrderScan(t, f.k, f.tag, f.scanRoom, f.scanDevice, f.scanStaff)
	scanPID := awaitWaiterOf(t, f.k.db, holder.pid)
	amended := startLockOrderCommand(t, func(ctx context.Context) error { return operations.AmendVisit(ctx, *visit) })
	awaitWaiterOf(t, f.k.db, scanPID)
	holder.commit(t)
	testutil.AssertSuccessResponse(t, awaitScan(t, scan), 200)
	require.NoError(t, awaitReattach(t, amended))
	open, err := f.k.presence(t).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{StudentIDs: []int64{f.student}, OpenOnly: true})
	require.NoError(t, err)
	require.Len(t, open, 1)
	require.Equal(t, target, open[0].ActiveGroupID)
}

func newForceStartScanFixture(t *testing.T) reattachScanFixture {
	t.Helper()
	f := newReattachScanFixture(t)
	ctx := testpkg.Ctx(t)
	// O is the activity session whose children are taken over; D is the
	// running device session. The scan changes the child from O into D.
	_, err := f.k.db.NewUpdate().Table("active.groups").Set("end_time = NULL").Where("id = ?", f.ended).Exec(ctx)
	require.NoError(t, err)
	_, err = f.k.db.NewUpdate().Table("active.visits").Set("active_group_id = ?", f.stay).Where("id = ?", f.scannedV).Exec(ctx)
	require.NoError(t, err)
	deviceSession, err := f.active.GetActiveGroup(ctx, f.ended)
	require.NoError(t, err)
	f.room = deviceSession.RoomID
	f.scanDevice = f.startKiosk

	return f
}

func TestScanAndForceStartDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := newForceStartScanFixture(t)
	ctx := testpkg.Ctx(t)
	holder := holdRow(t, f.k.db, "SELECT id FROM active.groups WHERE id = ? FOR UPDATE", f.ended)
	scan := startLockOrderScan(t, f.k, f.tag, f.room, f.scanDevice, f.scanStaff)
	scanPID := awaitWaiterOf(t, f.k.db, holder.pid)
	started := startLockOrderCommand(t, func(ctx context.Context) error {
		_, err := f.active.ForceStartActivitySessionWithSupervisors(ctx, f.activity, f.device, []int64{f.supervisor}, &f.room)
		return err
	})
	awaitWaiterOf(t, f.k.db, scanPID)
	holder.commit(t)
	testutil.AssertSuccessResponse(t, awaitScan(t, scan), 200)
	require.NoError(t, awaitReattach(t, started))
	session, err := f.active.GetDeviceCurrentSession(ctx, f.device)
	require.NoError(t, err)
	visits, err := f.k.presence(t).ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{f.scanned, f.takenOver}, OpenOnly: true})
	require.NoError(t, err)
	require.Len(t, visits, 2)
	for _, visit := range visits {
		require.Equal(t, session.ID, visit.ActiveGroupID)
	}
}

func TestNewSessionStartBeforeScanDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := newReattachScanFixture(t)
	f.activity = testpkg.CreateTestActivityGroup(t, f.k.db, "New Session First Activity").ID
	holder := holdRow(t, f.k.db, "SELECT id FROM facilities.rooms WHERE id = ? FOR UPDATE", f.room)
	started := startLockOrderCommand(t, func(ctx context.Context) error {
		_, err := f.active.StartActivitySessionWithSupervisors(ctx, f.activity, f.device, []int64{f.supervisor}, &f.room)
		return err
	})
	startPID := awaitWaiterOf(t, f.k.db, holder.pid)
	scan := startLockOrderScan(t, f.k, f.tag, f.room, f.scanDevice, f.scanStaff)
	awaitWaiterOf(t, f.k.db, startPID)
	holder.commit(t)
	require.NoError(t, awaitReattach(t, started))
	testutil.AssertSuccessResponse(t, awaitScan(t, scan), 200)
	visits, err := f.k.presence(t).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{StudentIDs: []int64{f.scanned, f.takenOver}, OpenOnly: true})
	require.NoError(t, err)
	require.Len(t, visits, 2)
}

func assertNoLockFailureRollback(t *testing.T, queries *testpkg.QueryCounter) {
	t.Helper()
	require.Empty(t, queries.Matching(func(sql string) bool { return strings.Contains(sql, "rollback to savepoint") }), "a best-effort heartbeat must not hide a PostgreSQL lock failure")
}

func startLockOrderScan(t *testing.T, k *kiosk, tag string, room int64, device, staff testutil.RequestOption) <-chan *httptest.ResponseRecorder {
	t.Helper()
	router := testutil.NewTenantRouter(k.db)
	router.Mount("/", k.resource.Router())
	request := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", checkinBody(tag, room), device, staff)
	ctx, cancel := context.WithTimeout(request.Context(), 20*time.Second)
	request = request.WithContext(ctx)
	result := make(chan *httptest.ResponseRecorder, 1)
	done := make(chan struct{})
	t.Cleanup(func() { cancel(); awaitLockOrderActor(t, done) })
	go func() { defer close(done); result <- testutil.ExecuteRequest(router, request) }()
	return result
}

func startLockOrderCommand(t *testing.T, command func(context.Context) error) <-chan error {
	t.Helper()
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	result := make(chan error, 1)
	done := make(chan struct{})
	t.Cleanup(func() { cancel(); awaitLockOrderActor(t, done) })
	go func() { defer close(done); result <- command(ctx) }()
	return result
}

func awaitLockOrderActor(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("actor did not stop after cancellation")
	}
}

func TestAdminTransferAndBulkMoveDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := newBulkMoveScanFixture(t)
	visit, err := f.k.presence(t).FindVisit(testpkg.Ctx(t), f.visit)
	require.NoError(t, err)
	visit.ActiveGroupID = f.moveTarget
	at := time.Now()
	visit.ExitTime = &at
	operations := api.NewActiveTestPresence(testpkg.NewDatabase(t, f.k.db), testpkg.TenantRuntime(t, f.k.db)).PresenceOperations()
	holder := holdRow(t, f.k.db, "SELECT id FROM active.visits WHERE id = ? FOR UPDATE", f.visit)
	moved := startLockOrderCommand(t, func(ctx context.Context) error {
		_, err := f.active.MoveStudentsToActiveGroupAuthorized(ctx, []int64{f.student}, f.moveTarget, studentpresence.StudentMoveAuthorization{BypassResourceChecks: true})
		return err
	})
	awaitWaiterOf(t, f.k.db, holder.pid)
	amended := startLockOrderCommand(t, func(ctx context.Context) error { return operations.AmendVisit(ctx, *visit) })
	// An admin now takes the visit first. A move waiting for that visit must
	// not already own its target session, or it would close the inverse edge.
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), f.k.db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
		var id int64
		return tx.NewRaw("SELECT id FROM active.groups WHERE tenant_id = ? AND id = ? FOR UPDATE NOWAIT", testpkg.Tenant(t), f.moveTarget).Scan(ctx, &id)
	}))
	holder.commit(t)
	require.NoError(t, awaitReattach(t, moved))
	require.NoError(t, awaitReattach(t, amended))
	f.assertOnlyOpenVisitIn(t, f.moveTarget)
}

// reattachKiosk composes the check-in route on a pool of the test's own
// (#5269): the row holder, the scan and session start requests and the lock
// diagnostics hold connections at the same time, so they must not compete
// with parallel tests for the shared package pool.
func reattachKiosk(t *testing.T) *kiosk {
	t.Helper()
	db := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })
	module, err := api.NewCheckinTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return &kiosk{db: db, resource: checkinAPI.NewResource(module.DeviceScan, testRuntime(), nil), rosters: module.Rosters}
}

func TestVisitIntervalRevisionWaitsForTheBulkMoveChild(t *testing.T) {
	t.Parallel()
	f := newBulkMoveScanFixture(t)
	// An interval correction of the first visit can also revise the matching
	// school attendance. Match the fixture intervals before either actor starts.
	_, err := f.k.db.NewRaw(`UPDATE active.visits SET entry_time = (SELECT check_in_time FROM active.attendance WHERE tenant_id = ? AND id = ?) WHERE tenant_id = ? AND id = ?`, testpkg.Tenant(t), f.attendance, testpkg.Tenant(t), f.visit).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	visit, err := f.k.presence(t).FindVisit(testpkg.Ctx(t), f.visit)
	require.NoError(t, err)
	visit.EntryTime = visit.EntryTime.Add(-time.Minute)
	operations := api.NewActiveTestPresence(testpkg.NewDatabase(t, f.k.db), testpkg.TenantRuntime(t, f.k.db)).PresenceOperations()
	holder := holdRow(t, f.k.db, "SELECT id FROM active.attendance WHERE id = ? FOR UPDATE", f.attendance)
	moved := startLockOrderCommand(t, func(ctx context.Context) error {
		_, err := f.active.MoveStudentsToActiveGroupAuthorized(ctx, []int64{f.student}, f.moveTarget, studentpresence.StudentMoveAuthorization{BypassResourceChecks: true})
		return err
	})
	movePID := awaitWaiterOf(t, f.k.db, holder.pid)
	amended := startLockOrderCommand(t, func(ctx context.Context) error { return operations.AmendVisit(ctx, *visit) })
	// Without the child prefix, the amendment owns the visit and waits for
	// attendance, while the move will own attendance and wait for that visit.
	awaitWaiterOf(t, f.k.db, movePID)
	holder.commit(t)
	require.NoError(t, awaitReattach(t, moved))
	require.ErrorIs(t, awaitReattach(t, amended), studentpresence.ErrStudentAlreadyActive, "reopening the ended interval must conflict with the move's replacement visit")
	f.assertOnlyOpenVisitIn(t, f.moveTarget)
}
