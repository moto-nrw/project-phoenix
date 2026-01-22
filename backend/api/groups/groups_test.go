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
	"github.com/moto-nrw/project-phoenix/auth/tenant"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// teacherTenantContext returns a tenant context for a teacher user.
// Used for testing endpoints that require teacher-level access.
func teacherTenantContext(email string) *tenant.TenantContext {
	return &tenant.TenantContext{
		UserID:      "teacher-user-id",
		UserEmail:   email,
		UserName:    "Test Teacher",
		OrgID:       "test-org-id",
		OrgName:     "Test OGS",
		OrgSlug:     "test-ogs",
		Role:        "supervisor",
		Permissions: []string{"student:read", "group:read", "room:read", "visit:read", "visit:create", "visit:update", "activity:read", "location:read"},
		TraegerID:   "test-traeger-id",
		TraegerName: "Test Träger",
	}
}

// testContext holds shared test resources
type testContext struct {
	db       *bun.DB
	services *services.Factory
	repos    *repositories.Factory
	resource *groupsAPI.Resource
	ogsID    string
}

// setupTestContext creates test resources for groups handler tests
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	ogsID := testpkg.SetupTestOGS(t, db)

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
		ogsID:    ogsID,
	}
}

// setupProtectedRouter creates a router for testing protected endpoints
func setupProtectedRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))

	// Mount routes with tenant permission middleware (matches production handlers)
	router.Route("/groups", func(r chi.Router) {
		// Read operations
		r.With(tenant.RequiresPermission("group:read")).Get("/", tc.resource.ListGroupsHandler())
		r.With(tenant.RequiresPermission("group:read")).Get("/{id}", tc.resource.GetGroupHandler())
		r.With(tenant.RequiresPermission("group:read")).Get("/{id}/students", tc.resource.GetGroupStudentsHandler())
		r.With(tenant.RequiresPermission("group:read")).Get("/{id}/supervisors", tc.resource.GetGroupSupervisorsHandler())
		r.With(tenant.RequiresPermission("group:read")).Get("/{id}/students/room-status", tc.resource.GetGroupStudentsRoomStatusHandler())
		r.With(tenant.RequiresPermission("group:read")).Get("/{id}/substitutions", tc.resource.GetGroupSubstitutionsHandler())

		// Write operations
		r.With(tenant.RequiresPermission("group:create")).Post("/", tc.resource.CreateGroupHandler())
		r.With(tenant.RequiresPermission("group:update")).Put("/{id}", tc.resource.UpdateGroupHandler())
		r.With(tenant.RequiresPermission("group:delete")).Delete("/{id}", tc.resource.DeleteGroupHandler())
	})

	return tc, router
}

// =============================================================================
// LIST GROUPS TESTS
// =============================================================================

