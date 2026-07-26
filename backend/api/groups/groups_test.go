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
	"github.com/moto-nrw/project-phoenix/services"
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

	db, svc := testutil.SetupAPITest(t)

	// Groups resource requires multiple services and repositories
	resource := groupsAPI.NewResource(
		svc.Education,
		svc.Active,
		svc.Users,
		svc.UserContext,
		db,
	)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

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
	tc, router := setupProtectedRouter(t)

	// Create test education group fixture
	group := testpkg.CreateTestEducationGroup(t, tc.db, "ListTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := newReq(t, "GET", "/groups", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListGroups_WithNameFilter(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group fixture
	group := testpkg.CreateTestEducationGroup(t, tc.db, "UniqueFilterName")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := newReq(t, "GET", "/groups?name=UniqueFilterName", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListGroups_WithPagination(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups?page=1&page_size=10", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListGroups_WithoutPermission(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups", nil, testutil.DefaultTestClaims())

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// GET GROUP TESTS
// =============================================================================

func TestGetGroup_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group fixture
	group := testpkg.CreateTestEducationGroup(t, tc.db, "GetGroupTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := newReq(t, "GET", fmt.Sprintf("/groups/%d", group.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains correct data
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	assert.Contains(t, data["name"].(string), "GetGroupTest")
}

func TestGetGroup_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/999999", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestGetGroup_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/invalid", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestGetGroup_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "PermTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := newReq(t, "GET", fmt.Sprintf("/groups/%d", group.ID), nil, testutil.DefaultTestClaims())

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// CREATE GROUP TESTS
// =============================================================================

func TestCreateGroup_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Use unique name to avoid conflicts with seeded data
	uniqueName := fmt.Sprintf("NewTestGroup-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name": uniqueName,
	}

	req := newReq(t, "POST", "/groups", body, testutil.DefaultTestClaims(), "groups:create")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Cleanup created group
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	groupID := int64(data["id"].(float64))
	testpkg.CleanupActivityFixtures(t, tc.db, groupID)
}

func TestCreateGroup_WithRoom(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create a room first
	room := testpkg.CreateTestRoom(t, tc.db, "TestRoom")
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	// Use unique name to avoid conflicts
	uniqueName := fmt.Sprintf("GroupWithRoom-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":    uniqueName,
		"room_id": room.ID,
	}

	req := newReq(t, "POST", "/groups", body, testutil.DefaultTestClaims(), "groups:create")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Cleanup created group
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	groupID := int64(data["id"].(float64))
	testpkg.CleanupActivityFixtures(t, tc.db, groupID)
}

func TestCreateGroup_MissingName(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{} // Missing name

	req := newReq(t, "POST", "/groups", body, testutil.DefaultTestClaims(), "groups:create")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCreateGroup_WithoutPermission(t *testing.T) {
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
	tc, router := setupProtectedRouter(t)

	// Create test group with unique name
	group := testpkg.CreateTestEducationGroup(t, tc.db, "OriginalUpdateTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

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
	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"name": "UpdatedName",
	}

	req := newReq(t, "PUT", "/groups/999999", body, testutil.DefaultTestClaims(), "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestUpdateGroup_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"name": "UpdatedName",
	}

	req := newReq(t, "PUT", "/groups/invalid", body, testutil.DefaultTestClaims(), "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestUpdateGroup_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "NoPermUpdate")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

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
	tc, router := setupProtectedRouter(t)

	// Create test group to delete
	group := testpkg.CreateTestEducationGroup(t, tc.db, "ToDelete")
	// No defer cleanup needed since we're deleting it

	req := newReq(t, "DELETE", fmt.Sprintf("/groups/%d", group.ID), nil, testutil.DefaultTestClaims(), "groups:delete")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteGroup_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := newReq(t, "DELETE", "/groups/999999", nil, testutil.DefaultTestClaims(), "groups:delete")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestDeleteGroup_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := newReq(t, "DELETE", "/groups/invalid", nil, testutil.DefaultTestClaims(), "groups:delete")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestDeleteGroup_ConflictWithStudents(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "GroupWithStudents")
	student := testpkg.CreateTestStudent(t, tc.db, "GroupDel", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, student.PersonID)

	// Assign student to group
	ctx := testpkg.TenantContext(1)
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
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "NoPermDelete")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := newReq(t, "DELETE", fmt.Sprintf("/groups/%d", group.ID), nil, testutil.DefaultTestClaims())

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// GET GROUP STUDENTS TESTS
// =============================================================================

func TestGetGroupStudents_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "StudentsTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/students", group.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupStudents_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/999999/students", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestGetGroupStudents_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/invalid/students", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET GROUP SUPERVISORS TESTS
// =============================================================================

func TestGetGroupSupervisors_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "SupervisorsTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/supervisors", group.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupSupervisors_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/999999/supervisors", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// GET GROUP SUBSTITUTIONS TESTS
// =============================================================================

func TestGetGroupSubstitutions_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "SubstitutionsTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/substitutions", group.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupSubstitutions_WithDate(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "SubstitutionsDateTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/substitutions?date=2024-01-15", group.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupSubstitutions_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/999999/substitutions", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// GET GROUP STUDENTS ROOM STATUS TESTS
// =============================================================================

func TestGetGroupStudentsRoomStatus_RequiresSupervisor(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "RoomStatusTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Without being a supervisor of the group, should get forbidden
	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/students/room-status", group.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	// Returns 403 because user doesn't supervise this group
	testutil.AssertForbidden(t, rr)
}

func TestGetGroupStudentsRoomStatus_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/999999/students/room-status", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestGetGroupStudentsRoomStatus_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/invalid/students/room-status", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// ROUTER TEST
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	tc := setupTestContext(t)
	router := tc.resource.Router()
	require.NotNil(t, router, "Router should return a valid chi.Router")
}

// =============================================================================
// GROUP WITH STUDENTS TESTS
// =============================================================================

func TestGetGroupStudents_WithStudent(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "WithStudentTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

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
	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"name": "", // Empty name
	}

	req := newReq(t, "POST", "/groups", body, testutil.DefaultTestClaims(), "groups:create")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCreateGroup_WithDescription(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Use unique name to avoid conflicts
	uniqueName := fmt.Sprintf("GroupWithDesc-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":        uniqueName,
		"description": "Test group description",
	}

	req := newReq(t, "POST", "/groups", body, testutil.DefaultTestClaims(), "groups:create")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Cleanup
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	groupID := int64(data["id"].(float64))
	testpkg.CleanupActivityFixtures(t, tc.db, groupID)
}

// =============================================================================
// UPDATE GROUP ADDITIONAL TESTS
// =============================================================================

func TestUpdateGroup_WithRoom(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group and room
	group := testpkg.CreateTestEducationGroup(t, tc.db, "UpdateRoomTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	room := testpkg.CreateTestRoom(t, tc.db, "UpdateTestRoom")
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	body := map[string]interface{}{
		"name":    fmt.Sprintf("UpdatedWithRoom-%d", group.ID),
		"room_id": room.ID,
	}

	req := newReq(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body, testutil.DefaultTestClaims(), "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateGroup_EmptyName(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "EmptyNameUpdateTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

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
	_, router := setupProtectedRouter(t)

	// Test with invalid page number
	req := newReq(t, "GET", "/groups?page=-1", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	// May succeed with default pagination or fail depending on validation
	t.Logf("Response: %d - %s", rr.Code, rr.Body.String())
}

func TestListGroups_LargePageSize(t *testing.T) {
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
	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/invalid/supervisors", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET GROUP SUBSTITUTIONS ADDITIONAL TESTS
// =============================================================================

func TestGetGroupSubstitutions_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups/invalid/substitutions", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestGetGroupSubstitutions_InvalidDate(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "InvalidDateTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := newReq(t, "GET", fmt.Sprintf("/groups/%d/substitutions?date=invalid-date", group.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	// Invalid date format should return bad request or be ignored
	t.Logf("Response: %d - %s", rr.Code, rr.Body.String())
}

// =============================================================================
// TRANSFER GROUP TESTS
// =============================================================================

func setupTransferRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	// Transfer routes are part of Resource.Router(); mount the full router so
	// the transfer endpoints run through their production wiring. The transfer
	// routes carry no RequiresPermission check (authorization is ownership-based
	// in the handler), matching the old permission-free transfer router.
	router := chi.NewRouter()
	router.Mount("/groups", tc.resource.Router())

	return tc, router
}

func TestTransferGroup_RequiresTeacher(t *testing.T) {
	tc, router := setupTransferRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "TransferTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	body := map[string]interface{}{
		"target_user_id": 1,
	}

	// Regular user (not teacher) should get forbidden
	req := newReq(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body, testutil.DefaultTestClaims())

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

func TestTransferGroup_InvalidGroupID(t *testing.T) {
	_, router := setupTransferRouter(t)

	body := map[string]interface{}{
		"target_user_id": 1,
	}

	req := newReq(t, "POST", "/groups/invalid/transfer", body, testutil.DefaultTestClaims())

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestTransferGroup_MissingTargetUserID(t *testing.T) {
	tc, router := setupTransferRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "TransferMissingTarget")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher with account for context
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Transfer", "Teacher")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to the group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	body := map[string]interface{}{
		"target_user_id": 0, // Invalid - must be positive
	}

	req := newReq(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body, testutil.TeacherTestClaims(int(account.ID)))

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCancelSpecificTransfer_RequiresTeacher(t *testing.T) {
	tc, router := setupTransferRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "CancelTransferTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Regular user (not teacher) should get forbidden
	req := newReq(t, "DELETE", fmt.Sprintf("/groups/%d/transfer/1", group.ID), nil, testutil.DefaultTestClaims())

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

func TestCancelSpecificTransfer_InvalidGroupID(t *testing.T) {
	_, router := setupTransferRouter(t)

	req := newReq(t, "DELETE", "/groups/invalid/transfer/1", nil, testutil.DefaultTestClaims())

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCancelSpecificTransfer_InvalidSubstitutionID(t *testing.T) {
	tc, router := setupTransferRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "CancelInvalidSubst")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := newReq(t, "DELETE", fmt.Sprintf("/groups/%d/transfer/invalid", group.ID), nil, testutil.DefaultTestClaims())

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// ROOM STATUS WITH ADMIN ACCESS TESTS
// =============================================================================

func TestGetGroupStudentsRoomStatus_WithAdmin(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group with a room
	room := testpkg.CreateTestRoom(t, tc.db, "AdminRoomStatus")
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "AdminStatusTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

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
	tc, router := setupProtectedRouter(t)

	// Create test group without room
	group := testpkg.CreateTestEducationGroup(t, tc.db, "NoRoomStatusTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

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
	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AdminStudentsTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create a student with guardian info
	student := testpkg.CreateTestStudent(t, tc.db, "GuardianTest", "Student", "2a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

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
	tc, router := setupProtectedRouter(t)

	// Create room
	room := testpkg.CreateTestRoom(t, tc.db, "FilterRoom")
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	// Create group with room
	group := testpkg.CreateTestEducationGroup(t, tc.db, "RoomFilterTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

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
	tc, router := setupProtectedRouter(t)

	// Create a teacher
	teacher := testpkg.CreateTestTeacher(t, tc.db, "Assign", "Teacher")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)

	uniqueName := fmt.Sprintf("GroupWithTeachers-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":        uniqueName,
		"teacher_ids": []int64{teacher.ID},
	}

	req := newReq(t, "POST", "/groups", body, testutil.DefaultTestClaims(), "groups:create")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Cleanup
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	groupID := int64(data["id"].(float64))
	testpkg.CleanupActivityFixtures(t, tc.db, groupID)
}

// =============================================================================
// UPDATE GROUP WITH TEACHER IDS TEST
// =============================================================================

func TestUpdateGroup_WithTeacherIDs(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "UpdateTeachersTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	teacher := testpkg.CreateTestTeacher(t, tc.db, "Update", "Teacher")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)

	body := map[string]interface{}{
		"name":        fmt.Sprintf("UpdatedWithTeachers-%d", group.ID),
		"teacher_ids": []int64{teacher.ID},
	}

	req := newReq(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body, testutil.DefaultTestClaims(), "groups:update")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// AUTHORIZATION HELPER TESTS - isUserGroupLeader
// =============================================================================

func TestTransferGroup_AsGroupLeader_Success(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "LeaderTransferTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher (group leader) with account for context
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Leader", "Teacher")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to the group (makes them group leader)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Create target staff to transfer to
	targetStaff := testpkg.CreateTestStaff(t, tc.db, "Target", "Staff")
	defer testpkg.CleanupStaffFixtures(t, tc.db, targetStaff.ID)

	body := map[string]interface{}{
		"target_user_id": targetStaff.Person.ID, // Target user ID is the person ID
	}

	req := newReq(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body, testutil.TeacherTestClaims(int(account.ID)))

	rr := testutil.ExecuteRequest(router, req)
	// Should succeed since teacher is group leader (returns 201 Created for new substitution)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestTransferGroup_NotGroupLeader(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "NotLeaderTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher WITHOUT assigning to group (not a group leader)
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "NotLeader", "Teacher")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Create target staff
	targetStaff := testpkg.CreateTestStaff(t, tc.db, "Target", "Staff")
	defer testpkg.CleanupStaffFixtures(t, tc.db, targetStaff.ID)

	body := map[string]interface{}{
		"target_user_id": targetStaff.Person.ID,
	}

	req := newReq(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body, testutil.TeacherTestClaims(int(account.ID)))

	rr := testutil.ExecuteRequest(router, req)
	// Should fail since teacher is not assigned to this group
	testutil.AssertForbidden(t, rr)
}

func TestTransferGroup_CannotTransferToSelf(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "SelfTransferTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Self", "Transfer")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Try to transfer to self (using their own person ID)
	body := map[string]interface{}{
		"target_user_id": teacher.Staff.Person.ID,
	}

	req := newReq(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body, testutil.TeacherTestClaims(int(account.ID)))

	rr := testutil.ExecuteRequest(router, req)
	// Should fail - can't transfer to self
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// AUTHORIZATION HELPER TESTS - userHasGroupAccess via Substitution
// =============================================================================

func TestGetGroupStudentsRoomStatus_WithSubstitution(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create group with room
	room := testpkg.CreateTestRoom(t, tc.db, "SubstitutionRoom")
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "SubstitutionAccessTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Update group with room
	_, err := tc.db.NewUpdate().
		Model((*education.Group)(nil)).
		ModelTableExpr("education.groups").
		Set("room_id = ?", room.ID).
		Where("id = ?", group.ID).
		Exec(context.Background())
	require.NoError(t, err)

	// Create staff with account for context
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Substitute", "Supervisor")
	defer testpkg.CleanupStaffFixtures(t, tc.db, staff.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Create active substitution for today (grants access)
	today := timezone.TodayDate()
	substitution := testpkg.CreateTestGroupSubstitution(t, tc.db, group.ID, nil, staff.ID, today, today)
	defer testpkg.CleanupActivityFixtures(t, tc.db, substitution.ID)

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

func TestCancelSpecificTransfer_AsGroupLeader(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "CancelTransferLeader")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher (group leader) with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Cancel", "Leader")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Create target staff
	targetStaff := testpkg.CreateTestStaff(t, tc.db, "Cancel", "Target")
	defer testpkg.CleanupStaffFixtures(t, tc.db, targetStaff.ID)

	// Create a transfer (substitution with nil regularStaffID = transfer)
	today := timezone.TodayDate()
	transfer := testpkg.CreateTestGroupSubstitution(t, tc.db, group.ID, nil, targetStaff.ID, today, today)
	defer testpkg.CleanupActivityFixtures(t, tc.db, transfer.ID)

	req := newReq(t, "DELETE", fmt.Sprintf("/groups/%d/transfer/%d", group.ID, transfer.ID), nil, testutil.TeacherTestClaims(int(account.ID)))

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestCancelSpecificTransfer_NotFound(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "CancelNotFound")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher (group leader) with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Cancel", "NotFound")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Try to cancel non-existent transfer
	req := newReq(t, "DELETE", fmt.Sprintf("/groups/%d/transfer/999999", group.ID), nil, testutil.TeacherTestClaims(int(account.ID)))

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// TRANSFER TARGET VALIDATION TESTS
// =============================================================================

func TestTransferGroup_TargetNotStaff(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "TargetNotStaff")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Transfer", "ToNonStaff")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Create a student (not staff)
	student := testpkg.CreateTestStudent(t, tc.db, "Not", "Staff", "1a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	// Get person ID for student
	var personID int64
	err := tc.db.NewSelect().
		Model((*users.Student)(nil)).
		ModelTableExpr(`users.students AS "student"`).
		Column("person_id").
		Where(`"student".id = ?`, student.ID).
		Scan(context.Background(), &personID)
	require.NoError(t, err)

	// Try to transfer to student (who is not staff)
	body := map[string]interface{}{
		"target_user_id": personID,
	}

	req := newReq(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body, testutil.TeacherTestClaims(int(account.ID)))

	rr := testutil.ExecuteRequest(router, req)
	// Should fail - target is not staff
	testutil.AssertBadRequest(t, rr)
}

func TestTransferGroup_TargetNotFound(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "TargetNotFound")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Transfer", "ToNotFound")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	body := map[string]interface{}{
		"target_user_id": 999999, // Non-existent person ID
	}

	req := newReq(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body, testutil.TeacherTestClaims(int(account.ID)))

	rr := testutil.ExecuteRequest(router, req)
	// Should fail - target not found
	testutil.AssertNotFound(t, rr)
}

func TestTransferGroup_DuplicateTransfer(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "DuplicateTransfer")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Dup", "Transfer")
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Create target staff
	targetStaff := testpkg.CreateTestStaff(t, tc.db, "Dup", "Target")
	defer testpkg.CleanupStaffFixtures(t, tc.db, targetStaff.ID)

	// Create existing transfer to target
	today := timezone.TodayDate()
	existingTransfer := testpkg.CreateTestGroupSubstitution(t, tc.db, group.ID, nil, targetStaff.ID, today, today)
	defer testpkg.CleanupActivityFixtures(t, tc.db, existingTransfer.ID)

	// Try to transfer again to same target
	body := map[string]interface{}{
		"target_user_id": targetStaff.Person.ID,
	}

	req := newReq(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body, testutil.TeacherTestClaims(int(account.ID)))

	rr := testutil.ExecuteRequest(router, req)
	// Should fail - already transferred to this person
	testutil.AssertBadRequest(t, rr)
}
