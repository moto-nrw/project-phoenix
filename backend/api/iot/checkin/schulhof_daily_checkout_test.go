package checkin_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestDeviceCheckout_SchulhofOffersNachHauseWithoutAutoSendingHome drives the
// GS-Barnstorf regression (#2377) through the real handler chain: a child whose
// education group owns a room checks in at the Schulhof and scans out there
// again.
//
// Both halves of the contract are asserted together on purpose, because fixing
// one by breaking the other is the trap this bug invites:
//
//   - daily_checkout_available must be true, or PyrePortal hides "nach Hause"
//     and the child stays "unterwegs" (the reported bug); and
//   - the action must stay "checked_out", NOT "checked_out_daily" — PyrePortal
//     builds its destination modal only for "checked_out", so an upgraded
//     action would show no buttons at all and silently send home every child
//     who scanned out at the yard, including those heading back inside.
func TestDeviceCheckout_SchulhofOffersNachHauseWithoutAutoSendingHome(t *testing.T) {
	ctx := setupTestContext(t)

	// No configured checkout time → the time gate is open, so this test
	// exercises the ROOM gate in isolation. Restored afterwards so a value in
	// the developer's .env cannot leak into (or out of) this test.
	previousCheckoutTime, hadCheckoutTime := os.LookupEnv("STUDENT_DAILY_CHECKOUT_TIME")
	require.NoError(t, os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME"))
	defer func() {
		if hadCheckoutTime {
			_ = os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", previousCheckoutTime)
		}
	}()

	device := testpkg.CreateTestDevice(t, ctx.db, "schulhof-home")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "SchulhofHome", "Staff")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, "SchulhofHome", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("SCHULHOFHOME%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// The whole point of the bug: the child's group HAS its own room, which is
	// what made the old room gate reject every Schulhof checkout.
	groupRoom := testpkg.CreateTestRoom(t, ctx.db, "Hausaufgaben 1/2")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, groupRoom.ID)

	educationGroup := testpkg.CreateTestEducationGroup(t, ctx.db, "Hausaufgaben")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, educationGroup.ID)

	defer assignGroupRoom(t, ctx, educationGroup.ID, groupRoom.ID)()
	defer assignStudentToGroup(t, ctx, student.ID, educationGroup.ID)()

	schulhof := createSchulhofRoom(t, ctx.db)
	defer cleanupSchulhofInfrastructure(t, ctx.db, schulhof.ID)

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	// Step 1: the child goes out to the yard (auto-creates the Schulhof
	// activity/active group on first use).
	checkinBody := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      schulhof.ID,
	}
	checkinReq := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", checkinBody,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)
	checkinRR := testutil.ExecuteRequest(router, checkinReq)
	testutil.AssertSuccessResponse(t, checkinRR, http.StatusOK)

	checkinData := responseData(t, checkinRR.Body.Bytes())
	require.Equal(t, "checked_in", checkinData["action"])
	require.Equal(t, "Schulhof", checkinData["room_name"])

	// Step 2: the child scans out at the yard kiosk to go home.
	checkoutBody := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkout",
	}
	checkoutReq := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", checkoutBody,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)
	checkoutRR := testutil.ExecuteRequest(router, checkoutReq)
	testutil.AssertSuccessResponse(t, checkoutRR, http.StatusOK)

	checkoutData := responseData(t, checkoutRR.Body.Bytes())

	assert.Equal(t, true, checkoutData["daily_checkout_available"],
		`the yard kiosk must offer "nach Hause" — without it children stay "unterwegs" (#2377)`)
	assert.Equal(t, "checked_out", checkoutData["action"],
		`the yard must not auto-send the child home: PyrePortal renders its destination modal only for "checked_out"`)
}

