// group_access_changed emission tests for every group-side write (#2084).
//
// Which groups a staff member may open is the union of education.group_teacher
// and education.group_substitution (usercontext.GetMyGroups). The flows below
// change that set without the affected colleague's client making any
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
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setupRecordingRouter mounts the production router with a recording
// broadcaster on the education service, mirroring the duck-typed
// SetBroadcaster block in services/factory.go.
func setupRecordingRouter(t *testing.T) (*testContext, chi.Router, *testpkg.RecordingBroadcaster) {
	t.Helper()

	tc, router := setupProtectedRouter(t)
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

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, label+"Leader", "Teacher")

	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	return group.ID, account.ID
}

func TestUpdateGroupTeachers_BroadcastsGroupAccessChanged(t *testing.T) {
	t.Parallel()

	tc, router, broadcaster := setupRecordingRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "SSEGroupTeachers")

	teacher := testpkg.CreateTestTeacher(t, tc.db, "SSEAssigned", "Teacher")

	body := map[string]interface{}{
		"name":        fmt.Sprintf("SSEGroupTeachers-%d", group.ID),
		"teacher_ids": []int64{teacher.ID},
	}
	claims := testutil.DefaultTestClaims()
	claims.TenantID = testpkg.RebaseTenantID(t, claims.TenantID)
	req := newReq(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body, claims, "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	assertGroupAccessChanged(t, broadcaster, "group_teachers", int64(claims.TenantID))
}

// The group form submits teacher_ids on every save, so a rename must not make
// every client in the school refetch its group list.
func TestUpdateGroupTeachers_Unchanged_BroadcastsNothing(t *testing.T) {
	t.Parallel()

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

// A failed teacher-set update can happen after old links were deleted and new
// links were inserted. The handler must reject and roll back the whole request;
// a 4xx response alone would make TenantTxMiddleware commit those partial
// writes without an event.
func TestUpdateGroupTeachers_PartialFailureRollsBack(t *testing.T) {
	t.Parallel()

	tc, router, broadcaster := setupRecordingRouter(t)
	groupID, _ := groupLeader(t, tc, "SSETeacherRollback")

	ctx := testpkg.Ctx(t)
	originalGroup, err := tc.services.Education.GetGroup(ctx, groupID)
	require.NoError(t, err)
	originalTeachers, err := tc.services.Education.GetGroupTeachers(ctx, groupID)
	require.NoError(t, err)
	require.Len(t, originalTeachers, 1)

	candidate := testpkg.CreateTestTeacher(t, tc.db, "SSERollback", "Candidate")

	missing := testpkg.CreateTestTeacher(t, tc.db, "SSERollback", "Missing")
	missingID := missing.ID
	// Deleting the second teacher is the ARRANGE step: the request must fail
	// on the missing ID and roll back. Not a teardown (#2419).
	testpkg.CleanupTeacherFixtures(t, tc.db, missingID)

	body := map[string]interface{}{
		"name":        fmt.Sprintf("SSETeacherRollback-renamed-%d", groupID),
		"teacher_ids": []int64{candidate.ID, missingID},
	}
	req := newReq(t, "PUT", fmt.Sprintf("/groups/%d", groupID), body, testutil.DefaultTestClaims(), "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)

	updatedGroup, err := tc.services.Education.GetGroup(ctx, groupID)
	require.NoError(t, err)
	assert.Equal(t, originalGroup.Name, updatedGroup.Name, "the group update must roll back with its teacher set")

	updatedTeachers, err := tc.services.Education.GetGroupTeachers(ctx, groupID)
	require.NoError(t, err)
	require.Len(t, updatedTeachers, 1)
	assert.Equal(t, originalTeachers[0].ID, updatedTeachers[0].ID)
	assert.False(t, broadcaster.HasEventType(realtime.EventGroupAccessChanged),
		"a rolled-back teacher-set update must not announce a change")
}

func TestCreateGroupTeachers_PartialFailureRollsBack(t *testing.T) {
	t.Parallel()

	tc, router, broadcaster := setupRecordingRouter(t)

	candidate := testpkg.CreateTestTeacher(t, tc.db, "SSECreateRollback", "Candidate")

	missing := testpkg.CreateTestTeacher(t, tc.db, "SSECreateRollback", "Missing")
	missingID := missing.ID
	// Deleting the second teacher is the ARRANGE step: the request must fail
	// on the missing ID and roll back. Not a teardown (#2419).
	testpkg.CleanupTeacherFixtures(t, tc.db, missingID)

	groupName := fmt.Sprintf("SSECreateRollback-%d", candidate.ID)
	claims := testutil.DefaultTestClaims()
	claims.TenantID = testpkg.RebaseTenantID(t, claims.TenantID)
	body := map[string]interface{}{
		"name":        groupName,
		"teacher_ids": []int64{candidate.ID, missingID},
	}
	req := newReq(t, "POST", "/groups", body, claims, "groups:create")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)

	groupCount, err := tc.services.Education.CountGroups(
		testpkg.Ctx(t),
		&educationModels.GroupListQuery{Name: groupName},
	)
	require.NoError(t, err)
	assert.Zero(t, groupCount, "the new group must roll back with its partial teacher set")
	assert.False(t, broadcaster.HasEventType(realtime.EventGroupAccessChanged),
		"a rolled-back group creation must not announce a change")
}

// Deleting a group revokes access for its leaders. Legacy personnel
// substitutions still cascade; typed group handovers must be ended first.
func TestDeleteGroup_BroadcastsGroupAccessChanged(t *testing.T) {
	t.Parallel()

	tc, router, broadcaster := setupRecordingRouter(t)
	groupID, _ := groupLeader(t, tc, "SSEDeleteGroup")

	substituteStaff := testpkg.CreateTestStaff(t, tc.db, "SSEDeleteGroupSub", "Staff")

	today := timezone.TodayDate()
	regularStaffID := substituteStaff.ID
	testpkg.CreateTestGroupSubstitution(t, tc.db, groupID, &regularStaffID, substituteStaff.ID, today, today)

	claims := testutil.DefaultTestClaims()
	claims.TenantID = testpkg.RebaseTenantID(t, claims.TenantID)
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
	assert.True(t, sources["group_delete"], "the deleted group list entry must be announced")
}

func TestDeleteEmptyGroup_BroadcastsGroupAccessChanged(t *testing.T) {
	t.Parallel()

	tc, router, broadcaster := setupRecordingRouter(t)
	group := testpkg.CreateTestEducationGroup(t, tc.db, "SSEDeleteEmptyGroup")

	claims := testutil.DefaultTestClaims()
	claims.TenantID = testpkg.RebaseTenantID(t, claims.TenantID)
	req := newReq(t, "DELETE", fmt.Sprintf("/groups/%d", group.ID), nil, claims, "groups:delete")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	assertGroupAccessChanged(t, broadcaster, "group_delete", int64(claims.TenantID))
}

// A rejected handover writes nothing, so it must announce nothing: every
// client of the school would otherwise pay a refetch for a non-change.
