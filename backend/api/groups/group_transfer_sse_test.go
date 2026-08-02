// Group-handover SSE emission tests (#2084).
//
// A Gruppenübergabe grants a colleague access to a group for the rest of the
// day. The receiving colleague's client makes no request of its own, and a
// handover changes no attendance, activity or timetable state — so none of the
// existing events fire. Without substitution_changed their open "Meine Gruppe"
// tab keeps the old group list until a reload.
package groups_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setupTransferRouterWithBroadcaster mounts the production transfer routes and
// attaches a recording broadcaster. Router() captures the resource pointer, so
// assigning after the mount still reaches every handler call.
func setupTransferRouterWithBroadcaster(t *testing.T) (*testContext, chi.Router, *testpkg.RecordingBroadcaster) {
	t.Helper()

	tc, router := setupTransferRouter(t)
	broadcaster := testpkg.NewRecordingBroadcaster()
	tc.resource.Broadcaster = broadcaster

	return tc, router, broadcaster
}

func assertSubstitutionChangedEvent(t *testing.T, broadcaster *testpkg.RecordingBroadcaster, source string) {
	t.Helper()

	events := broadcaster.EventsOfType(realtime.EventSubstitutionChanged)
	require.Len(t, events, 1, "expected exactly one substitution_changed event")

	require.NotNil(t, events[0].Data.Source)
	assert.Equal(t, source, *events[0].Data.Source)
	// The event reaches every staff client of the tenant, so naming the
	// substitute would tell colleagues outside the group who stands in.
	assert.Nil(t, events[0].Data.SupervisorIDs)
	assert.Empty(t, events[0].ActiveGroupID)

	// Routed to the acting teacher's tenant, never across tenants.
	calls := broadcaster.CallsByMethod("tenant")
	require.NotEmpty(t, calls)
	assert.Equal(t, int64(testutil.TeacherTestClaims(0).TenantID), calls[0].TenantID)
}

func TestTransferGroup_BroadcastsSubstitutionChanged(t *testing.T) {
	tc, router, broadcaster := setupTransferRouterWithBroadcaster(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "SSETransferTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "SSELeader", "Teacher")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	targetStaff := testpkg.CreateTestStaff(t, tc.db, "SSETarget", "Staff")
	defer testpkg.CleanupStaffFixtures(t, tc.db, targetStaff.ID)

	body := map[string]interface{}{"target_user_id": targetStaff.Person.ID}
	req := newReq(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body, testutil.TeacherTestClaims(int(account.ID)))

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	assertSubstitutionChangedEvent(t, broadcaster, "group_transfer")
}

func TestCancelTransfer_BroadcastsSubstitutionChanged(t *testing.T) {
	tc, router, broadcaster := setupTransferRouterWithBroadcaster(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "SSECancelTransferTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "SSECancelLeader", "Teacher")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	targetStaff := testpkg.CreateTestStaff(t, tc.db, "SSECancelTarget", "Staff")
	defer testpkg.CleanupStaffFixtures(t, tc.db, targetStaff.ID)

	today := timezone.TodayDate()
	transfer := testpkg.CreateTestGroupSubstitution(t, tc.db, group.ID, nil, targetStaff.ID, today, today)
	defer testpkg.CleanupActivityFixtures(t, tc.db, transfer.ID)

	req := newReq(t, "DELETE", fmt.Sprintf("/groups/%d/transfer/%d", group.ID, transfer.ID), nil, testutil.TeacherTestClaims(int(account.ID)))

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	assertSubstitutionChangedEvent(t, broadcaster, "group_transfer_cancel")
}

// A rejected handover writes nothing, so it must announce nothing: every
// client of the school would otherwise pay a refetch for a non-change.
func TestTransferGroup_NotGroupLeader_BroadcastsNothing(t *testing.T) {
	tc, router, broadcaster := setupTransferRouterWithBroadcaster(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "SSENoLeaderTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Teacher deliberately NOT assigned to the group.
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "SSENoLeader", "Teacher")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	targetStaff := testpkg.CreateTestStaff(t, tc.db, "SSENoLeaderTarget", "Staff")
	defer testpkg.CleanupStaffFixtures(t, tc.db, targetStaff.ID)

	body := map[string]interface{}{"target_user_id": targetStaff.Person.ID}
	req := newReq(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body, testutil.TeacherTestClaims(int(account.ID)))

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)

	assert.False(t, broadcaster.HasEventType(realtime.EventSubstitutionChanged))
}
