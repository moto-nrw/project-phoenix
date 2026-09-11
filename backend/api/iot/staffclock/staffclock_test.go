// Package staffclock_test exercises the staff-clock HTTP boundary with the
// real staff-clock workflow, repositories and tenant-scoped transactions. The
// resource is composed over the test-support envelope, so this package needs
// neither the shared HTTP package nor the persistence models.
package staffclock_test

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	staffclockAPI "github.com/moto-nrw/project-phoenix/api/iot/staffclock"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// staffClockFixtures links cards to people and sends device-authenticated
// requests to the composed resource. The closures capture the test database
// so the test never has to name the persistence types.
type staffClockFixtures struct {
	execute         func(t *testing.T, path string, body map[string]string) *httptest.ResponseRecorder
	linkStaffCard   func(t *testing.T, firstName, lastName, tag string) string
	linkStudentCard func(t *testing.T, firstName, lastName, tag string) string
}

func setupStaffClockRoute(t *testing.T) (*staffclockAPI.Resource, staffClockFixtures) {
	t.Helper()
	db, module := testutil.SetupWorkSessionModule(t)
	testDevice := testpkg.CreateTestDevice(t, db, "staff-clock-device")
	resource := staffclockAPI.NewResource(module.StaffClock, staffclockAPI.Runtime{
		Success: testutil.RespondSuccess,
		Failure: testutil.RespondCoded,
	})
	return resource, staffClockFixtures{
		execute: func(t *testing.T, path string, body map[string]string) *httptest.ResponseRecorder {
			t.Helper()
			router := testutil.NewTenantRouter(db)
			router.Mount("/", resource.Router())
			req := testutil.NewAuthenticatedRequest(t, "POST", path, body, testutil.WithDeviceContext(testDevice))
			return testutil.ExecuteRequest(router, req)
		},
		linkStaffCard: func(t *testing.T, firstName, lastName, tag string) string {
			t.Helper()
			staff := testpkg.CreateTestStaff(t, db, firstName, lastName)
			card := testpkg.CreateTestRFIDCard(t, db, tag)
			testpkg.LinkRFIDToStudent(t, db, staff.PersonID, card.ID)
			return card.ID
		},
		linkStudentCard: func(t *testing.T, firstName, lastName, tag string) string {
			t.Helper()
			student := testpkg.CreateTestStudent(t, db, firstName, lastName, "1a")
			card := testpkg.CreateTestRFIDCard(t, db, tag)
			testpkg.LinkRFIDToStudent(t, db, student.PersonID, card.ID)
			return card.ID
		},
	}
}

// call performs one request and returns its status and parsed body.
func (f staffClockFixtures) call(t *testing.T, path string, body map[string]string) (int, map[string]any) {
	t.Helper()
	response := f.execute(t, path, body)
	return response.Code, testutil.ParseJSONResponse(t, response.Body.Bytes())
}

// stamp performs one request that must succeed and returns its data payload.
func (f staffClockFixtures) stamp(t *testing.T, path string, body map[string]string) map[string]any {
	t.Helper()
	code, parsed := f.call(t, path, body)
	require.Equal(t, 200, code, "response: %v", parsed)
	data, ok := parsed["data"].(map[string]any)
	require.True(t, ok, "response carries a data object: %v", parsed)
	return data
}

