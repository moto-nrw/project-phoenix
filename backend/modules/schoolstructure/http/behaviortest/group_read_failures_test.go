package schoolstructurehttp_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	schoolHTTP "github.com/moto-ogs/project-phoenix/modules/schoolstructure/http"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// failingAttendanceReads fails the batched attendance read the location
// snapshot needs, as an aborted transaction does.
type failingAttendanceReads struct {
	studentpresence.Presence
}

func (failingAttendanceReads) GetStudentsAttendanceStatuses(context.Context, []int64) (map[int64]*studentpresence.DailyAttendanceStatus, error) {
	return nil, errors.New("attendance unavailable")
}

// failingGroupChildren fails the read of a group's children.
type failingGroupChildren struct {
	schoolHTTP.GroupPeople
}

func (failingGroupChildren) GroupStudents(context.Context, int64) ([]peopledirectory.StudentRecord, error) {
	return nil, errors.New("children unavailable")
}

// A failed presence read is an unknown state, not a confirmed absence: the
// group list answers 500 instead of every child as "Abwesend" (#6849).
func TestGetGroupStudents_PresenceReadFailureIsAnError(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	group := testpkg.CreateTestEducationGroup(t, tc.db, "PresenceFailure")
	child := testpkg.CreateTestStudent(t, tc.db, "Presence", "Failure", "P1")
	testpkg.AssignStudentToGroup(t, tc.db, child.ID, group.ID)
	tc.resource.ActiveService = failingAttendanceReads{Presence: tc.resource.ActiveService}

	rr := testutil.ExecuteRequest(router, newReq(t, http.MethodGet,
		fmt.Sprintf("/groups/%d/students", group.ID), nil, testutil.DefaultTestClaims(), "groups:read"))
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

// A failed children read is no empty group: the group answers 500 instead of
// a student count of 0 (#6849).
func TestGetGroup_ChildrenReadFailureIsAnError(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	group := testpkg.CreateTestEducationGroup(t, tc.db, "CountFailure")
	tc.resource.People = failingGroupChildren{GroupPeople: tc.resource.People}

	rr := testutil.ExecuteRequest(router, newReq(t, http.MethodGet,
		fmt.Sprintf("/groups/%d", group.ID), nil, testutil.DefaultTestClaims(), "groups:read"))
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

// A failed read-back after an update answers 500 and rolls the update back,
// instead of confirming it with a student count of 0 (#6849).
func TestUpdateGroup_ReadBackFailureRollsBack(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	group := testpkg.CreateTestEducationGroup(t, tc.db, "ReadBackFailure")
	tc.resource.People = failingGroupChildren{GroupPeople: tc.resource.People}

	rr := testutil.ExecuteRequest(router, newReq(t, http.MethodPut,
		fmt.Sprintf("/groups/%d", group.ID), map[string]any{"name": fmt.Sprintf("Renamed-%d", group.ID)},
		testutil.DefaultTestClaims(), "groups:update"))
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)

	stored, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), tc.db, tc.resource.EducationService.GetGroup, group.ID)
	require.NoError(t, err)
	require.Equal(t, group.Name, stored.Name)
}