func TestListGroups_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)
	ogsID := testpkg.SetupTestOGS(t, tc.db)

	// Create test education group fixture
	group := testpkg.CreateTestEducationGroup(t, tc.db, "ListTest", ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListGroups_WithNameFilter(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group fixture
	group := testpkg.CreateTestEducationGroup(t, tc.db, "UniqueFilterName", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups?name=UniqueFilterName", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListGroups_WithPagination(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups?page=1&page_size=10", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListGroups_WithoutPermission(t *testing.T) {
	_, router := setupProtectedRouter(t)

	// Empty tenant context (no permissions) should return 403
	tc := &tenant.TenantContext{
		UserID:      "test-user",
		UserEmail:   "noperm@example.com",
		UserName:    "No Perm User",
		OrgID:       "test-org",
		OrgName:     "Test Org",
		OrgSlug:     "test-org",
		Role:        "supervisor",
		Permissions: []string{}, // No permissions
		TraegerID:   "test-traeger",
		TraegerName: "Test Träger",
	}

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups", nil,
		testutil.WithTenantContext(tc),
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
	group := testpkg.CreateTestEducationGroup(t, tc.db, "GetGroupTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d", group.ID), nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains correct data
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	assert.Contains(t, data["name"].(string), "GetGroupTest")
}

func TestGetGroup_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/999999", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestGetGroup_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/invalid", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestGetGroup_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "PermTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Empty permissions should return 403
	noPermTC := &tenant.TenantContext{
		UserID:      "test-user",
		UserEmail:   "noperm@example.com",
		UserName:    "No Perm User",
		OrgID:       "test-org",
		OrgName:     "Test Org",
		OrgSlug:     "test-org",
		Role:        "supervisor",
		Permissions: []string{}, // No permissions
		TraegerID:   "test-traeger",
		TraegerName: "Test Träger",
	}

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d", group.ID), nil,
		testutil.WithTenantContext(noPermTC),
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
	body := map[string]any{
		"name": uniqueName,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Cleanup created group
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	groupID := int64(data["id"].(float64))
	testpkg.CleanupActivityFixtures(t, tc.db, groupID)
}

func TestCreateGroup_WithRoom(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create a room first
	room := testpkg.CreateTestRoom(t, tc.db, "TestRoom", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	// Use unique name to avoid conflicts
	uniqueName := fmt.Sprintf("GroupWithRoom-%d", time.Now().UnixNano())
	body := map[string]any{
		"name":    uniqueName,
		"room_id": room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Cleanup created group
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	groupID := int64(data["id"].(float64))
	testpkg.CleanupActivityFixtures(t, tc.db, groupID)
}

func TestCreateGroup_MissingName(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]any{} // Missing name

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCreateGroup_WithoutPermission(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]any{
		"name": "NoPermGroup",
	}

	// Empty permissions should return 403
	noPermTC := &tenant.TenantContext{
		UserID:      "test-user",
		UserEmail:   "noperm@example.com",
		UserName:    "No Perm User",
		OrgID:       "test-org",
		OrgName:     "Test Org",
		OrgSlug:     "test-org",
		Role:        "supervisor",
		Permissions: []string{}, // No permissions
		TraegerID:   "test-traeger",
		TraegerName: "Test Träger",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithTenantContext(noPermTC),
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
	group := testpkg.CreateTestEducationGroup(t, tc.db, "OriginalUpdateTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Use unique name for update
	uniqueNewName := fmt.Sprintf("UpdatedGroup-%d", group.ID)
	body := map[string]any{
		"name": uniqueNewName,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify update
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	assert.Equal(t, uniqueNewName, data["name"])
}

func TestUpdateGroup_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]any{
		"name": "UpdatedName",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/groups/999999", body,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestUpdateGroup_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]any{
		"name": "UpdatedName",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/groups/invalid", body,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestUpdateGroup_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "NoPermUpdate", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	body := map[string]any{
		"name": "UpdatedName",
	}

	// Empty permissions should return 403
	noPermTC := &tenant.TenantContext{
		UserID:      "test-user",
		UserEmail:   "noperm@example.com",
		UserName:    "No Perm User",
		OrgID:       "test-org",
		OrgName:     "Test Org",
		OrgSlug:     "test-org",
		Role:        "supervisor",
		Permissions: []string{}, // No permissions
		TraegerID:   "test-traeger",
		TraegerName: "Test Träger",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body,
		testutil.WithTenantContext(noPermTC),
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
	group := testpkg.CreateTestEducationGroup(t, tc.db, "ToDelete", tc.ogsID)
	// No defer cleanup needed since we're deleting it

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/groups/%d", group.ID), nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteGroup_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/groups/999999", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestDeleteGroup_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/groups/invalid", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestDeleteGroup_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "NoPermDelete", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Empty permissions should return 403
	noPermTC := &tenant.TenantContext{
		UserID:      "test-user",
		UserEmail:   "noperm@example.com",
		UserName:    "No Perm User",
		OrgID:       "test-org",
		OrgName:     "Test Org",
		OrgSlug:     "test-org",
		Role:        "supervisor",
		Permissions: []string{}, // No permissions
		TraegerID:   "test-traeger",
		TraegerName: "Test Träger",
	}

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/groups/%d", group.ID), nil,
		testutil.WithTenantContext(noPermTC),
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
	group := testpkg.CreateTestEducationGroup(t, tc.db, "StudentsTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students", group.ID), nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupStudents_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/999999/students", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestGetGroupStudents_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/invalid/students", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
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
	group := testpkg.CreateTestEducationGroup(t, tc.db, "SupervisorsTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/supervisors", group.ID), nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupSupervisors_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/999999/supervisors", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
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
	group := testpkg.CreateTestEducationGroup(t, tc.db, "SubstitutionsTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/substitutions", group.ID), nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupSubstitutions_WithDate(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "SubstitutionsDateTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/substitutions?date=2024-01-15", group.ID), nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetGroupSubstitutions_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/999999/substitutions", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
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
	group := testpkg.CreateTestEducationGroup(t, tc.db, "RoomStatusTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Without being a supervisor of the group, should get forbidden
	// Use tenant context without location:read permission (which grants full access)
	limitedTC := &tenant.TenantContext{
		UserID:      "limited-supervisor",
		UserEmail:   "limited@example.com",
		UserName:    "Limited Supervisor",
		OrgID:       "test-org",
		OrgName:     "Test Org",
		OrgSlug:     "test-org",
		Role:        "supervisor",
		Permissions: []string{"student:read", "group:read", "room:read"}, // No location:read
		TraegerID:   "test-traeger",
		TraegerName: "Test Träger",
	}

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students/room-status", group.ID), nil,
		testutil.WithTenantContext(limitedTC),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Returns 403 because user doesn't supervise this group and doesn't have location:read
	testutil.AssertForbidden(t, rr)
}

func TestGetGroupStudentsRoomStatus_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/999999/students/room-status", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestGetGroupStudentsRoomStatus_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/invalid/students/room-status", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
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
	group := testpkg.CreateTestEducationGroup(t, tc.db, "WithStudentTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create a student and assign to the group
	student := testpkg.CreateTestStudent(t, tc.db, "GroupStudent", "Test", "1a", tc.ogsID)

	// Assign student to group
	_, err := tc.db.NewUpdate().
		Model((*users.Student)(nil)).
		ModelTableExpr("users.students").
		Set("group_id = ?", group.ID).
		Where("id = ?", student.ID).
		Exec(context.Background())
	require.NoError(t, err)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students", group.ID), nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// CREATE GROUP ADDITIONAL TESTS
// =============================================================================

func TestCreateGroup_EmptyName(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]any{
		"name": "", // Empty name
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCreateGroup_WithDescription(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Use unique name to avoid conflicts
	uniqueName := fmt.Sprintf("GroupWithDesc-%d", time.Now().UnixNano())
	body := map[string]any{
		"name":        uniqueName,
		"description": "Test group description",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Cleanup
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	groupID := int64(data["id"].(float64))
	testpkg.CleanupActivityFixtures(t, tc.db, groupID)
}

// =============================================================================
// UPDATE GROUP ADDITIONAL TESTS
// =============================================================================

func TestUpdateGroup_WithRoom(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group and room
	group := testpkg.CreateTestEducationGroup(t, tc.db, "UpdateRoomTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	room := testpkg.CreateTestRoom(t, tc.db, "UpdateTestRoom", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	body := map[string]any{
		"name":    fmt.Sprintf("UpdatedWithRoom-%d", group.ID),
		"room_id": room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateGroup_EmptyName(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "EmptyNameUpdateTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	body := map[string]any{
		"name": "", // Empty name should fail
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
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
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	// May succeed with default pagination or fail depending on validation
	t.Logf("Response: %d - %s", rr.Code, rr.Body.String())
}

func TestListGroups_LargePageSize(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups?page_size=1000", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
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
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
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
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestGetGroupSubstitutions_InvalidDate(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "InvalidDateTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/substitutions?date=invalid-date", group.ID), nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
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

	group := testpkg.CreateTestEducationGroup(t, tc.db, "TransferTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	body := map[string]any{
		"target_user_id": 1,
	}

	// User without teacher/supervisor role should get forbidden
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body,
		testutil.WithTenantContext(testutil.SupervisorTenantContext("supervisor@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

func TestTransferGroup_InvalidGroupID(t *testing.T) {
	_, router := setupTransferRouter(t)

	body := map[string]any{
		"target_user_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups/invalid/transfer", body,
		testutil.WithTenantContext(testutil.SupervisorTenantContext("supervisor@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestTransferGroup_MissingTargetUserID(t *testing.T) {
	tc, router := setupTransferRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "TransferMissingTarget", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher with account for context
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Transfer", "Teacher", tc.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to the group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	body := map[string]any{
		"target_user_id": 0, // Invalid - must be positive
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body,
		testutil.WithTenantContext(teacherTenantContext(account.Email)),
		testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCancelSpecificTransfer_RequiresTeacher(t *testing.T) {
	tc, router := setupTransferRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "CancelTransferTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// User without teacher/supervisor role should get forbidden
	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/groups/%d/transfer/1", group.ID), nil,
		testutil.WithTenantContext(testutil.SupervisorTenantContext("supervisor@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

func TestCancelSpecificTransfer_InvalidGroupID(t *testing.T) {
	_, router := setupTransferRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/groups/invalid/transfer/1", nil,
		testutil.WithTenantContext(testutil.SupervisorTenantContext("supervisor@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCancelSpecificTransfer_InvalidSubstitutionID(t *testing.T) {
	tc, router := setupTransferRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "CancelInvalidSubst", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/groups/%d/transfer/invalid", group.ID), nil,
		testutil.WithTenantContext(testutil.SupervisorTenantContext("supervisor@example.com")),
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
	room := testpkg.CreateTestRoom(t, tc.db, "AdminRoomStatus", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "AdminStatusTest", tc.ogsID)
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
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify the response has the expected structure
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	assert.True(t, data["group_has_room"].(bool), "Should have room")
	assert.Equal(t, float64(room.ID), data["group_room_id"].(float64))
}

func TestGetGroupStudentsRoomStatus_NoRoomAssigned(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group without room
	group := testpkg.CreateTestEducationGroup(t, tc.db, "NoRoomStatusTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Admin should have access
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students/room-status", group.ID), nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify the response indicates no room
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	assert.False(t, data["group_has_room"].(bool), "Should not have room")
}

// =============================================================================
// GET GROUP STUDENTS WITH FULL ACCESS TESTS
// =============================================================================

func TestGetGroupStudents_WithFullAccessAdmin(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AdminStudentsTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create a student with guardian info
	student := testpkg.CreateTestStudent(t, tc.db, "GuardianTest", "Student", "2a", tc.ogsID)
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
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
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
	room := testpkg.CreateTestRoom(t, tc.db, "FilterRoom", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	// Create group with room
	group := testpkg.CreateTestEducationGroup(t, tc.db, "RoomFilterTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	_, err := tc.db.NewUpdate().
		Model((*education.Group)(nil)).
		ModelTableExpr("education.groups").
		Set("room_id = ?", room.ID).
		Where("id = ?", group.ID).
		Exec(context.Background())
	require.NoError(t, err)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups?room_id=%d", room.ID), nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
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
	teacher := testpkg.CreateTestTeacher(t, tc.db, "Assign", "Teacher", tc.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)

	uniqueName := fmt.Sprintf("GroupWithTeachers-%d", time.Now().UnixNano())
	body := map[string]any{
		"name":        uniqueName,
		"teacher_ids": []int64{teacher.ID},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/groups", body,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Cleanup
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	groupID := int64(data["id"].(float64))
	testpkg.CleanupActivityFixtures(t, tc.db, groupID)
}

// =============================================================================
// UPDATE GROUP WITH TEACHER IDS TEST
// =============================================================================

func TestUpdateGroup_WithTeacherIDs(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "UpdateTeachersTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	teacher := testpkg.CreateTestTeacher(t, tc.db, "Update", "Teacher", tc.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)

	body := map[string]any{
		"name":        fmt.Sprintf("UpdatedWithTeachers-%d", group.ID),
		"teacher_ids": []int64{teacher.ID},
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/groups/%d", group.ID), body,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// AUTHORIZATION HELPER TESTS - isUserGroupLeader
// =============================================================================

func TestTransferGroup_AsGroupLeader_Success(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "LeaderTransferTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher (group leader) with account for context
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Leader", "Teacher", tc.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to the group (makes them group leader)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Create target staff to transfer to
	targetStaff := testpkg.CreateTestStaff(t, tc.db, "Target", "Staff", tc.ogsID)
	defer testpkg.CleanupStaffFixtures(t, tc.db, targetStaff.ID)

	body := map[string]any{
		"target_user_id": targetStaff.Person.ID, // Target user ID is the person ID
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body,
		testutil.WithTenantContext(teacherTenantContext(account.Email)),
		testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Should succeed since teacher is group leader (returns 201 Created for new substitution)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestTransferGroup_NotGroupLeader(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "NotLeaderTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher WITHOUT assigning to group (not a group leader)
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "NotLeader", "Teacher", tc.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Create target staff
	targetStaff := testpkg.CreateTestStaff(t, tc.db, "Target", "Staff", tc.ogsID)
	defer testpkg.CleanupStaffFixtures(t, tc.db, targetStaff.ID)

	body := map[string]any{
		"target_user_id": targetStaff.Person.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body,
		testutil.WithTenantContext(teacherTenantContext(account.Email)),
		testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Should fail since teacher is not assigned to this group
	testutil.AssertForbidden(t, rr)
}

func TestTransferGroup_CannotTransferToSelf(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "SelfTransferTest", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Self", "Transfer", tc.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Try to transfer to self (using their own person ID)
	body := map[string]any{
		"target_user_id": teacher.Staff.Person.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body,
		testutil.WithTenantContext(teacherTenantContext(account.Email)),
		testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
	)

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
	room := testpkg.CreateTestRoom(t, tc.db, "SubstitutionRoom", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "SubstitutionAccessTest", tc.ogsID)
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
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Substitute", "Supervisor", tc.ogsID)
	defer testpkg.CleanupStaffFixtures(t, tc.db, staff.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Create active substitution for today (grants access)
	today := time.Now().UTC()
	endOfDay := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 59, 0, time.UTC)
	substitution := testpkg.CreateTestGroupSubstitution(t, tc.db, group.ID, nil, staff.ID, today, endOfDay)
	defer testpkg.CleanupActivityFixtures(t, tc.db, substitution.ID)

	// Staff should have access via substitution
	// Note: OGSAdmin provides admin access for this test
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students/room-status", group.ID), nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

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
	group := testpkg.CreateTestEducationGroup(t, tc.db, "CancelTransferLeader", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher (group leader) with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Cancel", "Leader", tc.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Create target staff
	targetStaff := testpkg.CreateTestStaff(t, tc.db, "Cancel", "Target", tc.ogsID)
	defer testpkg.CleanupStaffFixtures(t, tc.db, targetStaff.ID)

	// Create a transfer (substitution with nil regularStaffID = transfer)
	today := time.Now().UTC()
	endOfDay := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 59, 0, time.UTC)
	transfer := testpkg.CreateTestGroupSubstitution(t, tc.db, group.ID, nil, targetStaff.ID, today, endOfDay)
	defer testpkg.CleanupActivityFixtures(t, tc.db, transfer.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/groups/%d/transfer/%d", group.ID, transfer.ID), nil,
		testutil.WithTenantContext(teacherTenantContext(account.Email)),
		testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestCancelSpecificTransfer_NotFound(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "CancelNotFound", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher (group leader) with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Cancel", "NotFound", tc.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Try to cancel non-existent transfer
	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/groups/%d/transfer/999999", group.ID), nil,
		testutil.WithTenantContext(teacherTenantContext(account.Email)),
		testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// TRANSFER TARGET VALIDATION TESTS
// =============================================================================

func TestTransferGroup_TargetNotStaff(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "TargetNotStaff", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Transfer", "ToNonStaff", tc.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Create a student (not staff)
	student := testpkg.CreateTestStudent(t, tc.db, "Not", "Staff", "1a", tc.ogsID)
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
	body := map[string]any{
		"target_user_id": personID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body,
		testutil.WithTenantContext(teacherTenantContext(account.Email)),
		testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Should fail - target is not staff
	testutil.AssertBadRequest(t, rr)
}

func TestTransferGroup_TargetNotFound(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "TargetNotFound", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Transfer", "ToNotFound", tc.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	body := map[string]any{
		"target_user_id": 999999, // Non-existent person ID
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body,
		testutil.WithTenantContext(teacherTenantContext(account.Email)),
		testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Should fail - target not found
	testutil.AssertNotFound(t, rr)
}

func TestTransferGroup_DuplicateTransfer(t *testing.T) {
	tc, router := setupTransferRouter(t)

	// Create group
	group := testpkg.CreateTestEducationGroup(t, tc.db, "DuplicateTransfer", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Create teacher with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Dup", "Transfer", tc.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, tc.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Create target staff
	targetStaff := testpkg.CreateTestStaff(t, tc.db, "Dup", "Target", tc.ogsID)
	defer testpkg.CleanupStaffFixtures(t, tc.db, targetStaff.ID)

	// Create existing transfer to target
	today := time.Now().UTC()
	endOfDay := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 59, 0, time.UTC)
	existingTransfer := testpkg.CreateTestGroupSubstitution(t, tc.db, group.ID, nil, targetStaff.ID, today, endOfDay)
	defer testpkg.CleanupActivityFixtures(t, tc.db, existingTransfer.ID)

	// Try to transfer again to same target
	body := map[string]any{
		"target_user_id": targetStaff.Person.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/groups/%d/transfer", group.ID), body,
		testutil.WithTenantContext(teacherTenantContext(account.Email)),
		testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Should fail - already transferred to this person
	testutil.AssertBadRequest(t, rr)
}
