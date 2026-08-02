// group_access_changed emission tests for every group-side write (#2084).
//
// Which groups a staff member may open is the union of education.group_teacher
// and education.group_substitution (usercontext.GetMyGroups). All four flows
// below change that set without the affected colleague's client making any
// request, and none of them produces a check-in, activity or timetable event —
// so without this signal their open tab keeps the stale group list.
//
// Driven through the production router even though the emit lives in
// services/education: the subtle part is that the broadcast fires only after
// the request's tenant transaction COMMITS, which only the full chain proves.
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

// setupRecordingRouter mounts the production router with a recording
// broadcaster on the education service, mirroring the duck-typed
// SetBroadcaster block in services/factory.go.
func setupRecordingRouter(t *testing.T) (*testContext, chi.Router, *testpkg.RecordingBroadcaster) {
	t.Helper()

	tc, router := setupTransferRouter(t)
	broadcaster := testpkg.NewRecordingBroadcaster()

	aware, ok := tc.services.Education.(interface {
		SetBroadcaster(realtime.Broadcaster)
	})
	require.True(t, ok, "education service must accept a broadcaster")
	aware.SetBroadcaster(broadcaster)

	return tc, router, broadcaster
}

// assertGroupAccessChanged pins the tenant routing and privacy contract via the
// shared helper, then the emitting flow.
func assertGroupAccessChanged(t *testing.T, b *testpkg.RecordingBroadcaster, source string, tenantID int64) {
	t.Helper()

	event := testpkg.AssertSingleTenantEvent(t, b, realtime.EventGroupAccessChanged, tenantID)
	require.NotNil(t, event.Data.Source)
	assert.Equal(t, source, *event.Data.Source)
}

// groupLeader creates a group with an assigned teacher who owns an account, so
// the ownership-based transfer authorization passes.
func groupLeader(t *testing.T, tc *testContext, label string) (groupID int64, accountID int64) {
	t.Helper()

	group := testpkg.CreateTestEducationGroup(t, tc.db, label)
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, tc.db, group.ID) })

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, label+"Leader", "Teacher")
	t.Cleanup(func() { testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID) })
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, tc.db, account.ID) })

	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	return group.ID, account.ID
}

func TestTransferGroup_BroadcastsGroupAccessChanged(t *testing.T) {
	tc, router, broadcaster := setupRecordingRouter(t)
	groupID, accountID := groupLeader(t, tc, "SSETransfer")

	targetStaff := testpkg.CreateTestStaff(t, tc.db, "SSETarget", "Staff")
	defer testpkg.CleanupStaffFixtures(t, tc.db, targetStaff.ID)

	body := map[string]interface{}{"target_user_id": targetStaff.Person.ID}
	claims := testutil.TeacherTestClaims(int(accountID))
	req := newReq(t, "POST", fmt.Sprintf("/groups/%d/transfer", groupID), body, claims)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	assertGroupAccessChanged(t, broadcaster, "group_transfer", int64(claims.TenantID))
}

func TestCancelTransfer_BroadcastsGroupAccessChanged(t *testing.T) {
	tc, router, broadcaster := setupRecordingRouter(t)
	groupID, accountID := groupLeader(t, tc, "SSECancel")

	targetStaff := testpkg.CreateTestStaff(t, tc.db, "SSECancelTarget", "Staff")
	defer testpkg.CleanupStaffFixtures(t, tc.db, targetStaff.ID)

	today := timezone.TodayDate()
	transfer := testpkg.CreateTestGroupSubstitution(t, tc.db, groupID, nil, targetStaff.ID, today, today)
	defer testpkg.CleanupActivityFixtures(t, tc.db, transfer.ID)

	claims := testutil.TeacherTestClaims(int(accountID))
	req := newReq(t, "DELETE", fmt.Sprintf("/groups/%d/transfer/%d", groupID, transfer.ID), nil, claims)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Reported as substitution_delete, not as a distinct cancel label: both the
	// handover UI and the Vertretungsplan reach the same DeleteSubstitution, and
	// the service cannot see which caller it was. Accepted deliberately — source
	// only feeds log review, and threading a label through the service signature
	// would buy nothing a client acts on.
	assertGroupAccessChanged(t, broadcaster, "substitution_delete", int64(claims.TenantID))
}

