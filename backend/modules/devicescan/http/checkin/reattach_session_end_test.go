package checkin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	sessionsAPI "github.com/moto-nrw/project-phoenix/modules/devicescan/http/sessions"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// #5206 review: the kiosk start that joins a room stay moves the device's
// open visits first and locks the stay afterwards. When the stay ends while
// the takeover waits, the start must not leave those visits in the ended
// stay. It runs again against the room's new state: without another session
// of the activity it opens its own session; with one it answers 409 and its
// whole request transaction rolls back. Both tests go through the kiosk's
// session start route inside the request transaction, as production does.

func sessionStartRuntime() sessionsAPI.Runtime {
	return sessionsAPI.Runtime{
		ParseID:       testpkg.ParseHTTPIDParam,
		Authenticated: func(ctx context.Context) bool { _, _, ok := testutil.DeviceIdentity(ctx); return ok },
		Success:       testutil.RespondSuccess,
		Failure: func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest, status int, err error, message string) {
			testutil.RespondCoded(w, r, status, "", err, nil, message)
		},
		Conflict: func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest, message string, info devicescan.ConflictInfoResponse) {
			render.Status(r, http.StatusConflict)
			render.JSON(w, r, map[string]any{"status": "error", "error": message, "details": info})
		},
		MarkRollback: testutil.MarkRollback,
	}
}

// startOverHTTP posts the kiosk's session start in its own goroutine.
func (f reattachScanFixture) startOverHTTP(t *testing.T) <-chan *httptest.ResponseRecorder {
	t.Helper()
	database, unit := testpkg.NewDatabase(t, f.k.db), testpkg.TenantRuntime(t, f.k.db)
	lifecycle := api.NewActiveTestDeviceScan(database, unit).SessionLifecycle
	sessionEnd := api.BindIoTSessionEnd(api.NewActiveTestSessionEnd(database, unit).SessionEnd)
	router := testutil.NewTenantRouter(f.k.db)
	router.Mount("/", sessionsAPI.NewResource(lifecycle, sessionEnd, sessionStartRuntime()).Router())
	body := map[string]any{"activity_id": f.activity, "room_id": f.room, "supervisor_ids": []int64{f.supervisor}}
	req := testutil.NewAuthenticatedRequest(t, "POST", "/start", body, f.startKiosk)
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() { result <- testutil.ExecuteRequest(router, req) }()
	return result
}

// changeStayDuringTakeover holds one of the taken-over visits, starts the
// kiosk session, waits until the takeover waits for that visit, then lets
// change commit before the takeover's next step.
func (f reattachScanFixture) changeStayDuringTakeover(t *testing.T, change func(t *testing.T)) *httptest.ResponseRecorder {
	t.Helper()
	holder := holdRow(t, f.k.db, "SELECT id FROM active.visits WHERE id = ? FOR UPDATE", f.scannedV)
	start := f.startOverHTTP(t)
	awaitWaiterOf(t, f.k.db, holder.pid)
	change(t)
	holder.commit(t)
	return awaitScan(t, start)
}

// endStayDuringTakeover ends the stay while the takeover waits and runs
// between the session end's commit and the takeover's next step.
func (f reattachScanFixture) endStayDuringTakeover(t *testing.T, between func(t *testing.T)) *httptest.ResponseRecorder {
	t.Helper()
	return f.changeStayDuringTakeover(t, func(t *testing.T) {
		require.NoError(t, f.active.EndActivitySession(testpkg.Ctx(t), f.stay), "the stay ends while the takeover waits")
		between(t)
	})
}

func (f reattachScanFixture) openVisitsOf(t *testing.T) map[int64]int64 {
	t.Helper()
	return f.openVisitSessions(t)
}

func (f reattachScanFixture) assertStayEndedEmpty(t *testing.T) {
	t.Helper()
	var endTime *time.Time
	var deviceID *int64
	require.NoError(t, f.k.db.NewRaw("SELECT end_time, device_id FROM active.groups WHERE id = ?", f.stay).
		Scan(testpkg.Ctx(t), &endTime, &deviceID))
	assert.NotNil(t, endTime, "the stay stays ended")
	assert.Nil(t, deviceID, "the kiosk did not attach to the ended stay")
	visits, err := f.k.presence(t).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{ActiveGroupIDs: []int64{f.stay}})
	require.NoError(t, err)
	assert.Empty(t, visits, "no visit was moved into the ended stay")
}

