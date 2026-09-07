package active_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	activeAPI "github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingVisitReadback struct{ activeAPI.PresenceQueries }

func (failingVisitReadback) FindVisit(context.Context, int64) (*studentpresence.Visit, error) {
	return nil, errors.New("private readback failure")
}

func TestVisitCreateReadbackFailureRollsBackAndCanBeRetried(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	queries := tc.resource.Presence
	student := testpkg.CreateTestStudent(t, tc.db, "Create", "Readback", "3a")
	staff := testpkg.CreateTestStaff(t, tc.db, "Create", "Staff")
	device := testpkg.CreateTestDevice(t, tc.db, "create-readback")
	group := testpkg.CreateTestActiveGroupForTenant(t, tc.db, testpkg.Tenant(t))
	tc.resource.Presence = failingVisitReadback{PresenceQueries: queries}
	request := func() int {
		t.Helper()
		req := testutil.NewJSONRequest(t, http.MethodPost, "/active/visits", map[string]any{
			"student_id": student.ID, "active_group_id": group.ID, "check_in_time": time.Now(),
		})
		testutil.WithDeviceContext(device)(req)
		testutil.WithStaffContext(staff)(req)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(1), []string{permissions.GroupsCreate})
		assert.NotContains(t, rr.Body.String(), "private readback failure")
		return rr.Code
	}
	require.Equal(t, http.StatusInternalServerError, request())
	visits, err := queries.ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	assert.Empty(t, visits, "failed readback must roll back the visit")
	status, err := tc.resource.ActiveService.GetStudentAttendanceStatus(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	assert.Equal(t, "not_checked_in", status.Status, "failed readback must roll back attendance too")
	tc.resource.Presence = queries
	require.Equal(t, http.StatusCreated, request())
	visits, err = queries.ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Len(t, visits, 1)
}

func TestVisitEndReadbackFailureRollsBackAndCanBeRetried(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	queries := tc.resource.Presence
	student := testpkg.CreateTestStudent(t, tc.db, "Visit", "Rollback", "3a")
	activity := testpkg.CreateTestActivityGroup(t, tc.db, "Visit rollback")
	room := testpkg.CreateTestRoom(t, tc.db, "Visit rollback")
	group := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	visit := testpkg.CreateTestVisit(t, tc.db, student.ID, group.ID, time.Now().Add(-time.Hour), nil)
	tc.resource.Presence = failingVisitReadback{PresenceQueries: queries}
	request := func() int {
		t.Helper()
		req := testutil.NewJSONRequest(t, http.MethodPost, fmt.Sprintf("/active/visits/%d/end", visit.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(1), []string{permissions.GroupsUpdate})
		assert.NotContains(t, rr.Body.String(), "private readback failure")
		return rr.Code
	}
	require.Equal(t, http.StatusInternalServerError, request())
	stored, err := queries.FindVisit(testpkg.Ctx(t), visit.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.ExitTime, "the failed response must roll back the authoritative visit write")
	tc.resource.Presence = queries
	require.Equal(t, http.StatusOK, request())
	stored, err = queries.FindVisit(testpkg.Ctx(t), visit.ID)
	require.NoError(t, err)
	assert.NotNil(t, stored.ExitTime)
}