// Group leadership is the OTHER half of the access union. An admin adding a
// colleague to a group in the Gruppenverwaltung produces exactly the
// invisibility a handover does, so it rides the same event.
func TestUpdateGroupTeachers_BroadcastsGroupAccessChanged(t *testing.T) {
	tc, router, broadcaster := setupRecordingRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "SSEGroupTeachers")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	teacher := testpkg.CreateTestTeacher(t, tc.db, "SSEAssigned", "Teacher")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)

	body := map[string]interface{}{
		"name":        fmt.Sprintf("SSEGroupTeachers-%d", group.ID),
		"teacher_ids": []int64{teacher.ID},
	}
	claims := testutil.DefaultTestClaims()
	req := newReq(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body, claims, "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	assertGroupAccessChanged(t, broadcaster, "group_teachers", int64(claims.TenantID))
}

// The group form submits teacher_ids on every save, so a rename must not make
// every client in the school refetch its group list.
func TestUpdateGroupTeachers_Unchanged_BroadcastsNothing(t *testing.T) {
	tc, router, broadcaster := setupRecordingRouter(t)
	groupID, _ := groupLeader(t, tc, "SSESameTeachers")

	teachers, err := tc.services.Education.GetGroupTeachers(t.Context(), groupID)
	require.NoError(t, err)
	require.Len(t, teachers, 1)

	body := map[string]interface{}{
		"name":        fmt.Sprintf("SSESameTeachers-renamed-%d", groupID),
		"teacher_ids": []int64{teachers[0].ID},
	}
	req := newReq(t, "PUT", fmt.Sprintf("/groups/%d", groupID), body, testutil.DefaultTestClaims(), "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	assert.False(t, broadcaster.HasEventType(realtime.EventGroupAccessChanged),
		"an unchanged teacher set must not announce a group-access change")
}

// Deleting a group revokes access for its leaders AND its substitutes. The
// cascade runs inside services/education, so it would have been invisible to a
// handler-level emit. It legitimately announces once per removed link — the
// client debounce collapses the burst into one refetch.
func TestDeleteGroup_BroadcastsGroupAccessChanged(t *testing.T) {
	tc, router, broadcaster := setupRecordingRouter(t)
	groupID, _ := groupLeader(t, tc, "SSEDeleteGroup")

	substituteStaff := testpkg.CreateTestStaff(t, tc.db, "SSEDeleteGroupSub", "Staff")
	defer testpkg.CleanupStaffFixtures(t, tc.db, substituteStaff.ID)

	today := timezone.TodayDate()
	testpkg.CreateTestGroupSubstitution(t, tc.db, groupID, nil, substituteStaff.ID, today, today)

	claims := testutil.DefaultTestClaims()
	req := newReq(t, "DELETE", fmt.Sprintf("/groups/%d", groupID), nil, claims, "groups:delete")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	events := broadcaster.EventsOfType(realtime.EventGroupAccessChanged)
	require.NotEmpty(t, events, "deleting a group revokes access and must announce it")

	sources := make(map[string]bool, len(events))
	for _, event := range events {
		require.NotNil(t, event.Data.Source)
		sources[*event.Data.Source] = true
	}
	assert.True(t, sources["group_teacher_remove"], "the removed leader link must be announced")
	assert.True(t, sources["substitution_delete"], "the removed substitution must be announced")
}

// A rejected handover writes nothing, so it must announce nothing: every
// client of the school would otherwise pay a refetch for a non-change.
func TestTransferGroup_NotGroupLeader_BroadcastsNothing(t *testing.T) {
	tc, router, broadcaster := setupRecordingRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "SSENoLeader")
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

	assert.False(t, broadcaster.HasEventType(realtime.EventGroupAccessChanged))
}
