package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type staticParentRequestReviewAccess string

func (access staticParentRequestReviewAccess) AccessLevel(context.Context, []string) (string, error) {
	return string(access), nil
}

func (access staticParentRequestReviewAccess) Scope(context.Context, []string) (bool, []int64, error) {
	return access != "none", nil, nil
}

func (access staticParentRequestReviewAccess) AbsenceScope(context.Context, []string) (bool, []int64, error) {
	return access != "none", nil, nil
}

func TestChangeRequestAccessReportsGroupLeaderCapabilityFromPolicy(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Access", "Reviewer")
	tc.resource.RequestReviewAccess = staticParentRequestReviewAccess("group_leader")

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := authExec(
		t,
		tc,
		testutil.NewRequest("GET", "/change-requests/access", nil),
		claims,
		[]string{"users:read", "users:update"},
	)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var envelope struct {
		Data struct {
			ReviewAccess string `json:"review_access"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
	assert.Equal(t, "group_leader", envelope.Data.ReviewAccess)
}

func TestChangeRequestAccessReportsNoneFromPolicy(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Access", "No Group")
	tc.resource.RequestReviewAccess = staticParentRequestReviewAccess("none")

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := authExec(
		t,
		tc,
		testutil.NewRequest("GET", "/change-requests/access", nil),
		claims,
		[]string{"users:read", "users:update"},
	)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var envelope struct {
		Data struct {
			ReviewAccess string `json:"review_access"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
	assert.Equal(t, "none", envelope.Data.ReviewAccess)
}

type studentReviewCoverage struct {
	ReviewAccess string `json:"review_access"`
	Student      *struct {
		Requests bool `json:"requests"`
		Absences bool `json:"absences"`
	} `json:"student"`
}

// #3886: a message thread offers "Anfrage ansehen" only when the review scope
// reaches the thread's child. Staff read every thread of the school, but the
// request detail answers 403 outside the scope.
func TestChangeRequestAccessReportsStudentCoverage(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	leader, leaderAccount := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Coverage", "Leitung")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "Abdeckung")
	otherGroup := testpkg.CreateTestEducationGroup(t, tc.db, "AbdeckungFremd")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, leader.ID)
	own := testpkg.CreateTestStudent(t, tc.db, "Eigen", "Abdeckung", "AB1")
	testpkg.AssignStudentToGroup(t, tc.db, own.ID, group.ID)
	foreign := testpkg.CreateTestStudent(t, tc.db, "Fremd", "Abdeckung", "AB1")
	testpkg.AssignStudentToGroup(t, tc.db, foreign.ID, otherGroup.ID)

	perms := []string{"users:read", "users:update"}
	leaderClaims := testutil.TeacherTestClaims(int(leaderAccount.ID))
	coverage := func(claims jwt.AppClaims, studentID int64) studentReviewCoverage {
		t.Helper()
		rr := authExec(t, tc, testutil.NewRequest("GET", fmt.Sprintf("/change-requests/access?student_id=%d", studentID), nil), claims, perms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var envelope struct {
			Data studentReviewCoverage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
		require.NotNil(t, envelope.Data.Student, "student_id answers the coverage")
		return envelope.Data
	}
	setSetting := func(key string, value any) {
		t.Helper()
		require.NoError(t, tc.settings().SetValue(testpkg.Ctx(t), key, value, nil, nil))
	}

	// Default: the group-leader switch is off, so staff review nothing.
	got := coverage(leaderClaims, own.ID)
	assert.Equal(t, "none", got.ReviewAccess)
	assert.False(t, got.Student.Requests)
	assert.False(t, got.Student.Absences)

	// Switch on: own groups only.
	setSetting(settings.KeyParentRequestGroupLeaderReviewEnabled, true)
	got = coverage(leaderClaims, own.ID)
	assert.True(t, got.Student.Requests)
	assert.True(t, got.Student.Absences, "the absence scope inherits the request scope")
	got = coverage(leaderClaims, foreign.ID)
	assert.False(t, got.Student.Requests)
	assert.False(t, got.Student.Absences)

	// The absence scope may differ from the other request kinds.
	setSetting(settings.KeyParentRequestReviewScope, settings.ParentRequestReviewScopeAdmins)
	setSetting(settings.KeyParentAbsenceReviewScope, settings.ParentRequestReviewScopeAllStaff)
	got = coverage(leaderClaims, foreign.ID)
	assert.False(t, got.Student.Requests)
	assert.True(t, got.Student.Absences)

	// Administrators reach every child.
	got = coverage(testutil.AdminTestClaims(int(leaderAccount.ID)), foreign.ID)
	assert.True(t, got.Student.Requests)
	assert.True(t, got.Student.Absences)

	// Without student_id the response keeps its old shape.
	rr := authExec(t, tc, testutil.NewRequest("GET", "/change-requests/access", nil), leaderClaims, perms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.NotContains(t, rr.Body.String(), `"student"`)
}

func TestChangeRequestAccessRejectsInvalidStudentID(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Coverage", "Invalid")
	rr := authExec(t, tc, testutil.NewRequest("GET", "/change-requests/access?student_id=abc", nil),
		testutil.TeacherTestClaims(int(account.ID)), []string{"users:read", "users:update"})
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}
