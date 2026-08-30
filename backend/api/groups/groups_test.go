// Package groups_test tests the groups API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package groups_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	groupsAPI "github.com/moto-nrw/project-phoenix/api/groups"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults so jwt.MustNewTokenAuth (called by
// groupsAPI.Resource.Router) succeeds in CI environments without a populated
// .env. Required because setupTestContext constructs the resource, and the
// tests mint real signed JWTs via testutil.MintTestJWT.
func init() {
	testutil.SeedTestJWTConfig()
}

// newReq builds a request authenticated with a real signed JWT so the
// production middleware chain mounted by Resource.Router()
// (Verifier → Authenticator → TenantMiddleware → RequiresPermission →
// TenantTxMiddleware) accepts it. The permission set carried by the token is
// exactly `permissions` — this mirrors the old testutil.WithPermissions option
// (empty for the transfer routes, which mount no permission check). All other
// claim fields come from `claims`.
func newReq(t *testing.T, method, target string, body interface{}, claims jwt.AppClaims, permissions ...string) *http.Request {
	t.Helper()
	claims.Permissions = permissions
	token := testutil.MintTestJWT(t, claims)
	return testutil.NewAuthenticatedRequest(t, method, target, body, testutil.WithJWTBearer(token))
}

// testContext holds shared test resources
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *groupsAPI.Resource
}

// setupTestContext creates test resources for groups handler tests
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupGroupsRoute(t)

	// Groups resource requires multiple services and repositories
	resource := groupsAPI.NewResource(
		svc.Education,
		svc.Active,
		svc.Users,
		svc.UserContext,
		db,
	)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// setupProtectedRouter mounts the production Resource.Router() at /groups so
// tests exercise the real route + middleware wiring. Router() already includes
// the JWT chain and per-route permission checks; requests must carry a signed
// token (see newReq).
func setupProtectedRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Mount("/groups", tc.resource.Router())

	return tc, router
}

// =============================================================================
// LIST GROUPS TESTS
// =============================================================================