// TestDeviceCheckout_OrdinaryRoomDoesNotOfferNachHause is the counterpart: the
// Schulhof branch must not have opened "nach Hause" everywhere. A child leaving
// an ordinary room that is neither their group room nor the yard is still only
// moving between rooms.
func TestDeviceCheckout_OrdinaryRoomDoesNotOfferNachHause(t *testing.T) {
	ctx := setupTestContext(t)

	previousCheckoutTime, hadCheckoutTime := os.LookupEnv("STUDENT_DAILY_CHECKOUT_TIME")
	require.NoError(t, os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME"))
	defer func() {
		if hadCheckoutTime {
			_ = os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", previousCheckoutTime)
		}
	}()

	device := testpkg.CreateTestDevice(t, ctx.db, "ordinary-room")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Ordinary", "Staff")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, "Ordinary", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("ORDINARY%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	groupRoom := testpkg.CreateTestRoom(t, ctx.db, "Hausaufgaben 3/4")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, groupRoom.ID)

	educationGroup := testpkg.CreateTestEducationGroup(t, ctx.db, "Hausaufgaben34")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, educationGroup.ID)

	defer assignGroupRoom(t, ctx, educationGroup.ID, groupRoom.ID)()
	defer assignStudentToGroup(t, ctx, student.ID, educationGroup.ID)()

	// An ordinary room with its own running activity — not the group room, not
	// the Schulhof.
	otherRoom := testpkg.CreateTestRoom(t, ctx.db, "Musikraum")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, otherRoom.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Musik")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, otherRoom.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	checkinBody := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      otherRoom.ID,
	}
	checkinReq := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", checkinBody,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)
	checkinRR := testutil.ExecuteRequest(router, checkinReq)
	testutil.AssertSuccessResponse(t, checkinRR, http.StatusOK)
	require.Equal(t, "checked_in", responseData(t, checkinRR.Body.Bytes())["action"])

	checkoutBody := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkout",
	}
	checkoutReq := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", checkoutBody,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)
	checkoutRR := testutil.ExecuteRequest(router, checkoutReq)
	testutil.AssertSuccessResponse(t, checkoutRR, http.StatusOK)

	checkoutData := responseData(t, checkoutRR.Body.Bytes())
	assert.Equal(t, "checked_out", checkoutData["action"])
	assert.Equal(t, false, checkoutData["daily_checkout_available"],
		"an ordinary room must not offer nach Hause")
}

// responseData unwraps the standard {"data": {...}} envelope.
func responseData(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()

	response := testutil.ParseJSONResponse(t, body)
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "response should have a data field")
	return data
}

// assignGroupRoom points an education group at its group room and returns the
// undo. The fixture helper creates roomless groups, and a group WITH a room is
// precisely the configuration that triggered #2377.
//
// The caller must defer the returned undo: the shared fixture cleanup deletes
// the room while education.groups still references it, and that FK nulls the
// row's tenant_id, which the NOT NULL constraint then rejects — leaving the
// fixture behind.
func assignGroupRoom(t *testing.T, ctx *testContext, groupID, roomID int64) func() {
	t.Helper()

	setColumn(t, ctx, "education.groups", "room_id", roomID, groupID)
	return func() { setColumn(t, ctx, "education.groups", "room_id", nil, groupID) }
}

// assignStudentToGroup gives the student the group membership both daily
// checkout gates require, and returns the undo the caller must defer (same
// cleanup-ordering reason as assignGroupRoom).
func assignStudentToGroup(t *testing.T, ctx *testContext, studentID, groupID int64) func() {
	t.Helper()

	setColumn(t, ctx, "users.students", "group_id", groupID, studentID)
	return func() { setColumn(t, ctx, "users.students", "group_id", nil, studentID) }
}

// setColumn writes a single column on a fixture row, using nil to clear it.
func setColumn(t *testing.T, ctx *testContext, table, column string, value any, id int64) {
	t.Helper()

	_, err := ctx.db.NewUpdate().
		TableExpr(table).
		Set(column+" = ?", value).
		Where("id = ?", id).
		Exec(testutil.TenantContext(1))
	require.NoError(t, err, "failed to set %s.%s", table, column)
}
