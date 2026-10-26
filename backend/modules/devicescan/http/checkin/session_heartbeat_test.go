package checkin_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A successful kiosk scan keeps the device session alive by itself, so the
// kiosk needs no POST /session/activity after it (#5004). These cases are
// the paths where the session was not refreshed before.

// kioskSession links a running session in a fresh room to the device and
// backdates its heartbeat; it returns the room, the session and the stale
// heartbeat.
func (k *kiosk) kioskSession(t *testing.T, roomName string, deviceID int64) (int64, int64, time.Time) {
	t.Helper()
	roomID, _, sessionID := k.roomWithSession(t, roomName)
	testpkg.LinkDeviceToActiveGroup(t, k.db, sessionID, deviceID)
	return roomID, sessionID, backdateLastActivity(t, k.db, sessionID)
}

func TestScanHeartbeat_BinaryModeScan(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	testpkg.SetTestTenantSetting(t, k.db, testpkg.Tenant(t), "operations.presence_mode", `"binary"`)
	device, deviceID := k.deviceRow(t, "binary-heartbeat")
	tag, _, _ := k.studentCard(t, "Binary", "Heartbeat", "1a")
	roomID, sessionID, stale := k.kioskSession(t, "Binary Heartbeat Room", deviceID)

	rr := k.call(t, "POST", "/checkin", checkinBody(tag, roomID), device)

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "checked_in", responseData(t, rr.Body.Bytes())["action"])
	assert.True(t, testpkg.ActiveGroupLastActivity(t, k.db, sessionID).After(stale), "the binary scan refreshes the device session")
}

func TestScanHeartbeat_CheckinOutsideTheKioskRoom(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	k := setupCheckinRoute(t)
	device, deviceID := k.deviceRow(t, "wc-heartbeat")
	staff, _, _ := k.staff(t, "Heartbeat", "Staff")
	tag, _, _ := k.studentCard(t, "WC", "Heartbeat", "2a")
	_, sessionID, stale := k.kioskSession(t, "Kiosk Heartbeat Room", deviceID)
	wc := testpkg.CreateTestRoomNamed(t, k.db, "WC")

	rr := k.callIsolated(t, "POST", "/checkin", checkinBody(tag, wc.ID), device, staff)

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "checked_in", responseData(t, rr.Body.Bytes())["action"])
	assert.True(t, testpkg.ActiveGroupLastActivity(t, k.db, sessionID).After(stale), "a scan into another room refreshes the kiosk's own session")
}

func TestScanHeartbeat_CheckoutWithoutRoom(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	device, deviceID := k.deviceRow(t, "checkout-heartbeat")
	tag, studentID, _ := k.studentCard(t, "Checkout", "Heartbeat", "3a")
	_, sessionID, stale := k.kioskSession(t, "Checkout Heartbeat Room", deviceID)
	testpkg.CreateTestVisit(t, k.db, studentID, sessionID, time.Now().Add(-10*time.Minute), nil)

	rr := k.call(t, "POST", "/checkin", checkoutBody(tag), device)

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "checked_out", responseData(t, rr.Body.Bytes())["action"])
	assert.True(t, testpkg.ActiveGroupLastActivity(t, k.db, sessionID).After(stale), "a checkout without a room refreshes the device session")
}

func TestScanHeartbeat_PickupQuery(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	device, deviceID := k.deviceRow(t, "pickup-heartbeat")
	tag, _, _ := k.studentCard(t, "Pickup", "Heartbeat", "2b")
	_, sessionID, stale := k.kioskSession(t, "Pickup Heartbeat Room", deviceID)

	rr := k.call(t, "POST", "/pickup-query", map[string]any{"student_rfid": tag}, device)

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.True(t, testpkg.ActiveGroupLastActivity(t, k.db, sessionID).After(stale), "the pickup query refreshes the device session")
}

// PostgreSQL statement errors must not poison the attendance transaction.
func TestScanHeartbeat_LookupSQLFailurePreservesAttendance(t *testing.T) {
	t.Parallel()
	assertHeartbeatSQLFailurePreservesAttendance(t, "lookup")
}

func TestScanHeartbeat_UpdateSQLFailurePreservesAttendance(t *testing.T) {
	t.Parallel()
	assertHeartbeatSQLFailurePreservesAttendance(t, "update")
}

func TestScanHeartbeat_DetailedUpdateSQLFailurePreservesVisit(t *testing.T) {
	t.Parallel()
	assertHeartbeatSQLFailurePreservesAttendance(t, "detailed update")
}

