package checkin_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// Query budgets of the three kiosk scans (#5004): a check-in, a checkout at
// the kiosk the child is in, and a room change from another room. Each scan
// runs once with three and once with eight children present and must issue
// the same statements both times. The count covers the request transaction;
// device authentication runs before it and shows as Server-Timing "auth".
// The scans run on scanBudgetClock: on a care weekday the scan reads the
// day's pickup plan, on a weekend it does not, so a wall clock would make
// the count depend on the day the suite runs (#6266).

// scanBudgetClock pins the scans to a school Monday at noon.
func scanBudgetClock() time.Time {
	return calendar.NewDate(2026, time.August, 24).BerlinMidnight().Add(12 * time.Hour)
}

// budgetRoomWithSession is roomWithSession with the session started an hour
// before scanBudgetClock.
func budgetRoomWithSession(t *testing.T, k *kiosk, name string) (roomID, sessionID int64) {
	t.Helper()
	room := testpkg.CreateTestRoom(t, k.db, name)
	activity := testpkg.CreateTestActivityGroup(t, k.db, name+" Activity")
	session := testpkg.CreateTestActiveGroupAt(t, k.db, activity.ID, room.ID, scanBudgetClock().Add(-time.Hour))
	return room.ID, session.ID
}

// stayBefore gives the scanned child its stay before the scan.
type stayBefore func(t *testing.T, k *kiosk, studentID, kioskSessionID int64)

func noStay(*testing.T, *kiosk, int64, int64) {}

func stayInKioskRoom(t *testing.T, k *kiosk, studentID, kioskSessionID int64) {
	t.Helper()
	testpkg.CreateTestVisit(t, k.db, studentID, kioskSessionID, scanBudgetClock().Add(-30*time.Minute), nil)
}

func stayInOtherRoom(t *testing.T, k *kiosk, studentID, _ int64) {
	t.Helper()
	_, otherSession := budgetRoomWithSession(t, k, fmt.Sprintf("Budget Origin %d", studentID))
	testpkg.CreateTestVisit(t, k.db, studentID, otherSession, scanBudgetClock().Add(-30*time.Minute), nil)
}

// measureScan runs the scan at a kiosk with three and then eight children
// present, asserts that the statement count does not grow, and returns the
// statements of the larger run.
func measureScan(t *testing.T, before stayBefore, action string, requestMemo ...bool) testpkg.QueryMeasurement {
	t.Helper()
	k := setupCheckinRoute(t, scanBudgetClock)
	device, deviceID := k.deviceRow(t, "budget-"+action)
	staff, _, _ := k.staff(t, "Budget", "Staff")
	roomID, sessionID := budgetRoomWithSession(t, k, "Budget Kiosk Room")
	testpkg.LinkDeviceToActiveGroup(t, k.db, sessionID, deviceID)
	present := 0
	addPresent := func(count int) {
		for range count {
			present++
			peer := testpkg.CreateTestStudent(t, k.db, fmt.Sprintf("Peer%d", present), "Budget", "1a")
			testpkg.CreateTestVisit(t, k.db, peer.ID, sessionID, scanBudgetClock().Add(-time.Hour), nil)
		}
	}
	counter := testpkg.CaptureQueriesForContext(t, k.db)
	scan := func() testpkg.QueryMeasurement {
		tag, studentID, _ := k.studentCard(t, fmt.Sprintf("Scan%d", present), "Budget", "1a")
		before(t, k, studentID, sessionID)
		router := testutil.NewTenantRouter(k.db)
		router.Mount("/", k.resource.Router())
		req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", checkinBody(tag, roomID), device, staff)
		if len(requestMemo) > 0 && requestMemo[0] {
			req = req.WithContext(settings.WithSettingsRequestCache(req.Context()))
		}
		counter.Reset()
		rr := testutil.ExecuteRequest(testpkg.MountedQueryBudgetRouter("/api/iot", router), req.WithContext(counter.Context(req.Context())))
		testutil.AssertSuccessResponse(t, rr, 200)
		assert.Equal(t, action, responseData(t, rr.Body.Bytes())["action"])
		return counter.Measurement(counter.Queries())
	}

	addPresent(3)
	small := scan()
	addPresent(5)
	large := scan()
	t.Logf("scan=%s N3=%d N8=%d request_memo=%v", action, len(small.Statements), len(large.Statements), requestMemo)
	assert.Equal(t, len(small.Statements), len(large.Statements), "the scan must not grow with the children present")
	return large
}

func TestScanQueryBudget_CheckIn(t *testing.T) {
	t.Parallel()
	querybudgets.AssertQueryBudget(t, "api.iot.checkin.check_in", measureScan(t, noStay, "checked_in"))
}

func TestScanQueryBudget_CheckOut(t *testing.T) {
	t.Parallel()
	querybudgets.AssertQueryBudget(t, "api.iot.checkin.check_out", measureScan(t, stayInKioskRoom, "checked_out"))
}

func TestScanQueryBudget_RoomChange(t *testing.T) {
	t.Parallel()
	querybudgets.AssertQueryBudget(t, "api.iot.checkin.room_change", measureScan(t, stayInOtherRoom, "transferred"))
}

// Request memoization matches production middleware and warms lazily.
func TestScanQueryBudget_RequestMemoCheckIn(t *testing.T) {
	t.Parallel()
	querybudgets.AssertQueryBudget(t, "api.iot.checkin.request_memo.check_in", measureScan(t, noStay, "checked_in", true))
}
func TestScanQueryBudget_RequestMemoCheckOut(t *testing.T) {
	t.Parallel()
	querybudgets.AssertQueryBudget(t, "api.iot.checkin.request_memo.check_out", measureScan(t, stayInKioskRoom, "checked_out", true))
}
func TestScanQueryBudget_RequestMemoRoomChange(t *testing.T) {
	t.Parallel()
	querybudgets.AssertQueryBudget(t, "api.iot.checkin.request_memo.room_change", measureScan(t, stayInOtherRoom, "transferred", true))
}
