package checkin_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// Query budgets of the three kiosk scans (#5004): a check-in, a checkout at
// the kiosk the child is in, and a room change from another room. Each scan
// runs once with three and once with eight children present and must issue
// the same statements both times. The count covers the request transaction;
// device authentication runs before it and shows as Server-Timing "auth".

// stayBefore gives the scanned child its stay before the scan.
type stayBefore func(t *testing.T, k *kiosk, studentID, kioskSessionID int64)

func noStay(*testing.T, *kiosk, int64, int64) {}

func stayInKioskRoom(t *testing.T, k *kiosk, studentID, kioskSessionID int64) {
	t.Helper()
	testpkg.CreateTestVisit(t, k.db, studentID, kioskSessionID, time.Now().Add(-30*time.Minute), nil)
}

func stayInOtherRoom(t *testing.T, k *kiosk, studentID, _ int64) {
	t.Helper()
	_, _, otherSession := k.roomWithSession(t, fmt.Sprintf("Budget Origin %d", studentID))
	testpkg.CreateTestVisit(t, k.db, studentID, otherSession, time.Now().Add(-30*time.Minute), nil)
}

// measureScan runs the scan at a kiosk with three and then eight children
// present, asserts that the statement count does not grow, and returns the
// statements of the larger run.
func measureScan(t *testing.T, before stayBefore, action string) []string {
	t.Helper()
	k := setupCheckinRoute(t)
	device, deviceID := k.deviceRow(t, "budget-"+action)
	staff, _, _ := k.staff(t, "Budget", "Staff")
	roomID, _, sessionID := k.roomWithSession(t, "Budget Kiosk Room")
	testpkg.LinkDeviceToActiveGroup(t, k.db, sessionID, deviceID)
	present := 0
	addPresent := func(count int) {
		for range count {
			present++
			peer := testpkg.CreateTestStudent(t, k.db, fmt.Sprintf("Peer%d", present), "Budget", "1a")
			testpkg.CreateTestVisit(t, k.db, peer.ID, sessionID, time.Now().Add(-time.Hour), nil)
		}
	}
	counter := testpkg.CaptureQueriesForContext(t, k.db)
	scan := func() []string {
		tag, studentID, _ := k.studentCard(t, fmt.Sprintf("Scan%d", present), "Budget", "1a")
		before(t, k, studentID, sessionID)
		router := testutil.NewTenantRouter(k.db)
		router.Mount("/", k.resource.Router())
		req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", checkinBody(tag, roomID), device, staff)
		counter.Reset()
		rr := testutil.ExecuteRequest(router, req.WithContext(counter.Context(req.Context())))
		testutil.AssertSuccessResponse(t, rr, 200)
		assert.Equal(t, action, responseData(t, rr.Body.Bytes())["action"])
		return counter.Queries()
	}

	addPresent(3)
	small := scan()
	addPresent(5)
	large := scan()
	assert.Equal(t, len(small), len(large), "the scan must not grow with the children present")
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
