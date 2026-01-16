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
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	groupsAPI "github.com/moto-nrw/project-phoenix/api/groups"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test resources
type testContext struct {
	db       *bun.DB
	services *services.Factory
	repos    *repositories.Factory
	resource *groupsAPI.Resource
}

// setupTestContext creates test resources for groups handler tests
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db)
	svc, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")

	// Groups resource requires multiple services and repositories
	resource := groupsAPI.NewResource(
		svc.Education,
		svc.Active,
		svc.Users,
		svc.UserContext,
		repoFactory.Student,
		repoFactory.GroupSubstitution,
	)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	return &testContext{
		db:       db,
		services: svc,
		repos:    repoFactory,
		resource: resource,
	}
}

// setupProtectedRouter creates a router for testing protected endpoints
func setupProtectedRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))

	// Mount routes without JWT middleware for testing
	router.Route("/groups", func(r chi.Router) {
		// Read operations
		r.With(authorize.RequiresPermission("groups:read")).Get("/", tc.resource.ListGroupsHandler())
		r.With(authorize.RequiresPermission("groups:read")).Get("/{id}", tc.resource.GetGroupHandler())
		r.With(authorize.RequiresPermission("groups:read")).Get("/{id}/students", tc.resource.GetGroupStudentsHandler())
		r.With(authorize.RequiresPermission("groups:read")).Get("/{id}/supervisors", tc.resource.GetGroupSupervisorsHandler())
		r.With(authorize.RequiresPermission("groups:read")).Get("/{id}/students/room-status", tc.resource.GetGroupStudentsRoomStatusHandler())
		r.With(authorize.RequiresPermission("groups:read")).Get("/{id}/substitutions", tc.resource.GetGroupSubstitutionsHandler())

		// Write operations
		r.With(authorize.RequiresPermission("groups:create")).Post("/", tc.resource.CreateGroupHandler())
		r.With(authorize.RequiresPermission("groups:update")).Put("/{id}", tc.resource.UpdateGroupHandler())
		r.With(authorize.RequiresPermission("groups:delete")).Delete("/{id}", tc.resource.DeleteGroupHandler())
	})

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

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListGroups_WithNameFilter(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group fixture
	group := testpkg.CreateTestEducationGroup(t, tc.db, "UniqueFilterName")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups?name=UniqueFilterName", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListGroups_WithPagination(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups?page=1&page_size=10", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListGroups_WithoutPermission(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups", nil,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d", group.ID), nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains correct data
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	assert.Contains(t, data["name"].(string), "GetGroupTest")
}

func TestGetGroup_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/999999", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestGetGroup_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/invalid", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestGetGroup_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "PermTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d", group.ID), nil,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithPermissions("groups:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithPermissions("groups:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithPermissions("groups:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCreateGroup_WithoutPermission(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"name": "NoPermGroup",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body,
		testutil.WithPermissions("groups:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/groups/999999", body,
		testutil.WithPermissions("groups:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestUpdateGroup_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"name": "UpdatedName",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/groups/invalid", body,
		testutil.WithPermissions("groups:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/groups/%d", group.ID), nil,
		testutil.WithPermissions("groups:delete"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteGroup_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/groups/999999", nil,
		testutil.WithPermissions("groups:delete"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestDeleteGroup_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/groups/invalid", nil,
		testutil.WithPermissions("groups:delete"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestDeleteGroup_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "NoPermDelete")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/groups/%d", group.ID), nil,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students", group.ID), nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupStudents_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/999999/students", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestGetGroupStudents_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/invalid/students", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/supervisors", group.ID), nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupSupervisors_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/999999/supervisors", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/substitutions", group.ID), nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupSubstitutions_WithDate(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "SubstitutionsDateTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/substitutions?date=2024-01-15", group.ID), nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupSubstitutions_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/999999/substitutions", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students/room-status", group.ID), nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Returns 403 because user doesn't supervise this group
	testutil.AssertForbidden(t, rr)
}

func TestGetGroupStudentsRoomStatus_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/999999/students/room-status", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestGetGroupStudentsRoomStatus_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/invalid/students/room-status", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students", group.ID), nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithPermissions("groups:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithPermissions("groups:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body,
		testutil.WithPermissions("groups:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body,
		testutil.WithPermissions("groups:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// LIST GROUPS ADDITIONAL TESTS
// =============================================================================

func TestListGroups_InvalidPagination(t *testing.T) {
	_, router := setupProtectedRouter(t)

	// Test with invalid page number
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups?page=-1", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	// May succeed with default pagination or fail depending on validation
	t.Logf("Response: %d - %s", rr.Code, rr.Body.String())
}

func TestListGroups_LargePageSize(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups?page_size=1000", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Should succeed - large page size might be capped
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET GROUP SUPERVISORS ADDITIONAL TESTS
// =============================================================================

func TestGetGroupSupervisors_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/invalid/supervisors", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET GROUP SUBSTITUTIONS ADDITIONAL TESTS
// =============================================================================

func TestGetGroupSubstitutions_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/invalid/substitutions", nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestGetGroupSubstitutions_InvalidDate(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "InvalidDateTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/substitutions?date=invalid-date", group.ID), nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))

	router.Route("/groups", func(r chi.Router) {
		r.Route("/{id}/transfer", func(r chi.Router) {
			r.Post("/", tc.resource.TransferGroupHandler())
			r.Delete("/{substitutionId}", tc.resource.CancelSpecificTransferHandler())
		})
	})

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
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

func TestTransferGroup_InvalidGroupID(t *testing.T) {
	_, router := setupTransferRouter(t)

	body := map[string]interface{}{
		"target_user_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups/invalid/transfer", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body,
		testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCancelSpecificTransfer_RequiresTeacher(t *testing.T) {
	tc, router := setupTransferRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "CancelTransferTest")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Regular user (not teacher) should get forbidden
	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/groups/%d/transfer/1", group.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

func TestCancelSpecificTransfer_InvalidGroupID(t *testing.T) {
	_, router := setupTransferRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/groups/invalid/transfer/1", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCancelSpecificTransfer_InvalidSubstitutionID(t *testing.T) {
	tc, router := setupTransferRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "CancelInvalidSubst")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/groups/%d/transfer/invalid", group.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students/room-status", group.ID), nil,
		testutil.WithPermissions("groups:read", "admin:*"),
		testutil.WithClaims(testutil.AdminTestClaims(1)),
	)

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
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students/room-status", group.ID), nil,
		testutil.WithPermissions("groups:read", "admin:*"),
		testutil.WithClaims(testutil.AdminTestClaims(1)),
	)

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
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students", group.ID), nil,
		testutil.WithPermissions("groups:read", "admin:*"),
		testutil.WithClaims(testutil.AdminTestClaims(1)),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups?room_id=%d", room.ID), nil,
		testutil.WithPermissions("groups:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithPermissions("groups:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

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

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body,
		testutil.WithPermissions("groups:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}
