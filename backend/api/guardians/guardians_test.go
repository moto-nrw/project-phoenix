// Package guardians_test tests the guardians API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package guardians_test

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

	guardiansAPI "github.com/moto-nrw/project-phoenix/api/guardians"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *guardiansAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	// Create repository factory for student repository
	repoFactory := repositories.NewFactory(db)

	resource := guardiansAPI.NewResource(
		svc.Guardian,
		svc.Users,
		svc.Education,
		svc.UserContext,
		repoFactory.Student,
	)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// cleanupGuardian cleans up a guardian profile and related records
func cleanupGuardian(t *testing.T, db *bun.DB, guardianID int64) {
	t.Helper()
	ctx := context.Background()

	// Delete student-guardian relationships
	_, _ = db.NewDelete().
		TableExpr("users.student_guardians").
		Where("guardian_id = ?", guardianID).
		Exec(ctx)

	// Delete guardian invitations
	_, _ = db.NewDelete().
		TableExpr("users.guardian_invitations").
		Where("guardian_id = ?", guardianID).
		Exec(ctx)

	// Delete guardian profile
	_, _ = db.NewDelete().
		TableExpr("users.guardian_profiles").
		Where("id = ?", guardianID).
		Exec(ctx)
}

// =============================================================================
// LIST GUARDIANS TESTS
// =============================================================================

