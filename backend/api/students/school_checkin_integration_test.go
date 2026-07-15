package students_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests for POST /api/students/{id}/school-checkin.
// Follow the codebase convention: real DB via testpkg.SetupTestDB, real
// fixtures, external _test package. These fill the coverage gap on the
// HTTP-handler functions (schoolCheckinHandler, applySchoolCheckinAction,
// enforceWebCheckinAccess) that can't be exercised with unit-level mocks
// because they depend on the full active.Service surface.

// bytesReader is a local convenience wrapper — the existing helpers pass
// nil for GET bodies, but our POST cases need a real reader.
func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

// setAllStaffAccess overrides attendance.web_checkin_access so the test
// caller isn't forced through the group-supervisor check. Cleanup resets
// to the registry default automatically.
func setAllStaffAccess(t *testing.T, tc *testContext) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	err := tc.services.Settings.SetValue(
		ctx,
		configModel.KeyWebCheckinAccess,
		configModel.WebCheckinAccessAllStaff,
		nil,
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(ctx, configModel.KeyWebCheckinAccess, nil, nil)
	})
}

// NOTE on fixtures: the handler resolves JWT claims through
// PersonService.FindByAccountID → Staff lookup, so tests must create the
// full chain with CreateTestStaffWithAccount rather than the bare
// CreateTestStaff helper. Pass `account.ID` (not `staff.ID`) into
// testutil.AdminTestClaims so claims.ID matches what the middleware expects.

