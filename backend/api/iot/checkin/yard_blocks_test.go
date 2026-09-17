package checkin_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Yard scans while blocks run (ADR 0019, #3282): a child joins the running
// block whose day roster lists it, the newest on several matches; any other
// child gets an independent stay in the Freispiel session.

// blockRosters is the roster lookup the composed workflow reads.
type blockRosters interface {
	SessionsRosteringStudent(ctx context.Context, studentID int64, sessionIDs []int64) ([]int64, error)
}

// openVisitSession returns the session of the child's single open visit.
func (k *kiosk) openVisitSession(t *testing.T, studentID int64) int64 {
	t.Helper()
	var open []int64
	for _, visit := range k.openVisits(t, studentID) {
		if visit.ExitTime == nil {
			open = append(open, visit.ActiveGroupID)
		}
	}
	require.Len(t, open, 1, "student %d should have exactly one open visit", studentID)
	return open[0]
}

func TestDeviceCheckin_SchulhofBlockRosterDecidesTheTarget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "YardBlock", "Staff")
	device := k.device(t, "yard-blocks")
	yard := testpkg.CreateTestSystemRoom(t, k.db, "Schulhof", true)
	tagA, inA, _ := k.studentCard(t, "RosterA", "Child", "1a")
	tagBoth, inBoth, _ := k.studentCard(t, "RosterBoth", "Child", "1b")
	tagNone, inNone, _ := k.studentCard(t, "RosterNone", "Child", "2a")
	// Block A starts first; the former rule sent every child to the later B.
	blockA := testpkg.CreateTestRunningBlock(t, k.db, yard.ID, "Hof Fußball", inA, inBoth)
	blockB := testpkg.CreateTestRunningBlock(t, k.db, yard.ID, "Hof Seilspringen", inBoth)

	rr := k.callIsolated(t, "POST", "/checkin", checkinBody(tagA, yard.ID), device, staff)
	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "Schulhof", responseData(t, rr.Body.Bytes())["room_name"])
	assert.Equal(t, blockA.ID, k.openVisitSession(t, inA), "rostered in A only: A, regardless of start order")

	rr = k.callIsolated(t, "POST", "/checkin", checkinBody(tagBoth, yard.ID), device, staff)
	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, blockB.ID, k.openVisitSession(t, inBoth), "rostered in both: the newest block")

	rr = k.callIsolated(t, "POST", "/checkin", checkinBody(tagNone, yard.ID), device, staff)
	testutil.AssertSuccessResponse(t, rr, 200)
	freeplay := testpkg.LatestActiveGroupInRoom(t, k.db, yard.ID)
	assert.NotContains(t, []int64{blockA.ID, blockB.ID}, freeplay.ID, "a Freispiel session is provisioned")
	assert.Nil(t, freeplay.DeviceID, "the Freispiel session belongs to the room")
	assert.Equal(t, freeplay.ID, k.openVisitSession(t, inNone), "rostered nowhere: independent stay")

	// A second unrostered child reuses that Freispiel session.
	tagLate, inLate, _ := k.studentCard(t, "RosterLate", "Child", "2b")
	rr = k.callIsolated(t, "POST", "/checkin", checkinBody(tagLate, yard.ID), device, staff)
	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, freeplay.ID, k.openVisitSession(t, inLate))
	assert.Equal(t, freeplay.ID, testpkg.LatestActiveGroupInRoom(t, k.db, yard.ID).ID, "no second Freispiel session")
}

func TestDeviceCheckin_SchulhofWithoutBlocksKeepsTheKioskSession(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "YardKiosk", "Staff")
	device, deviceID := k.deviceRow(t, "yard-kiosk")
	yard := testpkg.CreateTestSystemRoom(t, k.db, "Schulhof", true)
	tagFirst, first, _ := k.studentCard(t, "KioskFirst", "Child", "1a")
	tagSecond, second, _ := k.studentCard(t, "KioskSecond", "Child", "1b")

	rr := k.callIsolated(t, "POST", "/checkin", checkinBody(tagFirst, yard.ID), device, staff)
	testutil.AssertSuccessResponse(t, rr, 200)
	freeplay := testpkg.LatestActiveGroupInRoom(t, k.db, yard.ID)
	// The kiosk takes over the Freispiel session, as after a phone start.
	testpkg.LinkDeviceToActiveGroup(t, k.db, freeplay.ID, deviceID)

	rr = k.callIsolated(t, "POST", "/checkin", checkinBody(tagSecond, yard.ID), device, staff)
	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, freeplay.ID, k.openVisitSession(t, first))
	assert.Equal(t, freeplay.ID, k.openVisitSession(t, second))
}

func TestDeviceCheckin_UnreleasedSchulhofAppliesOnlyTheRoster(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "YardClosed", "Staff")
	device := k.device(t, "yard-closed")
	yard := testpkg.CreateTestSystemRoom(t, k.db, "Schulhof", false)
	tagRostered, rostered, _ := k.studentCard(t, "ClosedRoster", "Child", "1a")
	tagOther, other, _ := k.studentCard(t, "ClosedOther", "Child", "1b")

	// Without a running session an unreleased yard stays a plain room.
	rr := k.callIsolated(t, "POST", "/checkin", checkinBody(tagOther, yard.ID), device, staff)
	testutil.AssertNotFound(t, rr)
	assert.Contains(t, rr.Body.String(), "no active groups in specified room")

	blockA := testpkg.CreateTestRunningBlock(t, k.db, yard.ID, "Hof Basteln", rostered)
	blockB := testpkg.CreateTestRunningBlock(t, k.db, yard.ID, "Hof Tanzen")

	rr = k.callIsolated(t, "POST", "/checkin", checkinBody(tagRostered, yard.ID), device, staff)
	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, blockA.ID, k.openVisitSession(t, rostered))

	// No Freispiel session without release: the ordinary newest-session rule.
	rr = k.callIsolated(t, "POST", "/checkin", checkinBody(tagOther, yard.ID), device, staff)
	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, blockB.ID, k.openVisitSession(t, other))
	assert.Equal(t, blockB.ID, testpkg.LatestActiveGroupInRoom(t, k.db, yard.ID).ID, "no session was provisioned")
}

func TestDeviceCheckin_SchulhofBlockRostersStayInsideTheSchool(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	yard := testpkg.CreateTestSystemRoom(t, k.db, "Schulhof", true)
	_, studentID, _ := k.studentCard(t, "RosterTenant", "Child", "1a")
	block := testpkg.CreateTestRunningBlock(t, k.db, yard.ID, "Hof Tenant", studentID)
	otherTenant, _ := testpkg.CreateTestTenant(t, k.db)

	lookup := func(tenantID int64) []int64 {
		var sessions []int64
		require.NoError(t, testpkg.WithTenantTx(t, context.Background(), k.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
			var err error
			sessions, err = k.rosters.SessionsRosteringStudent(ctx, studentID, []int64{block.ID})
			return err
		}))
		return sessions
	}

	assert.Equal(t, []int64{block.ID}, lookup(testpkg.Tenant(t)))
	assert.Empty(t, lookup(otherTenant), "another school's device never sees this school's rosters")
}
