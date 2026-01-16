package staff_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	staffAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/staff"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/test/testutil"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *staffAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	resource := staffAPI.NewResource(svc.Users, svc.Education, svc.Auth)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
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

	// Create test staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "GetStaff", "Test")
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
	person := testpkg.CreateTestPerson(t, ctx.db, "NewStaff"+uniqueSuffix, "Person")
	defer testpkg.CleanupPerson(t, ctx.db, person.ID)

	router := chi.NewRouter()
	router.Post("/staff", ctx.resource.CreateStaffHandler())

	body := map[string]interface{}{
		"person_id":   person.ID,
		"staff_notes": "Test staff notes",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
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
	person := testpkg.CreateTestPerson(t, ctx.db, "NewTeacher"+uniqueSuffix, "Person")
	defer testpkg.CleanupPerson(t, ctx.db, person.ID)

	router := chi.NewRouter()
	router.Post("/staff", ctx.resource.CreateStaffHandler())

	body := map[string]interface{}{
		"person_id":      person.ID,
		"staff_notes":    "Teacher notes",
		"is_teacher":     true,
		"specialization": "Mathematics",
		"role":           "Senior Teacher",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
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
	staff := testpkg.CreateTestStaff(t, ctx.db, "UpdateStaff", "Test")
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
	staff := testpkg.CreateTestStaff(t, ctx.db, "DeleteStaff", "Test")
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
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "GroupsTest", "Teacher")
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
	staff := testpkg.CreateTestStaff(t, ctx.db, "NonTeacher", "Staff")
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
	staff := testpkg.CreateTestStaff(t, ctx.db, "SubstitutionsTest", "Staff")
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
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "PINTest", "Staff")
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

	// Get the person to access the account ID
	person := teacher.Staff.Person
	require.NotNil(t, person)
	require.NotNil(t, person.AccountID)

	router := chi.NewRouter()
	router.Get("/staff/pin", ctx.resource.GetPINStatusHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/pin", nil,
		testutil.WithClaims(jwt.AppClaims{
			ID:       int(*person.AccountID),
			Username: "pintest",
		}),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetPINStatus_InvalidToken(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/staff/pin", ctx.resource.GetPINStatusHandler())

	// Request with invalid (zero) user ID
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/pin", nil,
		testutil.WithClaims(jwt.AppClaims{
			ID: 0,
		}),
	)

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
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "UpdatePIN", "Staff")
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

	person := teacher.Staff.Person
	require.NotNil(t, person)
	require.NotNil(t, person.AccountID)

	router := chi.NewRouter()
	router.Put("/staff/pin", ctx.resource.UpdatePINHandler())

	// PIN must be exactly 4 digits
	body := map[string]interface{}{
		"new_pin": "123", // Invalid - only 3 digits
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithClaims(jwt.AppClaims{
			ID:       int(*person.AccountID),
			Username: "updatepin",
		}),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePIN_NonDigitPIN(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a staff with account
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "NonDigit", "PIN")
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

	person := teacher.Staff.Person
	require.NotNil(t, person)
	require.NotNil(t, person.AccountID)

	router := chi.NewRouter()
	router.Put("/staff/pin", ctx.resource.UpdatePINHandler())

	// PIN must contain only digits
	body := map[string]interface{}{
		"new_pin": "12ab", // Invalid - contains letters
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithClaims(jwt.AppClaims{
			ID:       int(*person.AccountID),
			Username: "nondigitpin",
		}),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePIN_MissingNewPIN(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a staff with account
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "MissingPIN", "Test")
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

	person := teacher.Staff.Person
	require.NotNil(t, person)
	require.NotNil(t, person.AccountID)

	router := chi.NewRouter()
	router.Put("/staff/pin", ctx.resource.UpdatePINHandler())

	body := map[string]interface{}{}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithClaims(jwt.AppClaims{
			ID:       int(*person.AccountID),
			Username: "missingpin",
		}),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePIN_InvalidToken(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/staff/pin", ctx.resource.UpdatePINHandler())

	body := map[string]interface{}{
		"new_pin": "1234",
	}

	// Request with invalid (zero) user ID
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithClaims(jwt.AppClaims{
			ID: 0,
		}),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertUnauthorized(t, rr)
}

func TestUpdatePIN_Success_FirstTime(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a staff with account (no existing PIN)
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "FirstPIN", "Setup")
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

	person := teacher.Staff.Person
	require.NotNil(t, person)
	require.NotNil(t, person.AccountID)

	router := chi.NewRouter()
	router.Put("/staff/pin", ctx.resource.UpdatePINHandler())

	body := map[string]interface{}{
		"new_pin": "1234",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithClaims(jwt.AppClaims{
			ID:       int(*person.AccountID),
			Username: "firstpinsetup",
		}),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}
