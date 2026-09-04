package activities_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func activityStaffClaims(account *auth.Account, accountPermissions ...string) jwt.AppClaims {
	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.Sub = account.Email
	claims.Roles = []string{"user"}
	claims.Permissions = accountPermissions
	claims.IsAdmin = false
	return claims
}

func TestActivityPersonalDataReadsRequirePermission(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)
	const (
		studentFirstName = "RosterSecretFirst"
		studentLastName  = "RosterSecretLast"
		staffFirstName   = "StaffSecretFirst"
		staffLastName    = "StaffSecretLast"
	)
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Private roster")
	student := testpkg.CreateTestStudent(t, ctx.db, studentFirstName, studentLastName, "1a")
	require.NoError(t, ctx.resource.ActivityService.EnrollStudent(testpkg.Ctx(t), activity.ID, student.ID))

	testpkg.CreateTestStaff(t, ctx.db, staffFirstName, staffLastName)
	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "Unauthorised", "Reader")
	claims := activityStaffClaims(account, permissions.ClassDayRead)

	tests := []struct {
		name string
		path string
	}{
		{name: "student roster", path: fmt.Sprintf("/activities/%d/students", activity.ID)},
		{name: "supervisor directory", path: "/activities/supervisors/available"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := testutil.NewAuthenticatedRequest(t, http.MethodGet, tt.path, nil)
			rr := testutil.ExecuteWithAuth(t, ctx.router, req, claims)

			assert.Equal(t, http.StatusForbidden, rr.Code, "body: %s", rr.Body.String())
			assert.NotContains(t, rr.Body.String(), studentFirstName)
			assert.NotContains(t, rr.Body.String(), studentLastName)
			assert.NotContains(t, rr.Body.String(), staffFirstName)
			assert.NotContains(t, rr.Body.String(), staffLastName)
		})
	}
}

func TestActivityRosterReadAllowsAuthorisedStaff(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)

	const (
		studentFirstName = "VisibleFirst"
		studentLastName  = "VisibleLast"
	)
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Authorised roster")
	student := testpkg.CreateTestStudent(t, ctx.db, studentFirstName, studentLastName, "1a")
	require.NoError(t, ctx.resource.ActivityService.EnrollStudent(testpkg.Ctx(t), activity.ID, student.ID))
	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "Authorised", "Reader")

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/activities/%d/students", activity.ID), nil)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, activityStaffClaims(account, permissions.ActivitiesRead))

	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), studentFirstName)
	assert.Contains(t, rr.Body.String(), studentLastName)
}

func TestActivityListAllowsListPermission(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)
	testpkg.CreateTestActivityGroup(t, ctx.db, "List permission")
	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "List", "Reader")

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/activities/", nil)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, activityStaffClaims(account, permissions.ActivitiesList))

	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
}

func TestActivityRosterReadFiltersNonStaffPersonalData(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Filtered roster")
	student := testpkg.CreateTestStudent(t, ctx.db, "FilteredSecretFirst", "FilteredSecretLast", "1a")
	require.NoError(t, ctx.resource.ActivityService.EnrollStudent(testpkg.Ctx(t), activity.ID, student.ID))
	account := testpkg.CreateTestAccount(t, ctx.db, "activity-roster-non-staff")

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/activities/%d/students", activity.ID), nil)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, activityStaffClaims(account, permissions.ActivitiesRead))

	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
	assert.NotContains(t, rr.Body.String(), "FilteredSecretFirst")
	assert.NotContains(t, rr.Body.String(), "FilteredSecretLast")
}

func TestActivitySupervisorDirectoryRequiresAssignPermission(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)

	testpkg.CreateTestStaff(t, ctx.db, "DirectorySecretFirst", "DirectorySecretLast")
	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "ReadOnly", "Staff")
	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/activities/supervisors/available", nil)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, activityStaffClaims(account, permissions.ActivitiesRead))

	assert.Equal(t, http.StatusForbidden, rr.Code, "body: %s", rr.Body.String())
	assert.NotContains(t, rr.Body.String(), "DirectorySecretFirst")
	assert.NotContains(t, rr.Body.String(), "DirectorySecretLast")
}

type activityWriteCase struct {
	name   string
	method string
	path   string
	body   any
}