func TestListGuardians_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians", ctx.resource.ListGuardiansHandler())

	req := testutil.NewAuthenticatedRequest("GET", "/guardians", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

func TestListGuardians_WithSearchFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians", ctx.resource.ListGuardiansHandler())

	req := testutil.NewAuthenticatedRequest("GET", "/guardians?search=test", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET GUARDIAN TESTS
// =============================================================================

func TestGetGuardian_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/{id}", ctx.resource.GetGuardianHandler())

	req := testutil.NewAuthenticatedRequest("GET", "/guardians/99999", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetGuardian_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/{id}", ctx.resource.GetGuardianHandler())

	req := testutil.NewAuthenticatedRequest("GET", "/guardians/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// CREATE GUARDIAN TESTS
// =============================================================================

func TestCreateGuardian_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians", ctx.resource.CreateGuardianHandler())

	body := map[string]interface{}{
		"first_name":               fmt.Sprintf("TestGuardian-%d", time.Now().UnixNano()),
		"last_name":                "Test",
		"email":                    fmt.Sprintf("guardian-%d@test.com", time.Now().UnixNano()),
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	// Use admin claims with admin:* permission - guardian creation requires admin or group supervisor
	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest("POST", "/guardians", body,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.NotZero(t, data["id"])

	// Cleanup created guardian
	if id, ok := data["id"].(float64); ok {
		cleanupGuardian(t, ctx.db, int64(id))
	}
}

func TestCreateGuardian_Forbidden_NonStaffUser(t *testing.T) {
	// Non-staff users cannot create guardian profiles
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians", ctx.resource.CreateGuardianHandler())

	body := map[string]interface{}{
		"first_name":               "Test",
		"last_name":                "Guardian",
		"email":                    "test@test.com",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	req := testutil.NewAuthenticatedRequest("POST", "/guardians", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestCreateGuardian_BadRequest_MissingFirstName(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians", ctx.resource.CreateGuardianHandler())

	body := map[string]interface{}{
		"last_name":                "Test",
		"email":                    "test@test.com",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	// Use admin claims with admin:* permission - guardian creation requires admin or group supervisor
	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest("POST", "/guardians", body,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateGuardian_BadRequest_MissingLastName(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians", ctx.resource.CreateGuardianHandler())

	body := map[string]interface{}{
		"first_name":               "Test",
		"email":                    "test@test.com",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	// Use admin claims with admin:* permission - guardian creation requires admin or group supervisor
	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest("POST", "/guardians", body,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateGuardian_BadRequest_NoContactMethod(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians", ctx.resource.CreateGuardianHandler())

	body := map[string]interface{}{
		"first_name":               "Test",
		"last_name":                "Guardian",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	// Use admin claims with admin:* permission - guardian creation requires admin or group supervisor
	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest("POST", "/guardians", body,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// UPDATE GUARDIAN TESTS
// =============================================================================

func TestUpdateGuardian_Forbidden_NonStaff(t *testing.T) {
	// Non-staff users cannot update guardian profiles
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/guardians/{id}", ctx.resource.UpdateGuardianHandler())

	body := map[string]interface{}{
		"first_name": "Updated",
	}

	req := testutil.NewAuthenticatedRequest("PUT", "/guardians/99999", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestUpdateGuardian_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/guardians/{id}", ctx.resource.UpdateGuardianHandler())

	body := map[string]interface{}{
		"first_name": "Updated",
	}

	// Use admin claims with admin:* permission - guardian update requires admin or group supervisor
	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest("PUT", "/guardians/99999", body,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdateGuardian_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/guardians/{id}", ctx.resource.UpdateGuardianHandler())

	body := map[string]interface{}{
		"first_name": "Updated",
	}

	req := testutil.NewAuthenticatedRequest("PUT", "/guardians/invalid", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DELETE GUARDIAN TESTS
// =============================================================================

func TestDeleteGuardian_Forbidden_NonStaff(t *testing.T) {
	// Non-staff users cannot delete guardian profiles
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/guardians/{id}", ctx.resource.DeleteGuardianHandler())

	req := testutil.NewAuthenticatedRequest("DELETE", "/guardians/99999", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestDeleteGuardian_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/guardians/{id}", ctx.resource.DeleteGuardianHandler())

	// Use admin claims with admin:* permission - guardian delete requires admin or group supervisor
	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest("DELETE", "/guardians/99999", nil,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestDeleteGuardian_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/guardians/{id}", ctx.resource.DeleteGuardianHandler())

	req := testutil.NewAuthenticatedRequest("DELETE", "/guardians/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// SPECIAL LIST ENDPOINTS TESTS
// =============================================================================

func TestListGuardiansWithoutAccount_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/without-account", ctx.resource.ListGuardiansWithoutAccountHandler())

	req := testutil.NewAuthenticatedRequest("GET", "/guardians/without-account", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

func TestListInvitableGuardians_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/invitable", ctx.resource.ListInvitableGuardiansHandler())

	req := testutil.NewAuthenticatedRequest("GET", "/guardians/invitable", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

func TestListPendingInvitations_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/invitations/pending", ctx.resource.ListPendingInvitationsHandler())

	req := testutil.NewAuthenticatedRequest("GET", "/guardians/invitations/pending", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

// =============================================================================
// STUDENT-GUARDIAN RELATIONSHIP TESTS
// =============================================================================

func TestGetStudentGuardians_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	student := testpkg.CreateTestStudent(t, ctx.db, "Guardian", "TestStudent", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Get("/guardians/students/{studentId}/guardians", ctx.resource.GetStudentGuardiansHandler())

	req := testutil.NewAuthenticatedRequest("GET", fmt.Sprintf("/guardians/students/%d/guardians", student.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

func TestGetStudentGuardians_InvalidStudentID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/students/{studentId}/guardians", ctx.resource.GetStudentGuardiansHandler())

	req := testutil.NewAuthenticatedRequest("GET", "/guardians/students/invalid/guardians", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGetGuardianStudents_NonExistent_ReturnsEmptyArray(t *testing.T) {
	// API returns 200 with empty array for non-existent guardian (valid design choice)
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/{id}/students", ctx.resource.GetGuardianStudentsHandler())

	req := testutil.NewAuthenticatedRequest("GET", "/guardians/99999/students", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
	assert.Empty(t, data, "Expected empty array for non-existent guardian")
}

func TestGetGuardianStudents_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/{id}/students", ctx.resource.GetGuardianStudentsHandler())

	req := testutil.NewAuthenticatedRequest("GET", "/guardians/invalid/students", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// INVITATION VALIDATION TESTS (PUBLIC ENDPOINTS)
// =============================================================================

func TestValidateGuardianInvitation_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/invitations/{token}", ctx.resource.ValidateGuardianInvitationHandler())

	req := testutil.NewJSONRequest("GET", "/guardians/invitations/invalid-token-12345", nil)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestAcceptGuardianInvitation_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/invitations/{token}/accept", ctx.resource.AcceptGuardianInvitationHandler())

	body := map[string]interface{}{
		"password":         "Test1234%",
		"confirm_password": "Test1234%",
	}

	req := testutil.NewJSONRequest("POST", "/guardians/invitations/invalid-token-12345/accept", body)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestAcceptGuardianInvitation_BadRequest_MissingPassword(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/invitations/{token}/accept", ctx.resource.AcceptGuardianInvitationHandler())

	body := map[string]interface{}{
		"confirm_password": "Test1234%",
	}

	req := testutil.NewJSONRequest("POST", "/guardians/invitations/some-token/accept", body)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAcceptGuardianInvitation_BadRequest_PasswordMismatch(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/invitations/{token}/accept", ctx.resource.AcceptGuardianInvitationHandler())

	body := map[string]interface{}{
		"password":         "Test1234%",
		"confirm_password": "DifferentPassword%",
	}

	req := testutil.NewJSONRequest("POST", "/guardians/invitations/some-token/accept", body)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}
