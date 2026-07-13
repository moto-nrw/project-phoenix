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
	tc := setupTestContext(t)

	chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
	defer testpkg.CleanupParentGuardianChain(t, tc.db, chain)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "PickupWake", "GuardianExc")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

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