// TestKioskStartOpensItsOwnSessionWhenTheJoinedStayEnds: nothing else runs
// the activity, so the start opens a session of its own and takes the
// children over into it.
func TestKioskStartOpensItsOwnSessionWhenTheJoinedStayEnds(t *testing.T) {
	t.Parallel()
	f := newReattachScanFixture(t)

	rr := f.endStayDuringTakeover(t, func(*testing.T) {})

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	started := int64(responseData(t, rr.Body.Bytes())["active_group_id"].(float64))
	assert.NotEqual(t, f.stay, started, "the start opened its own session")
	f.assertStayEndedEmpty(t)
	assert.Equal(t, map[int64]int64{f.scanned: started, f.takenOver: started}, f.openVisitsOf(t),
		"both children moved into the new session")
	var deviceID *int64
	require.NoError(t, f.k.db.NewRaw("SELECT device_id FROM active.groups WHERE id = ?", started).Scan(testpkg.Ctx(t), &deviceID))
	require.NotNil(t, deviceID)
	assert.Equal(t, f.device, *deviceID)
}

// TestKioskStartRollsBackWhenTheJoinedStayEndsAndTheActivityRunsElsewhere:
// while the takeover waits, an admin opens the activity on another kiosk in
// another room, so
// the repeated start is a conflict. The kiosk gets 409 and nothing of the
// start commits: the children stay in the device's ended session.
func TestKioskStartRollsBackWhenTheJoinedStayEndsAndTheActivityRunsElsewhere(t *testing.T) {
	t.Parallel()
	f := newReattachScanFixture(t)
	elsewhere := testpkg.CreateTestRoom(t, f.k.db, "Reattach Turnhalle")
	otherKiosk := testpkg.CreateTestDevice(t, f.k.db, "reattach-other-kiosk")

	rr := f.endStayDuringTakeover(t, func(t *testing.T) {
		now := time.Now()
		activityID := f.activity
		require.NoError(t, f.active.CreateActiveGroup(testpkg.Ctx(t), &studentpresence.LiveGroup{
			ActivityGroupID: &activityID, RoomID: elsewhere.ID, DeviceID: &otherKiosk.ID, StartTime: now, LastActivity: now,
		}), "an admin opens the activity on another kiosk in another room")
	})

	assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
	f.assertStayEndedEmpty(t)
	assert.Equal(t, map[int64]int64{f.scanned: f.ended, f.takenOver: f.ended}, f.openVisitsOf(t),
		"the rolled back start left both children in the device's ended session")
	var started int
	require.NoError(t, f.k.db.NewRaw("SELECT count(*) FROM active.groups WHERE device_id = ? AND end_time IS NULL", f.device).
		Scan(testpkg.Ctx(t), &started))
	assert.Zero(t, started, "no session of the kiosk committed")
}

// TestKioskStartRollsBackWhenTheJoinedStayMovesRoom: while the takeover
// waits, an admin moves the stay to another room (UpdateActiveGroup takes
// only that room's lock). The start must not take the children into the
// moved stay and count them against the kiosk's room. Run again, it finds the
// activity's stay in another room, as a start after the move would, and
// answers 409 without committing anything.
func TestKioskStartRollsBackWhenTheJoinedStayMovesRoom(t *testing.T) {
	t.Parallel()
	f := newReattachScanFixture(t)
	elsewhere := testpkg.CreateTestRoom(t, f.k.db, "Reattach Werkraum")
	operations := api.NewActiveTestPresence(testpkg.NewDatabase(t, f.k.db), testpkg.TenantRuntime(t, f.k.db)).PresenceOperations()

	rr := f.changeStayDuringTakeover(t, func(t *testing.T) {
		groups, err := f.k.presence(t).ListLiveGroups(testpkg.Ctx(t), []int64{f.stay})
		require.NoError(t, err)
		require.Len(t, groups, 1)
		moved := groups[0]
		moved.RoomID = elsewhere.ID
		_, err = operations.ReviseSession(testpkg.Ctx(t), moved)
		require.NoError(t, err, "an admin moves the stay to another room")
	})

	assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
	var stayRoom int64
	var stayDevice *int64
	require.NoError(t, f.k.db.NewRaw("SELECT room_id, device_id FROM active.groups WHERE id = ?", f.stay).Scan(testpkg.Ctx(t), &stayRoom, &stayDevice))
	assert.Equal(t, elsewhere.ID, stayRoom)
	assert.Nil(t, stayDevice, "the kiosk did not attach to the moved stay")
	assert.Equal(t, map[int64]int64{f.scanned: f.ended, f.takenOver: f.ended}, f.openVisitsOf(t),
		"the rolled back start left both children in the device's ended session")
	var started int
	require.NoError(t, f.k.db.NewRaw("SELECT count(*) FROM active.groups WHERE device_id = ? AND end_time IS NULL", f.device).
		Scan(testpkg.Ctx(t), &started))
	assert.Zero(t, started, "no session of the kiosk committed")
}
