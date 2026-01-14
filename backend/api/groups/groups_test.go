// Package groups_test tests the groups API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package groups_test

import (
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
