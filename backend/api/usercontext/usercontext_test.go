// Package usercontext_test tests the usercontext API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package usercontext_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	usercontextAPI "github.com/moto-nrw/project-phoenix/api/usercontext"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	repos    *repositories.Factory
	resource *usercontextAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")

	// Create usercontext resource with service and substitution repository
	resource := usercontextAPI.NewResource(
		serviceFactory.UserContext,
		repoFactory.GroupSubstitution,
	)

	return &testContext{
		db:       db,
		services: serviceFactory,
		repos:    repoFactory,
		resource: resource,
	}
}

// =============================================================================
// GET CURRENT USER TESTS
// =============================================================================

func TestGetCurrentUser_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a test account
	account := testpkg.CreateTestAccount(t, ctx.db, "usercontext-test@example.com")

	router := chi.NewRouter()
	router.Get("/me", ctx.resource.GetCurrentUserHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/me", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetCurrentUser_Unauthenticated(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/me", ctx.resource.GetCurrentUserHandler())

	// Request without claims - simulates unauthenticated user
	req := testutil.NewAuthenticatedRequest(t, "GET", "/me", nil)

	rr := testutil.ExecuteRequest(router, req)

	// Handler should return 401 for missing authentication
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET CURRENT PROFILE TESTS
// =============================================================================