func assertHeartbeatSQLFailurePreservesAttendance(t *testing.T, failure string) {
	t.Helper()
	testpkg.SetupIsolatedTestDB(t)
	k := setupCheckinRoute(t)
	ctx := testpkg.Ctx(t)
	mode := `"binary"`
	if failure == "detailed update" {
		mode = `"detailed"`
	}
	testpkg.SetTestTenantSetting(t, k.db, testpkg.Tenant(t), "operations.presence_mode", mode)
	device, deviceID := k.deviceRow(t, "failed-heartbeat")
	tag, studentID, _ := k.studentCard(t, "Failed", "Heartbeat", "1a")
	roomID, sessionID, stale := k.kioskSession(t, "Failed Heartbeat Room", deviceID)
	if failure == "lookup" {
		_, err := k.db.ExecContext(ctx, "REVOKE SELECT ON active.groups FROM phoenix_tenant")
		require.NoError(t, err)
	} else {
		_, err := k.db.ExecContext(ctx, "ALTER TABLE active.groups ADD CONSTRAINT reject_heartbeat CHECK (last_activity <= ?)", stale)
		require.NoError(t, err)
	}

	staff, _, _ := k.staff(t, "Heartbeat", "Staff")
	rr := k.callIsolated(t, "POST", "/checkin", checkinBody(tag, roomID), device, staff)

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "checked_in", responseData(t, rr.Body.Bytes())["action"])
	if failure == "detailed update" {
		var visits int
		require.NoError(t, k.db.NewRaw("SELECT count(*) FROM active.visits WHERE student_id = ? AND exit_time IS NULL", studentID).Scan(ctx, &visits))
		assert.Equal(t, 1, visits, "the visit commits despite a heartbeat SQL error")
	} else {
		var present bool
		require.NoError(t, k.db.NewRaw("SELECT EXISTS (SELECT 1 FROM active.attendance WHERE student_id = ? AND check_in_time IS NOT NULL AND check_out_time IS NULL)", studentID).Scan(ctx, &present))
		assert.True(t, present, "attendance commits despite a heartbeat SQL error")
	}
	assert.Equal(t, stale, testpkg.ActiveGroupLastActivity(t, k.db, sessionID))
}

// The lookup of the device's session in the scanned room is part of the
// heartbeat. A PostgreSQL error in that statement (for example a
// statement_timeout) must not abort the scan's transaction: the checkout
// before it commits and only the heartbeat is skipped.
func TestScanHeartbeat_RoomLookupSQLFailurePreservesCheckout(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	k := setupCheckinRoute(t)
	device, deviceID := k.deviceRow(t, "room-lookup-checkout")
	tag, studentID, _ := k.studentCard(t, "RoomLookup", "Checkout", "2a")
	roomID, sessionID, stale := k.kioskSession(t, "Room Lookup Checkout Room", deviceID)
	visit := testpkg.CreateTestVisit(t, k.db, studentID, sessionID, time.Now().Add(-10*time.Minute), nil)
	failDeviceRoomLookup(t, k, roomID, deviceID)

	rr := k.callIsolated(t, "POST", "/checkin", checkinBody(tag, roomID), device)

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "checked_out", responseData(t, rr.Body.Bytes())["action"])
	ended, err := k.presence(t).FindVisit(testpkg.Ctx(t), visit.ID)
	require.NoError(t, err)
	assert.NotNil(t, ended.ExitTime, "the checkout commits despite the failed lookup")
	assert.Equal(t, stale, testpkg.ActiveGroupLastActivity(t, k.db, sessionID), "the failed lookup skips the heartbeat")
}

// failDeviceRoomLookup makes PostgreSQL reject the statement that looks up
// the device's open session in roomID: a restrictive row policy for the
// request role raises on that session's row while the running statement
// filters on that room and device. Every other statement passes. The test
// owns its database clone, so the policy stays there.
func failDeviceRoomLookup(t *testing.T, k *kiosk, roomID, deviceID int64) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	marker := fmt.Sprintf("(room_id = %d) AND (device_id = %d)", roomID, deviceID)
	_, err := k.db.ExecContext(ctx, fmt.Sprintf(`CREATE FUNCTION active.fail_device_room_lookup() RETURNS boolean LANGUAGE plpgsql AS $$
BEGIN
	IF strpos(current_query(), %s) > 0 THEN
		RAISE EXCEPTION 'injected device-session lookup failure';
	END IF;
	RETURN true;
END $$`, quoteLiteral(marker)))
	require.NoError(t, err)
	_, err = k.db.ExecContext(ctx, `CREATE POLICY fail_device_room_lookup ON active.groups AS RESTRICTIVE FOR SELECT TO phoenix_tenant USING (active.fail_device_room_lookup())`)
	require.NoError(t, err)
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
