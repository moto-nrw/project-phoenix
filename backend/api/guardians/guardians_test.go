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

	// Delete student-guardian relationships (column is guardian_profile_id, not guardian_id)
	_, _ = db.NewDelete().
		TableExpr("users.students_guardians").
		Where("guardian_profile_id = ?", guardianID).
		Exec(ctx)

	// Delete guardian invitations (column is guardian_profile_id, not guardian_id)
	_, _ = db.NewDelete().
		TableExpr("auth.guardian_invitations").
		Where("guardian_profile_id = ?", guardianID).
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

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]any)
	require.True(t, ok, "Expected data to be an array")
}

func TestListGuardians_WithSearchFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians", ctx.resource.ListGuardiansHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians?search=test", nil,
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

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/99999", nil,
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

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/invalid", nil,
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

	body := map[string]any{
		"first_name":               fmt.Sprintf("TestGuardian-%d", time.Now().UnixNano()),
		"last_name":                "Test",
		"email":                    fmt.Sprintf("guardian-%d@test.com", time.Now().UnixNano()),
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	// Use admin claims with admin:* permission - guardian creation requires admin or group supervisor
	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians", body,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
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

	body := map[string]any{
		"first_name":               "Test",
		"last_name":                "Guardian",
		"email":                    "test@test.com",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestCreateGuardian_Success_MissingFirstName(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians", ctx.resource.CreateGuardianHandler())

	body := map[string]any{
		"last_name":                "Test",
		"email":                    "test-no-firstname@test.com",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	// Guardian names are optional (e.g., CSV imports may only have relationship type)
	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians", body,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestCreateGuardian_Success_MissingLastName(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians", ctx.resource.CreateGuardianHandler())

	body := map[string]any{
		"first_name":               "Test",
		"email":                    "test-no-lastname@test.com",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	// Guardian names are optional (e.g., CSV imports may only have relationship type)
	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians", body,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestCreateGuardian_Success_WithoutContactMethod(t *testing.T) {
	// With the flexible phone numbers system, guardians can be created without
	// immediate contact methods - phone numbers are added in a separate step
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians", ctx.resource.CreateGuardianHandler())

	body := map[string]any{
		"first_name":               "Test",
		"last_name":                "Guardian",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	// Use admin claims with admin:* permission - guardian creation requires admin or group supervisor
	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians", body,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should succeed - phone numbers can be added separately
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
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

	body := map[string]any{
		"first_name": "Updated",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/guardians/99999", body,
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

	body := map[string]any{
		"first_name": "Updated",
	}

	// Use admin claims with admin:* permission - guardian update requires admin or group supervisor
	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/guardians/99999", body,
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

	body := map[string]any{
		"first_name": "Updated",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/guardians/invalid", body,
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

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/guardians/99999", nil,
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
	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/guardians/99999", nil,
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

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/guardians/invalid", nil,
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

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/without-account", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]any)
	require.True(t, ok, "Expected data to be an array")
}

func TestListInvitableGuardians_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/invitable", ctx.resource.ListInvitableGuardiansHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/invitable", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]any)
	require.True(t, ok, "Expected data to be an array")
}

func TestListPendingInvitations_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/invitations/pending", ctx.resource.ListPendingInvitationsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/invitations/pending", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]any)
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

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/guardians/students/%d/guardians", student.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]any)
	require.True(t, ok, "Expected data to be an array")
}

func TestGetStudentGuardians_InvalidStudentID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/students/{studentId}/guardians", ctx.resource.GetStudentGuardiansHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/students/invalid/guardians", nil,
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

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/99999/students", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]any)
	require.True(t, ok, "Expected data to be an array")
	assert.Empty(t, data, "Expected empty array for non-existent guardian")
}