func TestStaffClock_FullNFCFlow(t *testing.T) {
	t.Parallel()

	_, route := setupStaffClockRoute(t)
	tag := route.linkStaffCard(t, "Nora", "Kiosk", "A1654BEEF")

	initial := route.stamp(t, "/staff-clock/state", map[string]string{"rfid_tag": tag})
	assert.Equal(t, devicescan.StaffClockStateCheckedOut, initial["state"])
	assert.Equal(t, "Nora Kiosk", initial["staff_name"])

	checkedIn := route.stamp(t, "/staff-clock", map[string]string{
		"rfid_tag": tag,
		"action":   devicescan.StaffClockActionCheckIn,
		"status":   workforce.WorkSessionStatusPresent,
	})
	assert.Equal(t, devicescan.StaffClockStateCheckedIn, checkedIn["state"])
	session := checkedIn["session"].(map[string]any)
	assert.Equal(t, workforce.WorkSessionSourceNFC, session["source"])
	assert.Equal(t, workforce.WorkSessionStatusPresent, session["status"])

	// A second check-in while clocked in is a state conflict the kiosk knows
	// how to recover from, with the stable code PyrePortal maps.
	duplicateCode, duplicate := route.call(t, "/staff-clock", map[string]string{
		"rfid_tag": tag,
		"action":   devicescan.StaffClockActionCheckIn,
		"status":   workforce.WorkSessionStatusPresent,
	})
	require.Equal(t, 409, duplicateCode, "response: %v", duplicate)
	assert.Equal(t, "invalid_staff_clock_state", duplicate["code"])
	assert.Equal(t, "already checked in", duplicate["error"])

	onBreak := route.stamp(t, "/staff-clock", map[string]string{
		"rfid_tag": tag,
		"action":   devicescan.StaffClockActionBreakStart,
	})
	assert.Equal(t, devicescan.StaffClockStateOnBreak, onBreak["state"])
	require.NotNil(t, onBreak["active_break"], "a running break is reported")

	resumed := route.stamp(t, "/staff-clock", map[string]string{
		"rfid_tag": tag,
		"action":   devicescan.StaffClockActionBreakEnd,
	})
	assert.Equal(t, devicescan.StaffClockStateCheckedIn, resumed["state"])

	checkedOut := route.stamp(t, "/staff-clock", map[string]string{
		"rfid_tag": tag,
		"action":   devicescan.StaffClockActionCheckOut,
	})
	assert.Equal(t, devicescan.StaffClockStateCheckedOut, checkedOut["state"])

	// Checking in again with a different work location starts a NEW block
	// carrying that status (#2402): one stamp, no conflict, no reason. The
	// first block stays closed with its own status.
	secondBlock := route.stamp(t, "/staff-clock", map[string]string{
		"rfid_tag": tag,
		"action":   devicescan.StaffClockActionCheckIn,
		"status":   workforce.WorkSessionStatusHomeOffice,
	})
	assert.Equal(t, devicescan.StaffClockStateCheckedIn, secondBlock["state"])
	secondBlockSession := secondBlock["session"].(map[string]any)
	assert.Equal(t, workforce.WorkSessionStatusHomeOffice, secondBlockSession["status"])
	assert.Equal(t, workforce.WorkSessionSourceNFC, secondBlockSession["source"])
	assert.NotEqual(t, session["id"], secondBlockSession["id"],
		"the second check-in must create a new block, not reopen the first")

	// A stamp that does not fit the state answers with the same stable code
	// whether the workflow or the work session service rejected it.
	breakEndCode, breakEnd := route.call(t, "/staff-clock", map[string]string{
		"rfid_tag": tag,
		"action":   devicescan.StaffClockActionBreakEnd,
	})
	require.Equal(t, 409, breakEndCode, "response: %v", breakEnd)
	assert.Equal(t, "invalid_staff_clock_state", breakEnd["code"])
	assert.Equal(t, "no active break found", breakEnd["error"])
}

func TestStaffClock_RejectsStudentCard(t *testing.T) {
	t.Parallel()

	_, route := setupStaffClockRoute(t)
	tag := route.linkStudentCard(t, "Sam", "Schueler", "B1654CAFE")

	code, parsed := route.call(t, "/staff-clock/state", map[string]string{"rfid_tag": tag})
	assert.Equal(t, 409, code)
	assert.Equal(t, "rfid_tag_not_staff", parsed["code"])
}

func TestStaffClock_ClassifiesCardAndRequestFailures(t *testing.T) {
	t.Parallel()

	_, route := setupStaffClockRoute(t)

	code, parsed := route.call(t, "/staff-clock/state", map[string]string{"rfid_tag": "not a tag"})
	assert.Equal(t, 400, code)
	assert.Equal(t, "invalid_rfid_tag", parsed["code"])

	code, parsed = route.call(t, "/staff-clock/state", map[string]string{"rfid_tag": "C1654FEED"})
	assert.Equal(t, 404, code)
	assert.Equal(t, "rfid_tag_not_found", parsed["code"])

	code, parsed = route.call(t, "/staff-clock", map[string]string{"rfid_tag": "C1654FEED", "action": "dance"})
	assert.Equal(t, 400, code)
	assert.Equal(t, "invalid_staff_clock_request", parsed["code"])

	code, parsed = route.call(t, "/staff-clock", map[string]string{"rfid_tag": "C1654FEED", "action": devicescan.StaffClockActionCheckIn})
	assert.Equal(t, 400, code)
	assert.Equal(t, "invalid_staff_clock_request", parsed["code"])
	assert.Equal(t, "status is required for check-in", parsed["error"])
}