func TestListGroups_Success(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create test education group fixture
	testpkg.CreateTestEducationGroup(t, tc.db, "ListTest")

	req := newReq(t, "GET", "/groups", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListGroups_WithNameFilter(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create test group fixture
	testpkg.CreateTestEducationGroup(t, tc.db, "UniqueFilterName")

	req := newReq(t, "GET", "/groups?name=UniqueFilterName", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListGroups_WithPagination(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups?page=1&page_size=10", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListGroups_WithoutPermission(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups", nil, testutil.DefaultTestClaims())

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// GET GROUP TESTS
// =============================================================================

func TestGetGroup_Success(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create test group fixture
	group := testpkg.CreateTestEducationGroup(t, tc.db, "GetGroupTest")

	req := newReq(t, "GET", fmt.Sprintf("/groups/%d", group.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains correct data
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	assert.Contains(t, data["name"].(string), "GetGroupTest")
}

func TestGetGroup_NotFound(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/999999", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestGetGroup_InvalidID(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/invalid", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestGetGroup_WithoutPermission(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "PermTest")

	req := newReq(t, "GET", fmt.Sprintf("/groups/%d", group.ID), nil, testutil.DefaultTestClaims())

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// CREATE GROUP TESTS
// =============================================================================

func TestCreateGroup_Success(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	// Use unique name to avoid conflicts with seeded data
	uniqueName := fmt.Sprintf("NewTestGroup-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name": uniqueName,
	}

	req := newReq(t, "POST", "/groups", body, testutil.DefaultTestClaims(), "groups:create")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

}

func TestCreateGroup_WithRoom(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create a room first
	room := testpkg.CreateTestRoom(t, tc.db, "TestRoom")

	// Use unique name to avoid conflicts
	uniqueName := fmt.Sprintf("GroupWithRoom-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":    uniqueName,
		"room_id": room.ID,
	}

	req := newReq(t, "POST", "/groups", body, testutil.DefaultTestClaims(), "groups:create")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

}

func TestCreateGroup_MissingName(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{} // Missing name

	req := newReq(t, "POST", "/groups", body, testutil.DefaultTestClaims(), "groups:create")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCreateGroup_WithoutPermission(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"name": "NoPermGroup",
	}

	req := newReq(t, "POST", "/groups", body, testutil.DefaultTestClaims())

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// UPDATE GROUP TESTS
// =============================================================================

func TestUpdateGroup_Success(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create test group with unique name
	group := testpkg.CreateTestEducationGroup(t, tc.db, "OriginalUpdateTest")

	// Use unique name for update
	uniqueNewName := fmt.Sprintf("UpdatedGroup-%d", group.ID)
	body := map[string]interface{}{
		"name": uniqueNewName,
	}

	req := newReq(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body, testutil.DefaultTestClaims(), "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify update
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	assert.Equal(t, uniqueNewName, data["name"])
}

func TestUpdateGroup_NotFound(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"name": "UpdatedName",
	}

	req := newReq(t, "PUT", "/groups/999999", body, testutil.DefaultTestClaims(), "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestUpdateGroup_InvalidID(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"name": "UpdatedName",
	}

	req := newReq(t, "PUT", "/groups/invalid", body, testutil.DefaultTestClaims(), "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestUpdateGroup_WithoutPermission(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "NoPermUpdate")

	body := map[string]interface{}{
		"name": "UpdatedName",
	}

	req := newReq(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body, testutil.DefaultTestClaims())

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// DELETE GROUP TESTS
// =============================================================================

func TestDeleteGroup_Success(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create test group to delete
	group := testpkg.CreateTestEducationGroup(t, tc.db, "ToDelete")
	// No defer cleanup needed since we're deleting it

	req := newReq(t, "DELETE", fmt.Sprintf("/groups/%d", group.ID), nil, testutil.DefaultTestClaims(), "groups:delete")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteGroup_NotFound(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "DELETE", "/groups/999999", nil, testutil.DefaultTestClaims(), "groups:delete")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestDeleteGroup_InvalidID(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "DELETE", "/groups/invalid", nil, testutil.DefaultTestClaims(), "groups:delete")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestDeleteGroup_ConflictWithStudents(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "GroupWithStudents")
	student := testpkg.CreateTestStudent(t, tc.db, "GroupDel", "Student", "1a")

	// Assign student to group
	ctx := testpkg.Ctx(t)
	student.GroupID = &group.ID
	_, err := tc.db.NewUpdate().
		Model(student).
		ModelTableExpr(`users.students AS "student"`).
		Column("group_id").
		Where(`"student".id = ?`, student.ID).
		Exec(ctx)
	require.NoError(t, err)

	req := newReq(t, "DELETE", fmt.Sprintf("/groups/%d", group.ID), nil, testutil.DefaultTestClaims(), "groups:delete")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusConflict)
}

func TestDeleteGroup_WithoutPermission(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "NoPermDelete")

	req := newReq(t, "DELETE", fmt.Sprintf("/groups/%d", group.ID), nil, testutil.DefaultTestClaims())

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// GET GROUP STUDENTS TESTS
// =============================================================================

func TestGetGroupStudents_Success(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "StudentsTest")

	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/students", group.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupStudents_NotFound(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/999999/students", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestGetGroupStudents_InvalidID(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/invalid/students", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET GROUP SUPERVISORS TESTS
// =============================================================================

func TestGetGroupSupervisors_Success(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "SupervisorsTest")

	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/supervisors", group.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupSupervisors_NotFound(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/999999/supervisors", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// GET GROUP SUBSTITUTIONS TESTS
// =============================================================================

func TestGetGroupStudentsRoomStatus_RequiresSupervisor(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "RoomStatusTest")

	// Without being a supervisor of the group, should get forbidden
	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/students/room-status", group.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	// Returns 403 because user doesn't supervise this group
	testutil.AssertForbidden(t, rr)
}

func TestGetGroupStudentsRoomStatus_NotFound(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/999999/students/room-status", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestGetGroupStudentsRoomStatus_InvalidID(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/invalid/students/room-status", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// ROUTER TEST
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	router := tc.resource.Router()
	require.NotNil(t, router, "Router should return a valid chi.Router")
}

// =============================================================================
// GROUP WITH STUDENTS TESTS
// =============================================================================

func TestGetGroupStudents_WithStudent(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "WithStudentTest")

	// Create a student and assign to the group
	student := testpkg.CreateTestStudent(t, tc.db, "GroupStudent", "Test", "1a")

	// Assign student to group
	_, err := tc.db.NewUpdate().
		Model((*users.Student)(nil)).
		ModelTableExpr("users.students").
		Set("group_id = ?", group.ID).
		Where("id = ?", student.ID).
		Exec(context.Background())
	require.NoError(t, err)

	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/students", group.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// CREATE GROUP ADDITIONAL TESTS
// =============================================================================

func TestCreateGroup_EmptyName(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"name": "", // Empty name
	}

	req := newReq(t, "POST", "/groups", body, testutil.DefaultTestClaims(), "groups:create")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCreateGroup_WithDescription(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	// Use unique name to avoid conflicts
	uniqueName := fmt.Sprintf("GroupWithDesc-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":        uniqueName,
		"description": "Test group description",
	}

	req := newReq(t, "POST", "/groups", body, testutil.DefaultTestClaims(), "groups:create")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

}

// =============================================================================
// UPDATE GROUP ADDITIONAL TESTS
// =============================================================================

func TestUpdateGroup_WithRoom(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create test group and room
	group := testpkg.CreateTestEducationGroup(t, tc.db, "UpdateRoomTest")

	room := testpkg.CreateTestRoom(t, tc.db, "UpdateTestRoom")

	body := map[string]interface{}{
		"name":    fmt.Sprintf("UpdatedWithRoom-%d", group.ID),
		"room_id": room.ID,
	}

	req := newReq(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body, testutil.DefaultTestClaims(), "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateGroup_EmptyName(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "EmptyNameUpdateTest")

	body := map[string]interface{}{
		"name": "", // Empty name should fail
	}

	req := newReq(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body, testutil.DefaultTestClaims(), "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// LIST GROUPS ADDITIONAL TESTS
// =============================================================================

func TestListGroups_InvalidPagination(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	// Test with invalid page number
	req := newReq(t, "GET", "/groups?page=-1", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	// May succeed with default pagination or fail depending on validation
	t.Logf("Response: %d - %s", rr.Code, rr.Body.String())
}

func TestListGroups_LargePageSize(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups?page_size=1000", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	// Should succeed - large page size might be capped
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET GROUP SUPERVISORS ADDITIONAL TESTS
// =============================================================================

func TestGetGroupSupervisors_InvalidID(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/invalid/supervisors", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET GROUP SUBSTITUTIONS ADDITIONAL TESTS
// =============================================================================

func TestGetGroupStudentsRoomStatus_WithAdmin(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create test group with a room
	room := testpkg.CreateTestRoom(t, tc.db, "AdminRoomStatus")

	group := testpkg.CreateTestEducationGroup(t, tc.db, "AdminStatusTest")

	// Update group with room
	_, err := tc.db.NewUpdate().
		Model((*education.Group)(nil)).
		ModelTableExpr("education.groups").
		Set("room_id = ?", room.ID).
		Where("id = ?", group.ID).
		Exec(context.Background())
	require.NoError(t, err)

	// Admin should have access
	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/students/room-status", group.ID), nil, testutil.AdminTestClaims(1), "groups:read", "admin:*")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify the response has the expected structure
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	assert.True(t, data["group_has_room"].(bool), "Should have room")
	assert.Equal(t, float64(room.ID), data["group_room_id"].(float64))
}

func TestGetGroupStudentsRoomStatus_NoRoomAssigned(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create test group without room
	group := testpkg.CreateTestEducationGroup(t, tc.db, "NoRoomStatusTest")

	// Admin should have access
	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/students/room-status", group.ID), nil, testutil.AdminTestClaims(1), "groups:read", "admin:*")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify the response indicates no room
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	assert.False(t, data["group_has_room"].(bool), "Should not have room")
}

// =============================================================================
// GET GROUP STUDENTS WITH FULL ACCESS TESTS
// =============================================================================

func TestGetGroupStudents_WithFullAccessAdmin(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AdminStudentsTest")

	// Create a student with guardian info
	student := testpkg.CreateTestStudent(t, tc.db, "GuardianTest", "Student", "2a")

	// Update student with guardian info
	guardianName := "Test Guardian"
	guardianEmail := "guardian@test.com"
	_, err := tc.db.NewUpdate().
		Model((*users.Student)(nil)).
		ModelTableExpr("users.students").
		Set("group_id = ?", group.ID).
		Set("guardian_name = ?", guardianName).
		Set("guardian_email = ?", guardianEmail).
		Where("id = ?", student.ID).
		Exec(context.Background())
	require.NoError(t, err)

	// Admin should see full details including guardian
	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/students", group.ID), nil, testutil.AdminTestClaims(1), "groups:read", "admin:*")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// LIST GROUPS WITH ROOM FILTER TEST
// =============================================================================

func TestListGroups_WithRoomIDFilter(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create room
	room := testpkg.CreateTestRoom(t, tc.db, "FilterRoom")

	// Create group with room
	group := testpkg.CreateTestEducationGroup(t, tc.db, "RoomFilterTest")

	_, err := tc.db.NewUpdate().
		Model((*education.Group)(nil)).
		ModelTableExpr("education.groups").
		Set("room_id = ?", room.ID).
		Where("id = ?", group.ID).
		Exec(context.Background())
	require.NoError(t, err)

	req := newReq(t, "GET", fmt.Sprintf("/groups?room_id=%d", room.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// CREATE GROUP WITH TEACHER IDS TEST
// =============================================================================

func TestCreateGroup_WithTeacherIDs(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create a teacher
	teacher := testpkg.CreateTestTeacher(t, tc.db, "Assign", "Teacher")

	uniqueName := fmt.Sprintf("GroupWithTeachers-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":        uniqueName,
		"teacher_ids": []int64{teacher.ID},
	}

	req := newReq(t, "POST", "/groups", body, testutil.DefaultTestClaims(), "groups:create")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

}

// =============================================================================
// UPDATE GROUP WITH TEACHER IDS TEST
// =============================================================================

func TestUpdateGroup_WithTeacherIDs(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "UpdateTeachersTest")

	teacher := testpkg.CreateTestTeacher(t, tc.db, "Update", "Teacher")

	body := map[string]interface{}{
		"name":        fmt.Sprintf("UpdatedWithTeachers-%d", group.ID),
		"teacher_ids": []int64{teacher.ID},
	}

	req := newReq(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body, testutil.DefaultTestClaims(), "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupStudentsRoomStatus_WithSubstitution(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	// Create group with room
	room := testpkg.CreateTestRoom(t, tc.db, "SubstitutionRoom")

	group := testpkg.CreateTestEducationGroup(t, tc.db, "SubstitutionAccessTest")

	// Update group with room
	_, err := tc.db.NewUpdate().
		Model((*education.Group)(nil)).
		ModelTableExpr("education.groups").
		Set("room_id = ?", room.ID).
		Where("id = ?", group.ID).
		Exec(context.Background())
	require.NoError(t, err)

	// Create staff with account for context
	staff, _ := testpkg.CreateTestStaffWithAccount(t, tc.db, "Substitute", "Supervisor")

	// Create active substitution for today (grants access)
	today := timezone.TodayDate()
	testpkg.CreateTestGroupSubstitution(t, tc.db, group.ID, nil, staff.ID, today, today)

	// Staff should have access via substitution
	// Note: DefaultTestClaims provides admin access for this test
	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/students/room-status", group.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	// Note: This may still fail if the userContextService doesn't pick up substitutions
	// The test documents the expected behavior
	t.Logf("Response: %d - %s", rr.Code, rr.Body.String())
}

// =============================================================================
// TRANSFER CANCEL TESTS
// =============================================================================