func TestGetGuardianStudents_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/{id}/students", ctx.resource.GetGuardianStudentsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/invalid/students", nil,
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

	req := testutil.NewJSONRequest(t, "GET", "/guardians/invitations/invalid-token-12345", nil)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestAcceptGuardianInvitation_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/invitations/{token}/accept", ctx.resource.AcceptGuardianInvitationHandler())

	body := map[string]any{
		"password":         "Test1234%",
		"confirm_password": "Test1234%",
	}

	req := testutil.NewJSONRequest(t, "POST", "/guardians/invitations/invalid-token-12345/accept", body)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestAcceptGuardianInvitation_BadRequest_MissingPassword(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/invitations/{token}/accept", ctx.resource.AcceptGuardianInvitationHandler())

	body := map[string]any{
		"confirm_password": "Test1234%",
	}

	req := testutil.NewJSONRequest(t, "POST", "/guardians/invitations/some-token/accept", body)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAcceptGuardianInvitation_BadRequest_PasswordMismatch(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/invitations/{token}/accept", ctx.resource.AcceptGuardianInvitationHandler())

	body := map[string]any{
		"password":         "Test1234%",
		"confirm_password": "DifferentPassword%",
	}

	req := testutil.NewJSONRequest(t, "POST", "/guardians/invitations/some-token/accept", body)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
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
// LINK GUARDIAN TO STUDENT TESTS
// =============================================================================

func TestLinkGuardianToStudent_Forbidden_NonStaff(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/students/{studentId}/guardians", ctx.resource.LinkGuardianToStudentHandler())

	body := map[string]any{
		"guardian_profile_id":  1,
		"relationship_type":    "parent",
		"is_primary":           true,
		"is_emergency_contact": true,
		"can_pickup":           true,
		"emergency_priority":   1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/students/1/guardians", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestLinkGuardianToStudent_InvalidStudentID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/students/{studentId}/guardians", ctx.resource.LinkGuardianToStudentHandler())

	body := map[string]any{
		"guardian_profile_id": 1,
		"relationship_type":   "parent",
		"emergency_priority":  1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/students/invalid/guardians", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestLinkGuardianToStudent_BadRequest_MissingGuardianID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a student with a group for the test
	student := testpkg.CreateTestStudent(t, ctx.db, "Link", "TestStudent", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/students/{studentId}/guardians", ctx.resource.LinkGuardianToStudentHandler())

	body := map[string]any{
		"relationship_type":  "parent",
		"emergency_priority": 1,
	}

	// Use admin permissions
	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/students/%d/guardians", student.ID), body,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestLinkGuardianToStudent_BadRequest_MissingRelationshipType(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	student := testpkg.CreateTestStudent(t, ctx.db, "Link2", "TestStudent", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/students/{studentId}/guardians", ctx.resource.LinkGuardianToStudentHandler())

	body := map[string]any{
		"guardian_profile_id": 1,
		"emergency_priority":  1,
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/students/%d/guardians", student.ID), body,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestLinkGuardianToStudent_BadRequest_InvalidEmergencyPriority(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	student := testpkg.CreateTestStudent(t, ctx.db, "Link3", "TestStudent", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/students/{studentId}/guardians", ctx.resource.LinkGuardianToStudentHandler())

	body := map[string]any{
		"guardian_profile_id": 1,
		"relationship_type":   "parent",
		"emergency_priority":  0, // Invalid - must be at least 1
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/students/%d/guardians", student.ID), body,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// UPDATE STUDENT-GUARDIAN RELATIONSHIP TESTS
// =============================================================================

func TestUpdateStudentGuardianRelationship_InvalidRelationshipID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/guardians/relationships/{relationshipId}", ctx.resource.UpdateStudentGuardianRelationshipHandler())

	body := map[string]any{
		"is_primary": true,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/guardians/relationships/invalid", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdateStudentGuardianRelationship_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/guardians/relationships/{relationshipId}", ctx.resource.UpdateStudentGuardianRelationshipHandler())

	body := map[string]any{
		"is_primary": true,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/guardians/relationships/99999", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// REMOVE GUARDIAN FROM STUDENT TESTS
// =============================================================================

func TestRemoveGuardianFromStudent_Forbidden_NonStaff(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/students/{studentId}/guardians/{guardianId}", ctx.resource.RemoveGuardianFromStudentHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/students/1/guardians/1", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestRemoveGuardianFromStudent_InvalidStudentID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/students/{studentId}/guardians/{guardianId}", ctx.resource.RemoveGuardianFromStudentHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/students/invalid/guardians/1", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestRemoveGuardianFromStudent_InvalidGuardianID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/students/{studentId}/guardians/{guardianId}", ctx.resource.RemoveGuardianFromStudentHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/students/1/guardians/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// SEND INVITATION TESTS
// =============================================================================

func TestSendInvitation_InvalidGuardianID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/{id}/invite", ctx.resource.SendInvitationHandler())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians/invalid/invite", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestSendInvitation_Unauthorized_NoClaims(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/{id}/invite", ctx.resource.SendInvitationHandler())

	// Request without proper claims (ID=0)
	req := testutil.NewJSONRequest(t, "POST", "/guardians/1/invite", nil)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertUnauthorized(t, rr)
}

// =============================================================================
// VALIDATE INVITATION TESTS
// =============================================================================

func TestValidateGuardianInvitation_EmptyToken(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/invitations/{token}", ctx.resource.ValidateGuardianInvitationHandler())

	// Empty token should return bad request
	req := testutil.NewJSONRequest(t, "GET", "/guardians/invitations/", nil)

	rr := testutil.ExecuteRequest(router, req)

	// Chi router treats empty param as 404
	assert.Contains(t, []int{http.StatusNotFound, http.StatusBadRequest}, rr.Code)
}

// =============================================================================
// ACCEPT INVITATION TESTS
// =============================================================================

func TestAcceptGuardianInvitation_EmptyToken(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/invitations/{token}/accept", ctx.resource.AcceptGuardianInvitationHandler())

	body := map[string]any{
		"password":         "Test1234%",
		"confirm_password": "Test1234%",
	}

	req := testutil.NewJSONRequest(t, "POST", "/guardians/invitations//accept", body)

	rr := testutil.ExecuteRequest(router, req)

	// Chi router treats empty param as 404
	assert.Contains(t, []int{http.StatusNotFound, http.StatusBadRequest}, rr.Code)
}

func TestAcceptGuardianInvitation_BadRequest_MissingConfirmPassword(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/invitations/{token}/accept", ctx.resource.AcceptGuardianInvitationHandler())

	body := map[string]any{
		"password": "Test1234%",
	}

	req := testutil.NewJSONRequest(t, "POST", "/guardians/invitations/some-token/accept", body)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// PHONE NUMBER HANDLER TESTS
// =============================================================================

// createTestGuardianWithPhones creates a guardian with phone numbers for testing
func createTestGuardianWithPhones(t *testing.T, ctx *testContext) (int64, int64, int64) {
	t.Helper()

	// Create guardian profile
	guardianReq := map[string]any{
		"first_name":               fmt.Sprintf("TestGuardian-%d", time.Now().UnixNano()),
		"last_name":                "PhoneTest",
		"email":                    fmt.Sprintf("phone-%d@test.com", time.Now().UnixNano()),
		"preferred_contact_method": "phone",
		"language_preference":      "de",
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians", guardianReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	router := chi.NewRouter()
	router.Post("/guardians", ctx.resource.CreateGuardianHandler())
	rr := testutil.ExecuteRequest(router, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardianID := int64(data["id"].(float64))

	// Add two phone numbers
	phoneRouter := chi.NewRouter()
	phoneRouter.Post("/guardians/{id}/phone-numbers", ctx.resource.AddPhoneNumberHandler())

	// Primary phone
	phoneReq1 := map[string]any{
		"phone_number": "+49 123 456789",
		"phone_type":   "mobile",
		"is_primary":   true,
	}
	req1 := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/guardians/%d/phone-numbers", guardianID), phoneReq1,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)
	rr1 := testutil.ExecuteRequest(phoneRouter, req1)
	require.Equal(t, http.StatusCreated, rr1.Code)
	response1 := testutil.ParseJSONResponse(t, rr1.Body.Bytes())
	data1 := response1["data"].(map[string]any)
	phone1ID := int64(data1["id"].(float64))

	// Secondary phone
	phoneReq2 := map[string]any{
		"phone_number": "+49 987 654321",
		"phone_type":   "work",
		"is_primary":   false,
	}
	req2 := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/guardians/%d/phone-numbers", guardianID), phoneReq2,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)
	rr2 := testutil.ExecuteRequest(phoneRouter, req2)
	require.Equal(t, http.StatusCreated, rr2.Code)
	response2 := testutil.ParseJSONResponse(t, rr2.Body.Bytes())
	data2 := response2["data"].(map[string]any)
	phone2ID := int64(data2["id"].(float64))

	return guardianID, phone1ID, phone2ID
}

// =============================================================================
// LIST PHONE NUMBERS TESTS
// =============================================================================

func TestListGuardianPhoneNumbers_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Get("/guardians/{id}/phone-numbers", ctx.resource.ListGuardianPhoneNumbersHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/guardians/%d/phone-numbers", guardianID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	phones := response["data"].([]any)
	assert.Len(t, phones, 2, "Expected 2 phone numbers")
}

func TestListGuardianPhoneNumbers_InvalidGuardianID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/guardians/{id}/phone-numbers", ctx.resource.ListGuardianPhoneNumbersHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/invalid/phone-numbers", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestListGuardianPhoneNumbers_EmptyList(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create guardian without phones
	guardianReq := map[string]any{
		"first_name":               fmt.Sprintf("NoPhone-%d", time.Now().UnixNano()),
		"last_name":                "Test",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians", guardianReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	createRouter := chi.NewRouter()
	createRouter.Post("/guardians", ctx.resource.CreateGuardianHandler())
	rr := testutil.ExecuteRequest(createRouter, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardianID := int64(data["id"].(float64))
	defer cleanupGuardian(t, ctx.db, guardianID)

	// List phones
	router := chi.NewRouter()
	router.Get("/guardians/{id}/phone-numbers", ctx.resource.ListGuardianPhoneNumbersHandler())

	listReq := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/guardians/%d/phone-numbers", guardianID), nil,
		testutil.WithClaims(claims),
		testutil.WithPermissions("users:read"),
	)

	listRr := testutil.ExecuteRequest(router, listReq)

	testutil.AssertSuccessResponse(t, listRr, http.StatusOK)

	listResponse := testutil.ParseJSONResponse(t, listRr.Body.Bytes())
	phones := listResponse["data"].([]any)
	assert.Empty(t, phones, "Expected empty phone list")
}

// =============================================================================
// ADD PHONE NUMBER TESTS
// =============================================================================

func TestAddPhoneNumber_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create guardian without phones first
	guardianReq := map[string]any{
		"first_name":               fmt.Sprintf("AddPhone-%d", time.Now().UnixNano()),
		"last_name":                "Test",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians", guardianReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	createRouter := chi.NewRouter()
	createRouter.Post("/guardians", ctx.resource.CreateGuardianHandler())
	rr := testutil.ExecuteRequest(createRouter, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardianID := int64(data["id"].(float64))
	defer cleanupGuardian(t, ctx.db, guardianID)

	// Add phone number
	router := chi.NewRouter()
	router.Post("/guardians/{id}/phone-numbers", ctx.resource.AddPhoneNumberHandler())

	phoneReq := map[string]any{
		"phone_number": "+49 123 456789",
		"phone_type":   "mobile",
		"label":        "Personal",
		"is_primary":   true,
	}

	phoneAddReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/guardians/%d/phone-numbers", guardianID), phoneReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	phoneRr := testutil.ExecuteRequest(router, phoneAddReq)

	testutil.AssertSuccessResponse(t, phoneRr, http.StatusCreated)

	phoneResponse := testutil.ParseJSONResponse(t, phoneRr.Body.Bytes())
	phoneData := phoneResponse["data"].(map[string]any)
	assert.Equal(t, "+49 123 456789", phoneData["phone_number"])
	assert.Equal(t, "mobile", phoneData["phone_type"])
	assert.Equal(t, "Personal", phoneData["label"])
	assert.True(t, phoneData["is_primary"].(bool))
}

func TestAddPhoneNumber_Forbidden_NonStaff(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/{id}/phone-numbers", ctx.resource.AddPhoneNumberHandler())

	phoneReq := map[string]any{
		"phone_number": "+49 123 456789",
		"phone_type":   "mobile",
		"is_primary":   true,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians/1/phone-numbers", phoneReq,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestAddPhoneNumber_InvalidGuardianID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/{id}/phone-numbers", ctx.resource.AddPhoneNumberHandler())

	phoneReq := map[string]any{
		"phone_number": "+49 123 456789",
		"phone_type":   "mobile",
		"is_primary":   true,
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians/invalid/phone-numbers", phoneReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAddPhoneNumber_BadRequest_MissingPhoneNumber(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Post("/guardians/{id}/phone-numbers", ctx.resource.AddPhoneNumberHandler())

	phoneReq := map[string]any{
		"phone_type": "mobile",
		"is_primary": true,
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/guardians/%d/phone-numbers", guardianID), phoneReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAddPhoneNumber_BadRequest_InvalidPhoneType(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Post("/guardians/{id}/phone-numbers", ctx.resource.AddPhoneNumberHandler())

	phoneReq := map[string]any{
		"phone_number": "+49 123 456789",
		"phone_type":   "invalid_type",
		"is_primary":   true,
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/guardians/%d/phone-numbers", guardianID), phoneReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAddPhoneNumber_DefaultPhoneType(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create guardian without phones first
	guardianReq := map[string]any{
		"first_name":               fmt.Sprintf("DefaultType-%d", time.Now().UnixNano()),
		"last_name":                "Test",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians", guardianReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	createRouter := chi.NewRouter()
	createRouter.Post("/guardians", ctx.resource.CreateGuardianHandler())
	rr := testutil.ExecuteRequest(createRouter, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardianID := int64(data["id"].(float64))
	defer cleanupGuardian(t, ctx.db, guardianID)

	// Add phone without specifying type
	router := chi.NewRouter()
	router.Post("/guardians/{id}/phone-numbers", ctx.resource.AddPhoneNumberHandler())

	phoneReq := map[string]any{
		"phone_number": "+49 123 456789",
		"is_primary":   true,
	}

	phoneAddReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/guardians/%d/phone-numbers", guardianID), phoneReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	phoneRr := testutil.ExecuteRequest(router, phoneAddReq)

	testutil.AssertSuccessResponse(t, phoneRr, http.StatusCreated)

	phoneResponse := testutil.ParseJSONResponse(t, phoneRr.Body.Bytes())
	phoneData := phoneResponse["data"].(map[string]any)
	assert.Equal(t, "mobile", phoneData["phone_type"], "Expected default phone_type to be 'mobile'")
}

// =============================================================================
// UPDATE PHONE NUMBER TESTS
// =============================================================================

func TestUpdatePhoneNumber_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, phone1ID, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Put("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.UpdatePhoneNumberHandler())

	updateReq := map[string]any{
		"phone_number": "+49 111 222333",
		"phone_type":   "home",
		"label":        "Updated Label",
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/guardians/%d/phone-numbers/%d", guardianID, phone1ID), updateReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	phoneData := response["data"].(map[string]any)
	assert.Equal(t, "+49 111 222333", phoneData["phone_number"])
	assert.Equal(t, "home", phoneData["phone_type"])
	assert.Equal(t, "Updated Label", phoneData["label"])
}

func TestUpdatePhoneNumber_InvalidGuardianID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.UpdatePhoneNumberHandler())

	updateReq := map[string]any{
		"phone_number": "+49 111 222333",
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/guardians/invalid/phone-numbers/1", updateReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePhoneNumber_InvalidPhoneID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Put("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.UpdatePhoneNumberHandler())

	updateReq := map[string]any{
		"phone_number": "+49 111 222333",
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/guardians/%d/phone-numbers/invalid", guardianID), updateReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePhoneNumber_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Put("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.UpdatePhoneNumberHandler())

	updateReq := map[string]any{
		"phone_number": "+49 111 222333",
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/guardians/%d/phone-numbers/99999", guardianID), updateReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdatePhoneNumber_Forbidden_NonStaff(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.UpdatePhoneNumberHandler())

	updateReq := map[string]any{
		"phone_number": "+49 111 222333",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/guardians/1/phone-numbers/1", updateReq,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestUpdatePhoneNumber_BadRequest_EmptyPhoneNumber(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, phone1ID, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Put("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.UpdatePhoneNumberHandler())

	emptyPhone := ""
	updateReq := map[string]any{
		"phone_number": &emptyPhone,
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/guardians/%d/phone-numbers/%d", guardianID, phone1ID), updateReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePhoneNumber_BadRequest_InvalidPhoneType(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, phone1ID, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Put("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.UpdatePhoneNumberHandler())

	invalidType := "invalid_type"
	updateReq := map[string]any{
		"phone_type": &invalidType,
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/guardians/%d/phone-numbers/%d", guardianID, phone1ID), updateReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePhoneNumber_Forbidden_PhoneNotBelongToGuardian(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create first guardian with phones
	guardian1ID, phone1ID, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardian1ID)

	// Create second guardian
	guardianReq := map[string]any{
		"first_name":               fmt.Sprintf("Guardian2-%d", time.Now().UnixNano()),
		"last_name":                "Test",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians", guardianReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	createRouter := chi.NewRouter()
	createRouter.Post("/guardians", ctx.resource.CreateGuardianHandler())
	rr := testutil.ExecuteRequest(createRouter, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardian2ID := int64(data["id"].(float64))
	defer cleanupGuardian(t, ctx.db, guardian2ID)

	// Try to update guardian1's phone via guardian2's endpoint
	router := chi.NewRouter()
	router.Put("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.UpdatePhoneNumberHandler())

	updateReq := map[string]any{
		"phone_number": "+49 111 222333",
	}

	updatePhoneReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/guardians/%d/phone-numbers/%d", guardian2ID, phone1ID), updateReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	updateRr := testutil.ExecuteRequest(router, updatePhoneReq)

	testutil.AssertForbidden(t, updateRr)
}

// =============================================================================
// DELETE PHONE NUMBER TESTS
// =============================================================================

func TestDeletePhoneNumber_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, _, phone2ID := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Delete("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.DeletePhoneNumberHandler())

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/guardians/%d/phone-numbers/%d", guardianID, phone2ID), nil,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeletePhoneNumber_InvalidGuardianID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.DeletePhoneNumberHandler())

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/guardians/invalid/phone-numbers/1", nil,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestDeletePhoneNumber_InvalidPhoneID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Delete("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.DeletePhoneNumberHandler())

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/guardians/%d/phone-numbers/invalid", guardianID), nil,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestDeletePhoneNumber_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Delete("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.DeletePhoneNumberHandler())

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/guardians/%d/phone-numbers/99999", guardianID), nil,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestDeletePhoneNumber_Forbidden_NonStaff(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.DeletePhoneNumberHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/guardians/1/phone-numbers/1", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestDeletePhoneNumber_Forbidden_PhoneNotBelongToGuardian(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create first guardian with phones
	guardian1ID, phone1ID, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardian1ID)

	// Create second guardian
	guardianReq := map[string]any{
		"first_name":               fmt.Sprintf("Guardian2Del-%d", time.Now().UnixNano()),
		"last_name":                "Test",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians", guardianReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	createRouter := chi.NewRouter()
	createRouter.Post("/guardians", ctx.resource.CreateGuardianHandler())
	rr := testutil.ExecuteRequest(createRouter, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardian2ID := int64(data["id"].(float64))
	defer cleanupGuardian(t, ctx.db, guardian2ID)

	// Try to delete guardian1's phone via guardian2's endpoint
	router := chi.NewRouter()
	router.Delete("/guardians/{id}/phone-numbers/{phoneId}", ctx.resource.DeletePhoneNumberHandler())

	deleteReq := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/guardians/%d/phone-numbers/%d", guardian2ID, phone1ID), nil,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	deleteRr := testutil.ExecuteRequest(router, deleteReq)

	testutil.AssertForbidden(t, deleteRr)
}

// =============================================================================
// SET PRIMARY PHONE TESTS
// =============================================================================

func TestSetPrimaryPhone_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, _, phone2ID := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Post("/guardians/{id}/phone-numbers/{phoneId}/set-primary", ctx.resource.SetPrimaryPhoneHandler())

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/guardians/%d/phone-numbers/%d/set-primary", guardianID, phone2ID), nil,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	phoneData := response["data"].(map[string]any)
	assert.True(t, phoneData["is_primary"].(bool), "Expected phone to be set as primary")
}

func TestSetPrimaryPhone_InvalidGuardianID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/{id}/phone-numbers/{phoneId}/set-primary", ctx.resource.SetPrimaryPhoneHandler())

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians/invalid/phone-numbers/1/set-primary", nil,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestSetPrimaryPhone_InvalidPhoneID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Post("/guardians/{id}/phone-numbers/{phoneId}/set-primary", ctx.resource.SetPrimaryPhoneHandler())

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/guardians/%d/phone-numbers/invalid/set-primary", guardianID), nil,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestSetPrimaryPhone_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := chi.NewRouter()
	router.Post("/guardians/{id}/phone-numbers/{phoneId}/set-primary", ctx.resource.SetPrimaryPhoneHandler())

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/guardians/%d/phone-numbers/99999/set-primary", guardianID), nil,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestSetPrimaryPhone_Forbidden_NonStaff(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/guardians/{id}/phone-numbers/{phoneId}/set-primary", ctx.resource.SetPrimaryPhoneHandler())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians/1/phone-numbers/1/set-primary", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestSetPrimaryPhone_Forbidden_PhoneNotBelongToGuardian(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create first guardian with phones
	guardian1ID, phone1ID, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardian1ID)

	// Create second guardian
	guardianReq := map[string]any{
		"first_name":               fmt.Sprintf("Guardian2Pri-%d", time.Now().UnixNano()),
		"last_name":                "Test",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/guardians", guardianReq,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	createRouter := chi.NewRouter()
	createRouter.Post("/guardians", ctx.resource.CreateGuardianHandler())
	rr := testutil.ExecuteRequest(createRouter, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardian2ID := int64(data["id"].(float64))
	defer cleanupGuardian(t, ctx.db, guardian2ID)

	// Try to set guardian1's phone as primary via guardian2's endpoint
	router := chi.NewRouter()
	router.Post("/guardians/{id}/phone-numbers/{phoneId}/set-primary", ctx.resource.SetPrimaryPhoneHandler())

	setPrimaryReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/guardians/%d/phone-numbers/%d/set-primary", guardian2ID, phone1ID), nil,
		testutil.WithClaims(claims),
		testutil.WithPermissions("admin:*"),
	)

	setPrimaryRr := testutil.ExecuteRequest(router, setPrimaryReq)

	testutil.AssertForbidden(t, setPrimaryRr)
}
