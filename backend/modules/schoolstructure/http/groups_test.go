// Package schoolstructurehttp_test tests the group routes with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package schoolstructurehttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	schoolstructurehttp "github.com/moto-nrw/project-phoenix/modules/schoolstructure/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults so jwt.MustNewTokenAuth (called by
// schoolstructurehttp.Resource.Router) succeeds in CI environments without a populated
// .env. Required because setupGroupsRoute constructs the resource, and the
// tests mint real signed JWTs via testutil.MintTestJWT.
func init() {
	testutil.SeedTestJWTConfig()
}

// newReq builds a request authenticated with a real signed JWT so the
// production middleware chain mounted by Resource.Router(rootAPI.ProtectedTenantRoutes)
// (Verifier → Authenticator → TenantMiddleware → RequiresPermission →
// TenantTxMiddleware) accepts it. The permission set carried by the token is
// exactly `permissions` — this mirrors the old testutil.WithPermissions option
// (empty for the transfer routes, which mount no permission check). All other
// claim fields come from `claims`.
func newReq(t *testing.T, method, target string, body interface{}, claims testutil.Claims, permissions ...string) *http.Request {
	t.Helper()
	claims.Permissions = permissions
	token := testutil.MintTestJWT(t, claims)
	return testutil.NewAuthenticatedRequest(t, method, target, body, testutil.WithJWTBearer(token))
}

// testContext holds shared test resources
type testContext struct {
	db       *bun.DB
	resource *schoolstructurehttp.Resource
}

// setupGroupsRoute creates test resources for groups handler tests
func setupGroupsRoute(t *testing.T, publishers ...*testpkg.RecordingBroadcaster) *testContext {
	t.Helper()

	db, svc, presence, people, caller := setupGroupsModule(t, publishers...)

	// Groups resource requires multiple services and repositories
	resource := schoolstructurehttp.NewResource(
		svc.Education,
		presence.Active,
		people.People,
		caller, transactionFailures.IsConstraintViolation)

	return &testContext{
		db:       db,
		resource: resource,
	}
}

// setupProtectedRouter mounts the production Resource.Router(rootAPI.ProtectedTenantRoutes) at /groups so
// tests exercise the real route + middleware wiring. Router() already includes
// the JWT chain and per-route permission checks; requests must carry a signed
// token (see newReq).
func setupProtectedRouter(t *testing.T, publishers ...*testpkg.RecordingBroadcaster) (*testContext, chi.Router) {
	t.Helper()

	tc := setupGroupsRoute(t, publishers...)

	router := chi.NewRouter()
	router.Mount("/groups", tc.resource.Router(rootAPI.ProtectedTenantRoutes))

	return tc, router
}

// groupListResponse is the part of the paginated GET /groups body the list
// tests assert on.
type groupListResponse struct {
	Data []struct {
		ID int64 `json:"id"`
	} `json:"data"`
	Pagination struct {
		PageSize     int `json:"page_size"`
		TotalPages   int `json:"total_pages"`
		TotalRecords int `json:"total_records"`
	} `json:"pagination"`
}

func decodeGroupList(t *testing.T, rr *httptest.ResponseRecorder) groupListResponse {
	t.Helper()
	var list groupListResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &list), rr.Body.String())
	return list
}

func (l groupListResponse) ids() []int64 {
	ids := make([]int64, 0, len(l.Data))
	for _, group := range l.Data {
		ids = append(ids, group.ID)
	}
	return ids
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

	match := testpkg.CreateTestEducationGroup(t, tc.db, "UniqueFilterName")
	other := testpkg.CreateTestEducationGroup(t, tc.db, "OtherGroup")

	req := newReq(t, "GET", "/groups?name=UniqueFilterName", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	ids := decodeGroupList(t, rr).ids()
	assert.Contains(t, ids, match.ID)
	assert.NotContains(t, ids, other.ID)
}

func TestListGroups_WithPagination(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)
	testpkg.CreateTestEducationGroup(t, tc.db, "PageOne")
	testpkg.CreateTestEducationGroup(t, tc.db, "PageTwo")

	req := newReq(t, "GET", "/groups?page=1&page_size=1", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	list := decodeGroupList(t, rr)
	assert.Len(t, list.Data, 1)
	assert.Equal(t, 1, list.Pagination.PageSize)
	assert.Equal(t, 2, list.Pagination.TotalRecords)
	assert.Equal(t, 2, list.Pagination.TotalPages)
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
		ModelTableExpr(`users.student_school_memberships AS "student"`).
		Column("group_id").
		Where(`"student".student_profile_id = ? AND "student".deleted_at IS NULL`, student.ID).
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
		Model((*testpkg.Student)(nil)).
		ModelTableExpr("users.student_school_memberships").
		Set("group_id = ?", group.ID).
		Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).
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

func TestListGroups_LargePageSize(t *testing.T) {
	t.Parallel()

	_, router := setupProtectedRouter(t)

	req := newReq(t, "GET", "/groups?page_size=1000", nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.Equal(t, 1000, decodeGroupList(t, rr).Pagination.PageSize, "page sizes below the maximum are not capped")
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

	// Assign the group and link the guardian through their owner records.
	guardian := testpkg.CreateTestGuardianProfileNamed(t, tc.db, "Test", "Guardian", "guardian@test.com")
	testpkg.CreateTestStudentGuardianLink(t, tc.db, student.ID, guardian.ID, "parent")
	_, err := tc.db.NewUpdate().
		Model((*testpkg.Student)(nil)).
		ModelTableExpr("users.student_school_memberships").
		Set("group_id = ?", group.ID).
		Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).
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
	withoutRoom := testpkg.CreateTestEducationGroup(t, tc.db, "NoRoomFilterTest")

	_, err := tc.db.NewUpdate().
		Model((*testpkg.EducationGroup)(nil)).
		ModelTableExpr("education.groups").
		Set("room_id = ?", room.ID).
		Where("id = ?", group.ID).
		Exec(context.Background())
	require.NoError(t, err)

	req := newReq(t, "GET", fmt.Sprintf("/groups?room_id=%d", room.ID), nil, testutil.DefaultTestClaims(), "groups:read")

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	list := decodeGroupList(t, rr)
	assert.Equal(t, []int64{group.ID}, list.ids())
	assert.NotContains(t, list.ids(), withoutRoom.ID)
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
// TRANSFER CANCEL TESTS
// =============================================================================