func TestSchoolCheckin_CheckIn_NewAttendance(t *testing.T) {
	tc := setupTestContext(t)
	setAllStaffAccess(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "Checkin", "Target", "1a")
	// Full chain: caller must have a resolvable account → person → staff
	// because the handler resolves the JWT claims through PersonService.
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Checkin", "Caller")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID, staff.PersonID)
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	bodyBytes, _ := json.Marshal(map[string]string{"action": "in"})
	req := testutil.NewRequest("POST", fmt.Sprintf("/%d/school-checkin", student.ID), bytesReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	var resp struct {
		Data struct {
			Status   string `json:"status"`
			Location string `json:"location"`
			Changed  bool   `json:"changed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "checked_in", resp.Data.Status)
	assert.Equal(t, "Anwesend", resp.Data.Location)
	assert.True(t, resp.Data.Changed)
}

func TestSchoolCheckin_CheckIn_Idempotent(t *testing.T) {
	tc := setupTestContext(t)
	setAllStaffAccess(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "Idem", "Target", "1a")
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Idem", "Caller")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID, staff.PersonID)
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	bodyBytes, _ := json.Marshal(map[string]string{"action": "in"})

	// First call creates the row.
	req1 := testutil.NewRequest("POST", fmt.Sprintf("/%d/school-checkin", student.ID), bytesReader(bodyBytes))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := authExec(t, tc, req1, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr1.Code, "body: %s", rr1.Body.String())

	// Second call with identical action should be a no-op.
	req2 := testutil.NewRequest("POST", fmt.Sprintf("/%d/school-checkin", student.ID), bytesReader(bodyBytes))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := authExec(t, tc, req2, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr2.Code)
	var resp struct {
		Data struct {
			Changed bool `json:"changed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr2.Body.Bytes(), &resp))
	assert.False(t, resp.Data.Changed, "second check-in on already-present student must be no-op")
}

func TestSchoolCheckin_CheckOut_FromCheckedIn(t *testing.T) {
	tc := setupTestContext(t)
	setAllStaffAccess(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "Checkout", "Target", "2b")
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Checkout", "Caller")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID, staff.PersonID)
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	// Check in first.
	bodyIn, _ := json.Marshal(map[string]string{"action": "in"})
	reqIn := testutil.NewRequest("POST", fmt.Sprintf("/%d/school-checkin", student.ID), bytesReader(bodyIn))
	reqIn.Header.Set("Content-Type", "application/json")
	rrIn := authExec(t, tc, reqIn, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rrIn.Code, "check-in body: %s", rrIn.Body.String())

	// Now check out.
	bodyOut, _ := json.Marshal(map[string]string{"action": "out"})
	reqOut := testutil.NewRequest("POST", fmt.Sprintf("/%d/school-checkin", student.ID), bytesReader(bodyOut))
	reqOut.Header.Set("Content-Type", "application/json")
	rrOut := authExec(t, tc, reqOut, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rrOut.Code, "check-out body: %s", rrOut.Body.String())
	var resp struct {
		Data struct {
			Status   string `json:"status"`
			Location string `json:"location"`
			Changed  bool   `json:"changed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rrOut.Body.Bytes(), &resp))
	assert.Equal(t, "checked_out", resp.Data.Status)
	assert.Equal(t, "Abwesend", resp.Data.Location)
	assert.True(t, resp.Data.Changed)
}

// TestSchoolCheckin_CheckOut_AlsoEndsOpenVisit covers the detailed-mode
// cleanup branch in applySchoolCheckinAction: when the student has an open
// active.visits row, web checkout must end the visit too so supervisor
// views don't keep showing "still in Room X" after attendance is closed.
//
// This branch was previously uncovered because TestSchoolCheckin_CheckOut_FromCheckedIn
// goes through "in" via the school-checkin endpoint, which only writes
// attendance, never a visit. The visit must come from a separate kiosk-side
// fixture for this branch to fire.
func TestSchoolCheckin_CheckOut_AlsoEndsOpenVisit(t *testing.T) {
	tc := setupTestContext(t)
	setAllStaffAccess(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "Visit", "Cleanup", "2c")
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Visit", "Caller")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID, staff.PersonID)
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	// Activity infrastructure for the visit. The kiosk path writes
	// active.visits directly via CreateVisit; we shortcut with the test
	// fixture so the integration test stays focused on the handler.
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "School Visit Cleanup")
	room := testpkg.CreateTestRoom(t, tc.db, "Visit Cleanup Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, activityGroup.ID, room.ID, activeGroup.ID)

	// Open visit + open attendance — same student, both still active.
	entryTime := time.Now().Add(-30 * time.Minute)
	visit := testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, entryTime, nil)
	defer testpkg.CleanupTableRecords(t, tc.db, "active.visits", visit.ID)

	device := testpkg.CreateTestDevice(t, tc.db, "school-checkin-visit-device")
	defer testpkg.CleanupActivityFixtures(t, tc.db, device.ID)
	checkInTime := time.Now().Add(-30 * time.Minute)
	att := testpkg.CreateTestAttendance(t, tc.db, student.ID, staff.ID, device.ID, checkInTime, nil)
	defer testpkg.CleanupTableRecords(t, tc.db, "active.attendance", att.ID)

	// Web checkout — must close attendance AND end the open visit.
	body, _ := json.Marshal(map[string]string{"action": "out"})
	req := testutil.NewRequest("POST", fmt.Sprintf("/%d/school-checkin", student.ID), bytesReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	// Confirm the visit is now closed (ExitTime set).
	updatedVisit, err := tc.services.Active.GetVisit(t.Context(), visit.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedVisit)
	assert.NotNil(t, updatedVisit.ExitTime, "open visit must be closed after web checkout")
}

func TestSchoolCheckin_GroupSupervisors_DeniesNonSupervisor(t *testing.T) {
	tc := setupTestContext(t)
	// Default setting is group_supervisors — don't override, don't grant.

	student := testpkg.CreateTestStudent(t, tc.db, "Denied", "Target", "3c")
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Denied", "Caller")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID, staff.PersonID)
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	bodyBytes, _ := json.Marshal(map[string]string{"action": "in"})
	req := testutil.NewRequest("POST", fmt.Sprintf("/%d/school-checkin", student.ID), bytesReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	// Non-admin claim so the users:checkin permission flows through the
	// attendance.web_checkin_access gate. Admins (admin:* / *:*) now
	// short-circuit the gate and would always pass — to test the
	// supervisor-only branch we have to drop the admin wildcard.
	claims := testutil.AdminTestClaims(int(account.ID))
	claims.Roles = []string{"user"}
	claims.IsAdmin = false
	claims.Permissions = []string{"users:checkin"}

	rr := authExec(t, tc, req, claims, []string{"users:checkin"})

	// The caller is not a supervisor of this student's group → 403 with
	// the specific denial reason (not the "person not found" auth error
	// we'd see if the fixture chain was incomplete).
	require.Equal(t, http.StatusForbidden, rr.Code, "body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "not a supervisor")
}

func TestSchoolCheckin_InvalidAction_Rejects(t *testing.T) {
	tc := setupTestContext(t)
	setAllStaffAccess(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "Invalid", "Target", "4d")
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Invalid", "Caller")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID, staff.PersonID)
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	bodyBytes, _ := json.Marshal(map[string]string{"action": "toggle"}) // not in | out
	req := testutil.NewRequest("POST", fmt.Sprintf("/%d/school-checkin", student.ID), bytesReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	// Body is JSON with escaped quotes (`\"in\"` not `"in"`), so match on
	// the unambiguous prefix rather than the full sentence.
	assert.Contains(t, rr.Body.String(), "action must be")
}
