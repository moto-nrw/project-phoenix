package staff_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	staffAPI "github.com/moto-nrw/project-phoenix/api/staff"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *staffAPI.Resource
	ogsID    string
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	ogsID := testpkg.SetupTestOGS(t, db)

	// Create repo factory to get GroupSupervisor repository
	repoFactory := repositories.NewFactory(db)
	resource := staffAPI.NewResource(svc.Users, svc.Education, svc.Auth, repoFactory.GroupSupervisor)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
		ogsID:    ogsID,
	}
}

// =============================================================================
// LIST STAFF TESTS
// =============================================================================

func TestListStaff_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff", ctx.resource.ListStaffHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// The response should be an array (even if empty)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, "success", response["status"])
}

func TestListStaff_WithTeachersOnlyFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff", ctx.resource.ListStaffHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?teachers_only=true", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithNameFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff", ctx.resource.ListStaffHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?first_name=Test", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET STAFF TESTS
// =============================================================================

func TestGetStaff_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, ctx.db)

	// Create test staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "GetStaff", "Test", ogsID)
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	router := chi.NewRouter()
	router.Get("/staff/{id}", ctx.resource.GetStaffHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaff_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/{id}", ctx.resource.GetStaffHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/999999", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetStaff_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/{id}", ctx.resource.GetStaffHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// CREATE STAFF TESTS
// =============================================================================

func TestCreateStaff_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a person without staff record
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	person := testpkg.CreateTestPerson(t, ctx.db, "NewStaff"+uniqueSuffix, "Person", ctx.ogsID)
	defer testpkg.CleanupPerson(t, ctx.db, person.ID)

	router := chi.NewRouter()
	router.Use(testutil.TenantRLSMiddleware(ctx.db))
	router.Post("/staff", ctx.resource.CreateStaffHandler())

	body := map[string]interface{}{
		"person_id":   person.ID,
		"staff_notes": "Test staff notes",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:create"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Parse response to get staff ID for cleanup
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if data, ok := response["data"].(map[string]interface{}); ok {
		if staffID, ok := data["id"].(float64); ok {
			defer testpkg.CleanupStaffFixtures(t, ctx.db, int64(staffID))
		}
	}
}

func TestCreateStaff_AsTeacher(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a person without staff record
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	person := testpkg.CreateTestPerson(t, ctx.db, "NewTeacher"+uniqueSuffix, "Person", ctx.ogsID)
	defer testpkg.CleanupPerson(t, ctx.db, person.ID)

	router := chi.NewRouter()
	router.Use(testutil.TenantRLSMiddleware(ctx.db))
	router.Post("/staff", ctx.resource.CreateStaffHandler())

	body := map[string]interface{}{
		"person_id":      person.ID,
		"staff_notes":    "Teacher notes",
		"is_teacher":     true,
		"specialization": "Mathematics",
		"role":           "Senior Teacher",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:create"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Parse response to get staff ID for cleanup
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if data, ok := response["data"].(map[string]interface{}); ok {
		if staffID, ok := data["id"].(float64); ok {
			defer testpkg.CleanupStaffFixtures(t, ctx.db, int64(staffID))
		}
	}
}

func TestCreateStaff_PersonNotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/staff", ctx.resource.CreateStaffHandler())

	body := map[string]interface{}{
		"person_id": 999999,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:create"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestCreateStaff_MissingPersonID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/staff", ctx.resource.CreateStaffHandler())

	body := map[string]interface{}{
		"staff_notes": "Missing person ID",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:create"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// UPDATE STAFF TESTS
// =============================================================================

func TestUpdateStaff_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "UpdateStaff", "Test", ctx.ogsID)
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	router := chi.NewRouter()
	router.Put("/staff/{id}", ctx.resource.UpdateStaffHandler())

	body := map[string]interface{}{
		"person_id":   staff.PersonID,
		"staff_notes": "Updated notes",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateStaff_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/staff/{id}", ctx.resource.UpdateStaffHandler())

	body := map[string]interface{}{
		"person_id":   1,
		"staff_notes": "Should fail",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/999999", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdateStaff_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/staff/{id}", ctx.resource.UpdateStaffHandler())

	body := map[string]interface{}{
		"person_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/invalid", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DELETE STAFF TESTS
// =============================================================================

func TestDeleteStaff_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "DeleteStaff", "Test", ctx.ogsID)
	// Note: No defer cleanup needed since we're deleting it

	router := chi.NewRouter()
	router.Delete("/staff/{id}", ctx.resource.DeleteStaffHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:delete"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteStaff_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/staff/{id}", ctx.resource.DeleteStaffHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/staff/999999", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:delete"),
	)

	rr := testutil.ExecuteRequest(router, req)

	// NOTE: The delete operation returns success even for non-existent IDs.
	// This is idempotent behavior - deleting something that doesn't exist is still "successful".
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteStaff_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/staff/{id}", ctx.resource.DeleteStaffHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/staff/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:delete"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET STAFF GROUPS TESTS
// =============================================================================

func TestGetStaffGroups_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a teacher with the full chain (person -> staff -> teacher)
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "GroupsTest", "Teacher", ctx.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

	router := chi.NewRouter()
	router.Get("/staff/{id}/groups", ctx.resource.GetStaffGroupsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/staff/%d/groups", teacher.StaffID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaffGroups_NonTeacher(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create regular staff (not a teacher)
	staff := testpkg.CreateTestStaff(t, ctx.db, "NonTeacher", "Staff", ctx.ogsID)
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	router := chi.NewRouter()
	router.Get("/staff/{id}/groups", ctx.resource.GetStaffGroupsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/staff/%d/groups", staff.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return success with empty groups array for non-teachers
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaffGroups_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/{id}/groups", ctx.resource.GetStaffGroupsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/999999/groups", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// GET STAFF SUBSTITUTIONS TESTS
// =============================================================================

func TestGetStaffSubstitutions_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "SubstitutionsTest", "Staff", ctx.ogsID)
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	router := chi.NewRouter()
	router.Get("/staff/{id}/substitutions", ctx.resource.GetStaffSubstitutionsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/staff/%d/substitutions", staff.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaffSubstitutions_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/{id}/substitutions", ctx.resource.GetStaffSubstitutionsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/999999/substitutions", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// GET AVAILABLE STAFF TESTS
// =============================================================================

func TestGetAvailableStaff_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/available", ctx.resource.GetAvailableStaffHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/available", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET AVAILABLE FOR SUBSTITUTION TESTS
// =============================================================================

func TestGetAvailableForSubstitution_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/available-for-substitution", ctx.resource.GetAvailableForSubstitutionHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/available-for-substitution", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetAvailableForSubstitution_WithDateFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/available-for-substitution", ctx.resource.GetAvailableForSubstitutionHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/available-for-substitution?date=2024-01-15", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetAvailableForSubstitution_WithSearchFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/available-for-substitution", ctx.resource.GetAvailableForSubstitutionHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/available-for-substitution?search=Test", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET STAFF BY ROLE TESTS
// =============================================================================

func TestGetStaffByRole_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/by-role", ctx.resource.GetStaffByRoleHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/by-role?role=user", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaffByRole_MissingRoleParam(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/by-role", ctx.resource.GetStaffByRoleHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/by-role", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// PIN STATUS TESTS
// =============================================================================

func TestGetPINStatus_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a staff with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "PINTest", "Staff", ctx.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, ctx.db, account.ID)

	router := chi.NewRouter()
	router.Get("/staff/pin", ctx.resource.GetPINStatusHandler())

	// Use tenant context with account email (handler uses tenant.TenantFromCtx)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/pin", nil,
		testutil.WithTenantContext(testutil.SupervisorTenantContext(account.Email)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetPINStatus_InvalidToken(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/pin", ctx.resource.GetPINStatusHandler())

	// Request without tenant context should fail
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/pin", nil)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertUnauthorized(t, rr)
}

// =============================================================================
// UPDATE PIN TESTS
// =============================================================================

func TestUpdatePIN_InvalidPINFormat(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a staff with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "UpdatePIN", "Staff", ctx.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, ctx.db, account.ID)

	router := chi.NewRouter()
	router.Put("/staff/pin", ctx.resource.UpdatePINHandler())

	// PIN must be exactly 4 digits
	body := map[string]any{
		"new_pin": "123", // Invalid - only 3 digits
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithTenantContext(testutil.SupervisorTenantContext(account.Email)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePIN_NonDigitPIN(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a staff with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "NonDigit", "PIN", ctx.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, ctx.db, account.ID)

	router := chi.NewRouter()
	router.Put("/staff/pin", ctx.resource.UpdatePINHandler())

	// PIN must contain only digits
	body := map[string]any{
		"new_pin": "12ab", // Invalid - contains letters
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithTenantContext(testutil.SupervisorTenantContext(account.Email)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePIN_MissingNewPIN(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a staff with account
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "MissingPIN", "Test", ctx.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, ctx.db, account.ID)

	router := chi.NewRouter()
	router.Put("/staff/pin", ctx.resource.UpdatePINHandler())

	body := map[string]any{}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithTenantContext(testutil.SupervisorTenantContext(account.Email)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePIN_InvalidToken(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/staff/pin", ctx.resource.UpdatePINHandler())

	body := map[string]any{
		"new_pin": "1234",
	}

	// Request without tenant context should fail
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertUnauthorized(t, rr)
}

func TestUpdatePIN_Success_FirstTime(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a staff with account (no existing PIN)
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "FirstPIN", "Setup", ctx.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)
	defer testpkg.CleanupAuthFixtures(t, ctx.db, account.ID)

	router := chi.NewRouter()
	router.Put("/staff/pin", ctx.resource.UpdatePINHandler())

	body := map[string]any{
		"new_pin": "1234",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithTenantContext(testutil.SupervisorTenantContext(account.Email)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// ANWESEND FILTER TESTS (Staff Presence Tracking)
// =============================================================================

func TestListStaff_WithAnwesendFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff", ctx.resource.ListStaffHandler())

	// Test that the anwesend filter works (even if no staff are currently present)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?anwesend=true", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithRoleFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff", ctx.resource.ListStaffHandler())

	// Test filtering by role (the role filter branch)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?role=admin", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithLastNameFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff", ctx.resource.ListStaffHandler())

	// Test filtering by last name
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?last_name=Test", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithCombinedFilters(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff", ctx.resource.ListStaffHandler())

	// Test multiple filters combined
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?first_name=Test&last_name=Staff&teachers_only=true", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// UPDATE STAFF TO TEACHER TESTS (Teacher Record Update Paths)
// =============================================================================

func TestUpdateStaff_ConvertToTeacher(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create regular staff (not a teacher)
	staff := testpkg.CreateTestStaff(t, ctx.db, "ConvertTo", "Teacher", ctx.ogsID)
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	router := chi.NewRouter()
	router.Put("/staff/{id}", ctx.resource.UpdateStaffHandler())

	// Update to convert to teacher
	body := map[string]interface{}{
		"person_id":      staff.PersonID,
		"staff_notes":    "Converting to teacher",
		"is_teacher":     true,
		"specialization": "Mathematics",
		"role":           "Teacher",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:update"),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify it's now a teacher response
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if data, ok := response["data"].(map[string]any); ok {
		// Check for teacher_id field which indicates it's a teacher response
		_, hasTeacherID := data["teacher_id"]
		assert.True(t, hasTeacherID || data["is_teacher"] == true)
	}
}

func TestUpdateStaff_UpdateExistingTeacher(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a teacher with the full chain
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "UpdateTeacher", "Test", ctx.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

	router := chi.NewRouter()
	router.Put("/staff/{id}", ctx.resource.UpdateStaffHandler())

	// Update teacher fields
	body := map[string]interface{}{
		"person_id":      teacher.Staff.PersonID,
		"staff_notes":    "Updated teacher notes",
		"is_teacher":     true,
		"specialization": "Physics", // Changed specialization
		"role":           "Senior Teacher",
		"qualifications": "Ph.D. Physics",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", teacher.StaffID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateStaff_KeepExistingTeacherWithoutIsTeacher(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a teacher
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "KeepTeacher", "Test", ctx.ogsID)
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

	router := chi.NewRouter()
	router.Put("/staff/{id}", ctx.resource.UpdateStaffHandler())

	// Update without is_teacher flag - should still return teacher response
	body := map[string]interface{}{
		"person_id":   teacher.Staff.PersonID,
		"staff_notes": "Updated notes only",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", teacher.StaffID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateStaff_InvalidRequest(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	staff := testpkg.CreateTestStaff(t, ctx.db, "InvalidReq", "Staff", ctx.ogsID)
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	router := chi.NewRouter()
	router.Put("/staff/{id}", ctx.resource.UpdateStaffHandler())

	// Invalid request - missing person_id
	body := map[string]interface{}{
		"staff_notes": "Missing person ID",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdateStaff_ChangePersonID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create two persons
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	person1 := testpkg.CreateTestPerson(t, ctx.db, "Original"+uniqueSuffix, "Person", ctx.ogsID)
	person2 := testpkg.CreateTestPerson(t, ctx.db, "New"+uniqueSuffix, "Person", ctx.ogsID)
	defer testpkg.CleanupPerson(t, ctx.db, person1.ID)
	defer testpkg.CleanupPerson(t, ctx.db, person2.ID)

	// Create staff with person1
	staff := testpkg.CreateTestStaffForPerson(t, ctx.db, person1.ID, ctx.ogsID)
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	router := chi.NewRouter()
	router.Put("/staff/{id}", ctx.resource.UpdateStaffHandler())

	// Update to point to person2
	body := map[string]interface{}{
		"person_id":   person2.ID,
		"staff_notes": "Changed person",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateStaff_ChangeToNonExistentPerson(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	staff := testpkg.CreateTestStaff(t, ctx.db, "InvalidPerson", "Staff", ctx.ogsID)
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	router := chi.NewRouter()
	router.Put("/staff/{id}", ctx.resource.UpdateStaffHandler())

	// Try to update to non-existent person
	body := map[string]interface{}{
		"person_id":   999999,
		"staff_notes": "Should fail",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// DELETE TEACHER TESTS (Delete Staff Who Is Also Teacher)
// =============================================================================

func TestDeleteStaff_WhoIsTeacher(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a teacher (this creates person -> staff -> teacher chain)
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "DeleteTeacher", "Test", ctx.ogsID)
	// Note: No defer cleanup as we're deleting

	router := chi.NewRouter()
	router.Delete("/staff/{id}", ctx.resource.DeleteStaffHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/staff/%d", teacher.StaffID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:delete"),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should successfully delete both teacher and staff records
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET STAFF GROUPS - INVALID ID TEST
// =============================================================================

func TestGetStaffGroups_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/{id}/groups", ctx.resource.GetStaffGroupsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/invalid/groups", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET STAFF SUBSTITUTIONS - INVALID ID TEST
// =============================================================================

func TestGetStaffSubstitutions_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/{id}/substitutions", ctx.resource.GetStaffSubstitutionsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/invalid/substitutions", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET AVAILABLE FOR SUBSTITUTION - INVALID DATE TEST
// =============================================================================

func TestGetAvailableForSubstitution_WithInvalidDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/available-for-substitution", ctx.resource.GetAvailableForSubstitutionHandler())

	// Invalid date format should fall back to current date
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/available-for-substitution?date=invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should still succeed with fallback to current date
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}
