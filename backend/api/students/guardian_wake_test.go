package students_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStaffCareWrite_WakesChildGuardians is the end-to-end proof for #1725: a
// staff-side care write must wake the child's guardians with a message-
// independent parent_child_updated SSE event, so an open parents-app tab
// refetches the "Heute" pickup/presence tile live. The tenant-wide staff
// broadcasts (student_updated / arrival_schedule_changed) never reach the parent
// SSE stream, so without the guardian fan-out the parent view stays stale until a
// manual reload. One representative write (staff pickup exception) is asserted;
// every staff care-write path calls the same wakeChildGuardians helper.
func TestStaffCareWrite_WakesChildGuardians(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	chain := testpkg.CreateTestParentGuardianChain(t, tc.db)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "PickupWake", "GuardianExc")

	// Isolate the wake triggered by the write below from any setup broadcasts.
	tc.broadcaster.Reset()

	body := map[string]any{
		"exception_date": "2026-05-12",
		"pickup_time":    "13:00",
		"reason":         "Wake check",
	}
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", chain.StudentID), body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

	guardianCalls := tc.broadcaster.CallsByMethod("guardian")
	require.NotEmpty(t, guardianCalls,
		"a staff pickup-exception create must wake the child's guardians (#1725)")

	wokeGuardian := false
	for _, c := range guardianCalls {
		if c.GuardianID == chain.AccountID {
			wokeGuardian = true
			assert.Equal(t, realtime.EventParentChildUpdated, c.Event.Type,
				"the guardian wake must carry a parent_child_updated invalidation")
			assert.Equal(t, chain.TenantID, c.TenantID)
		}
	}
	assert.True(t, wokeGuardian,
		"the child's own guardian account %d must be among the woken guardians", chain.AccountID)
}

// TestStaffStatusUpdate_WakesChildGuardians covers #1725: PUT /students/{id} with
// a sick/excused field persists TODAY's status day — the exact signal the parent
// pickup tile resolves today_absent from — but its after-commit hook otherwise
// emits only the tenant-wide student_updated staff event, which never reaches the
// parents SSE stream. The status write must ALSO wake the child's guardians; a
// plain (non-status) edit must NOT, so a name/notes change never spams parent tabs.
func TestStaffStatusUpdate_WakesChildGuardians(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	chain := testpkg.CreateTestParentGuardianChain(t, tc.db)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "StatusWake", "GuardianUpd")

	// A sick=true update writes today's status day → must wake guardians.
	tc.broadcaster.Reset()
	sick := true
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", chain.StudentID), map[string]any{"sick": sick})
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

	wokeGuardian := false
	for _, c := range tc.broadcaster.CallsByMethod("guardian") {
		if c.GuardianID == chain.AccountID {
			wokeGuardian = true
			assert.Equal(t, realtime.EventParentChildUpdated, c.Event.Type,
				"the guardian wake must carry a parent_child_updated invalidation")
			assert.Equal(t, chain.TenantID, c.TenantID)
		}
	}
	assert.True(t, wokeGuardian,
		"a staff sick/excused update must wake the child's guardian account %d (#1725)", chain.AccountID)

	// A non-status update (supervisor notes) touches nothing parent-visible → no wake.
	tc.broadcaster.Reset()
	notes := "Interner Vermerk"
	req = testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", chain.StudentID), map[string]any{"supervisor_notes": notes})
	rr = authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

	for _, c := range tc.broadcaster.CallsByMethod("guardian") {
		assert.NotEqualf(t, chain.AccountID, c.GuardianID,
			"a non-status student update must not wake guardians (#1725)")
	}
}