func createOwnedActivity(t *testing.T, db *bun.DB, ownerID int64, name string) int64 {
	t.Helper()
	activity := testpkg.CreateTestActivityGroup(t, db, name)
	_, err := db.NewUpdate().
		TableExpr(`activities.groups`).
		Set("created_by = ?", ownerID).
		Where("id = ?", activity.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	return activity.ID
}

func supervisorWriteCases(t *testing.T, ctx *testContext, activityID int64) []activityWriteCase {
	t.Helper()
	first := testpkg.CreateTestStaff(t, ctx.db, "Existing", "Supervisor")
	second := testpkg.CreateTestStaff(t, ctx.db, "Second", "Supervisor")
	target := testpkg.CreateTestStaff(t, ctx.db, "TargetSecret", "Supervisor")
	firstSupervisor, err := ctx.resource.ActivityService.AddSupervisor(testpkg.Ctx(t), activityID, first.ID, true)
	require.NoError(t, err)
	secondSupervisor, err := ctx.resource.ActivityService.AddSupervisor(testpkg.Ctx(t), activityID, second.ID, false)
	require.NoError(t, err)

	return []activityWriteCase{
		{name: "assign supervisor", method: http.MethodPost, path: fmt.Sprintf("/activities/%d/supervisors", activityID), body: map[string]any{"staff_id": target.ID}},
		{name: "change supervisor role", method: http.MethodPut, path: fmt.Sprintf("/activities/%d/supervisors/%d", activityID, secondSupervisor.ID), body: map[string]any{"is_primary": true}},
		{name: "remove supervisor", method: http.MethodDelete, path: fmt.Sprintf("/activities/%d/supervisors/%d", activityID, firstSupervisor.ID)},
	}
}

func enrollmentWriteCases(t *testing.T, ctx *testContext, activityID int64) []activityWriteCase {
	t.Helper()
	existing := testpkg.CreateTestStudent(t, ctx.db, "ExistingSecret", "Student", "1a")
	target := testpkg.CreateTestStudent(t, ctx.db, "TargetSecret", "Student", "1b")
	require.NoError(t, ctx.resource.ActivityService.EnrollStudent(testpkg.Ctx(t), activityID, existing.ID))

	return []activityWriteCase{
		{name: "enroll student", method: http.MethodPost, path: fmt.Sprintf("/activities/%d/students/%d", activityID, target.ID)},
		{name: "unenroll student", method: http.MethodDelete, path: fmt.Sprintf("/activities/%d/students/%d", activityID, existing.ID)},
		{name: "replace roster", method: http.MethodPut, path: fmt.Sprintf("/activities/%d/students", activityID), body: map[string]any{"student_ids": []int64{target.ID}}},
	}
}

func TestActivityWriteRoutesRejectNonOwner(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)
	owner := testpkg.CreateTestStaff(t, ctx.db, "Activity", "Owner")
	activityID := createOwnedActivity(t, ctx.db, owner.ID, "Non-owner target")
	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "Unauthorised", "Writer")
	claims := activityStaffClaims(account, permissions.ActivitiesAssign, permissions.ActivitiesEnroll)
	cases := append(supervisorWriteCases(t, ctx, activityID), enrollmentWriteCases(t, ctx, activityID)...)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.NewAuthenticatedRequest(t, tc.method, tc.path, tc.body)
			rr := testutil.ExecuteWithAuth(t, ctx.router, req, claims)
			assert.Equal(t, http.StatusForbidden, rr.Code, "body: %s", rr.Body.String())
			assert.NotContains(t, rr.Body.String(), "TargetSecret")
			assert.NotContains(t, rr.Body.String(), "ExistingSecret")
		})
	}

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/activities/%d/supervisors", activityID), nil)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())
	require.Equal(t, http.StatusOK, rr.Code)
	assert.NotContains(t, rr.Body.String(), "TargetSecret")

	req = testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/activities/%d/students", activityID), nil)
	rr = testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "ExistingSecret")
	assert.NotContains(t, rr.Body.String(), "TargetSecret")
}

func assertActivityWritesAllowed(t *testing.T, ctx *testContext, activityID int64, claims jwt.AppClaims) {
	t.Helper()
	targetStaff := testpkg.CreateTestStaff(t, ctx.db, "Allowed", "Supervisor")
	student := testpkg.CreateTestStudent(t, ctx.db, "Allowed", "Student", "1a")

	request := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/activities/%d/supervisors", activityID), map[string]any{"staff_id": targetStaff.ID})
	response := testutil.ExecuteWithAuth(t, ctx.router, request, claims)
	require.Equal(t, http.StatusCreated, response.Code, "body: %s", response.Body.String())

	request = testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/activities/%d/students", activityID), map[string]any{"student_ids": []int64{student.ID}})
	response = testutil.ExecuteWithAuth(t, ctx.router, request, claims)
	require.Equal(t, http.StatusOK, response.Code, "body: %s", response.Body.String())
}

func TestActivityWritesRequireDomainPermission(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)
	owner, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "Permission", "Owner")
	activityID := createOwnedActivity(t, ctx.db, owner.ID, "Permission target")
	target := testpkg.CreateTestStaff(t, ctx.db, "PermissionSecret", "Supervisor")

	request := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/activities/%d/supervisors", activityID), map[string]any{"staff_id": target.ID})
	response := testutil.ExecuteWithAuth(t, ctx.router, request, activityStaffClaims(account))
	assert.Equal(t, http.StatusForbidden, response.Code, "body: %s", response.Body.String())

	request = testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/activities/%d/students", activityID), map[string]any{"student_ids": []int64{}})
	response = testutil.ExecuteWithAuth(t, ctx.router, request, activityStaffClaims(account))
	assert.Equal(t, http.StatusForbidden, response.Code, "body: %s", response.Body.String())
}

func TestActivityOwnerCanWriteSupervisorAndRoster(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)
	owner, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "Allowed", "Owner")
	activityID := createOwnedActivity(t, ctx.db, owner.ID, "Owner target")
	assertActivityWritesAllowed(t, ctx, activityID, activityStaffClaims(account, permissions.ActivitiesAssign, permissions.ActivitiesEnroll))
}

func TestActivitySupervisorCanWriteSupervisorAndRoster(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Supervisor-owned target")
	supervisor, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "Allowed", "ExistingSupervisor")
	_, err := ctx.resource.ActivityService.AddSupervisor(testpkg.Ctx(t), activity.ID, supervisor.ID, true)
	require.NoError(t, err)

	claims := activityStaffClaims(account, permissions.ActivitiesAssign, permissions.ActivitiesEnroll)
	assertActivityWritesAllowed(t, ctx, activity.ID, claims)
}

func TestActivityAdminCanWriteSupervisorAndRoster(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Admin target")
	assertActivityWritesAllowed(t, ctx, activity.ID, testutil.DefaultTestClaims())
}