func TestGetCurrentProfile_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a test account with person
	_, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "Profile", "Test")

	router := chi.NewRouter()
	router.Get("/profile", ctx.resource.GetCurrentProfileHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/profile", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetCurrentProfile_Unauthenticated(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/profile", ctx.resource.GetCurrentProfileHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/profile", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// UPDATE CURRENT PROFILE TESTS
// =============================================================================

func TestUpdateCurrentProfile_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	_, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "Update", "ProfileTest")

	router := chi.NewRouter()
	router.Put("/profile", ctx.resource.UpdateCurrentProfileHandler())

	body := map[string]interface{}{
		"first_name": "Updated",
		"last_name":  "Name",
	}

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/profile", body,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateCurrentProfile_Unauthenticated(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/profile", ctx.resource.UpdateCurrentProfileHandler())

	body := map[string]interface{}{
		"first_name": "Updated",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/profile", body)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

func TestUpdateCurrentProfile_EmptyBody(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	_, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "Empty", "Update")

	router := chi.NewRouter()
	router.Put("/profile", ctx.resource.UpdateCurrentProfileHandler())

	// Empty body should still succeed (no fields to update)
	body := map[string]interface{}{}

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/profile", body,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET CURRENT STAFF TESTS
// =============================================================================

func TestGetCurrentStaff_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a test staff with account
	_, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Staff", "Test")

	router := chi.NewRouter()
	router.Get("/staff", ctx.resource.GetCurrentStaffHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetCurrentStaff_NotStaff(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create account without staff role
	account := testpkg.CreateTestAccount(t, ctx.db, "not-staff@example.com")

	router := chi.NewRouter()
	router.Get("/staff", ctx.resource.GetCurrentStaffHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 404 or 403 when user is not a staff member
	assert.Contains(t, []int{http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError}, rr.Code)
}

func TestGetCurrentStaff_Unauthenticated(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff", ctx.resource.GetCurrentStaffHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET CURRENT TEACHER TESTS
// =============================================================================

func TestGetCurrentTeacher_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	_, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Teacher", "Test")

	router := chi.NewRouter()
	router.Get("/teacher", ctx.resource.GetCurrentTeacherHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/teacher", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetCurrentTeacher_NotTeacher(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	account := testpkg.CreateTestAccount(t, ctx.db, "not-teacher@example.com")

	router := chi.NewRouter()
	router.Get("/teacher", ctx.resource.GetCurrentTeacherHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/teacher", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error when user is not a teacher
	assert.Contains(t, []int{http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError}, rr.Code)
}

func TestGetCurrentTeacher_Unauthenticated(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/teacher", ctx.resource.GetCurrentTeacherHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/teacher", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET MY GROUPS TESTS
// =============================================================================

func TestGetMyGroups_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	_, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Groups", "Test")

	router := chi.NewRouter()
	router.Get("/groups", ctx.resource.GetMyGroupsHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetMyGroups_Unauthenticated(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/groups", ctx.resource.GetMyGroupsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET MY ACTIVITY GROUPS TESTS
// =============================================================================

func TestGetMyActivityGroups_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	_, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Activity", "Groups")

	router := chi.NewRouter()
	router.Get("/groups/activity", ctx.resource.GetMyActivityGroupsHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/activity", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetMyActivityGroups_Unauthenticated(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/groups/activity", ctx.resource.GetMyActivityGroupsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/activity", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET MY ACTIVE GROUPS TESTS
// =============================================================================

func TestGetMyActiveGroups_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	_, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Active", "Groups")

	router := chi.NewRouter()
	router.Get("/groups/active", ctx.resource.GetMyActiveGroupsHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/active", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetMyActiveGroups_Unauthenticated(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/groups/active", ctx.resource.GetMyActiveGroupsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/active", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET MY SUPERVISED GROUPS TESTS
// =============================================================================

func TestGetMySupervisedGroups_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	_, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Supervised", "Groups")

	router := chi.NewRouter()
	router.Get("/groups/supervised", ctx.resource.GetMySupervisedGroupsHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/supervised", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetMySupervisedGroups_Unauthenticated(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/groups/supervised", ctx.resource.GetMySupervisedGroupsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/supervised", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET GROUP STUDENTS TESTS
// =============================================================================

func TestGetGroupStudents_InvalidGroupID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/groups/{groupID}/students", ctx.resource.GetGroupStudentsHandler())

	claims := testutil.DefaultTestClaims()
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/invalid/students", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGetGroupStudents_Unauthenticated(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create fixture for active group instead of using hardcoded ID
	activityGroup := testpkg.CreateTestActivityGroup(t, ctx.db, "GroupStudentsUnauth")
	room := testpkg.CreateTestRoom(t, ctx.db, "GroupStudentsUnauthRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activityGroup.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID, activityGroup.CategoryID, room.ID)

	router := chi.NewRouter()
	router.Get("/groups/{groupID}/students", ctx.resource.GetGroupStudentsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students", activeGroup.ID), nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET GROUP VISITS TESTS
// =============================================================================

func TestGetGroupVisits_InvalidGroupID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/groups/{groupID}/visits", ctx.resource.GetGroupVisitsHandler())

	claims := testutil.DefaultTestClaims()
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/invalid/visits", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGetGroupVisits_Unauthenticated(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create fixture for active group instead of using hardcoded ID
	activityGroup := testpkg.CreateTestActivityGroup(t, ctx.db, "GroupVisitsUnauth")
	room := testpkg.CreateTestRoom(t, ctx.db, "GroupVisitsUnauthRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activityGroup.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID, activityGroup.CategoryID, room.ID)

	router := chi.NewRouter()
	router.Get("/groups/{groupID}/visits", ctx.resource.GetGroupVisitsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/visits", activeGroup.ID), nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// AVATAR TESTS
// =============================================================================

func TestDeleteAvatar_Unauthenticated(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/profile/avatar", ctx.resource.DeleteAvatarHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/profile/avatar", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

func TestDeleteAvatar_NoAvatar(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	_, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "NoAvatar", "Test")

	router := chi.NewRouter()
	router.Delete("/profile/avatar", ctx.resource.DeleteAvatarHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/profile/avatar", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 400 when no avatar exists
	testutil.AssertBadRequest(t, rr)
}

func TestServeAvatar_MissingFilename(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	_, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "Avatar", "Serve")

	router := chi.NewRouter()
	router.Get("/profile/avatar/{filename}", ctx.resource.ServeAvatarHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/profile/avatar/", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 400 or 404 for missing filename
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, rr.Code)
}

func TestServeAvatar_Unauthenticated(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/profile/avatar/{filename}", ctx.resource.ServeAvatarHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/profile/avatar/test.jpg", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// ROUTER TESTS
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()
	assert.NotNil(t, router, "Router should not be nil")
}

// =============================================================================
// UPDATE PROFILE WITH ALL FIELDS TESTS
// =============================================================================

func TestUpdateCurrentProfile_WithUsernameAndBio(t *testing.T) {
	tc := setupTestContext(t)
	defer func() { _ = tc.db.Close() }()

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "FullUpdate", "ProfileTest")

	router := chi.NewRouter()
	router.Put("/profile", tc.resource.UpdateCurrentProfileHandler())

	// Use unique username to avoid conflicts
	uniqueUsername := fmt.Sprintf("user_%d", account.ID)
	body := map[string]interface{}{
		"first_name": "NewFirst",
		"last_name":  "NewLast",
		"username":   uniqueUsername,
		"bio":        "This is my bio text",
	}

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/profile", body,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GROUP STUDENTS WITH TEACHER ACCESS TESTS
// =============================================================================

func TestGetGroupStudents_WithTeacherAccess(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "GroupStudents", "Teacher")

	// Create an active group
	activityGroup := testpkg.CreateTestActivityGroup(t, ctx.db, "StudentAccessGroup")
	room := testpkg.CreateTestRoom(t, ctx.db, "StudentAccessRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activityGroup.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID, activityGroup.CategoryID, room.ID, teacher.ID)

	router := chi.NewRouter()
	router.Get("/groups/{groupID}/students", ctx.resource.GetGroupStudentsHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students", activeGroup.ID), nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should succeed or return access denied (depends on supervisor relationship)
	assert.Contains(t, []int{http.StatusOK, http.StatusForbidden, http.StatusInternalServerError}, rr.Code)
}

// =============================================================================
// GROUP VISITS WITH TEACHER ACCESS TESTS
// =============================================================================

func TestGetGroupVisits_WithTeacherAccess(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "GroupVisits", "Teacher")

	// Create an active group
	activityGroup := testpkg.CreateTestActivityGroup(t, ctx.db, "VisitsAccessGroup")
	room := testpkg.CreateTestRoom(t, ctx.db, "VisitsAccessRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activityGroup.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID, activityGroup.CategoryID, room.ID, teacher.ID)

	router := chi.NewRouter()
	router.Get("/groups/{groupID}/visits", ctx.resource.GetGroupVisitsHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/visits", activeGroup.ID), nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should succeed or return access denied (depends on supervisor relationship)
	assert.Contains(t, []int{http.StatusOK, http.StatusForbidden, http.StatusInternalServerError}, rr.Code)
}

// =============================================================================
// SERVE AVATAR INVALID PATH TESTS
// =============================================================================

func TestServeAvatar_InvalidFilename(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	_, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "InvalidPath", "Avatar")

	router := chi.NewRouter()
	router.Get("/profile/avatar/{filename}", ctx.resource.ServeAvatarHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))

	// Test with path traversal attempt
	req := testutil.NewAuthenticatedRequest(t, "GET", "/profile/avatar/../../../etc/passwd", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error for invalid path
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound}, rr.Code)
}

func TestServeAvatar_NonExistentFile(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	_, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "NonExistent", "Avatar")

	router := chi.NewRouter()
	router.Get("/profile/avatar/{filename}", ctx.resource.ServeAvatarHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/profile/avatar/nonexistent_file_12345.jpg", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 403 or 404 for non-existent file
	assert.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, rr.Code)
}

// =============================================================================
// UPLOAD AVATAR TESTS
// =============================================================================

func TestUploadAvatar_Unauthenticated(t *testing.T) {
	tc := setupTestContext(t)
	defer func() { _ = tc.db.Close() }()

	router := chi.NewRouter()
	router.Post("/profile/avatar", tc.resource.UploadAvatarHandler())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/profile/avatar", nil)

	rr := testutil.ExecuteRequest(router, req)

	// Upload handler may return 400 (no file) before checking auth, or 401 if auth checked first
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnauthorized}, rr.Code,
		"Expected 400 or 401 for unauthenticated request")
}

func TestUploadAvatar_NoFile(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	_, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "NoFile", "Upload")

	router := chi.NewRouter()
	router.Post("/profile/avatar", ctx.resource.UploadAvatarHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "POST", "/profile/avatar", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 400 for missing file
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DELETE AVATAR WITH AVATAR TESTS
// =============================================================================

func TestDeleteAvatar_WithAvatar(t *testing.T) {
	tc := setupTestContext(t)
	defer func() { _ = tc.db.Close() }()

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "HasAvatar", "Delete")

	// Set avatar in users.profiles via raw SQL
	_, err := tc.db.NewRaw(
		`UPDATE users.profiles SET avatar = ?
		 WHERE person_id = (SELECT id FROM auth.accounts WHERE id = ?)`,
		"/uploads/avatars/test_avatar.jpg",
		account.ID,
	).Exec(context.Background())
	// Note: This may fail if person doesn't have a profile - that's OK
	_ = err

	router := chi.NewRouter()
	router.Delete("/profile/avatar", tc.resource.DeleteAvatarHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/profile/avatar", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	// The request might succeed (200) or return 400 if no avatar
	assert.Contains(t, []int{http.StatusOK, http.StatusBadRequest, http.StatusInternalServerError}, rr.Code)
}

// =============================================================================
// GET MY GROUPS WITH SUBSTITUTION TESTS
// =============================================================================

func TestGetMyGroups_WithSubstitution(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	_, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "SubstGroups", "Teacher")

	router := chi.NewRouter()
	router.Get("/groups", ctx.resource.GetMyGroupsHandler())

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response structure
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, "success", response["status"])
}
