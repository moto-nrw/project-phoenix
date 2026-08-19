// Package guardians_test tests the guardians API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
//
// Every test drives the resource through its real Router() with a signed JWT so
// the production middleware chain (Verifier → Authenticator → TenantMiddleware →
// RequiresPermission → TenantTxMiddleware) runs, rather than mounting a private
// handler behind a test-export wrapper (backend conventions Rule 5).
package guardians_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	guardiansAPI "github.com/moto-nrw/project-phoenix/api/guardians"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func init() {
	// Ensure a JWT secret exists so MintTestJWT and Router()'s MustNewTokenAuth
	// agree (no-op when AUTH_JWT_SECRET is already in the environment).
	testutil.SeedTestJWTConfig()
}

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

	resource := guardiansAPI.NewResource(
		svc.Guardian,
		svc.GuardianInvitation,
		svc.Users,
		svc.Education,
		svc.UserContext,
		db,
	)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// bearer mints a signed JWT for claims and returns it as an Authorization
// bearer option, driving the resource through its real Router() middleware
// chain instead of a deleted test-export handler wrapper.
func bearer(t *testing.T, claims jwt.AppClaims) testutil.RequestOption {
	t.Helper()
	return testutil.WithJWTBearer(testutil.MintTestJWT(t, claims))
}

// nonStaffClaims builds an authenticated but non-admin principal for account 1
// (which has no staff record in the test DB). It carries exactly perms so the
// request clears the route's RequiresPermission gate and is then denied by the
// handler's own staff/supervisor check — the 403 the old bare-handler
// "Forbidden_NonStaff" tests asserted.
func nonStaffClaims(perms ...string) jwt.AppClaims {
	return jwt.AppClaims{
		ID:          1,
		Sub:         "nonstaff@example.com",
		TenantID:    1,
		Roles:       []string{"user"},
		Permissions: perms,
	}
}

// withPerms returns a copy of claims narrowed to exactly perms, mirroring an old
// WithPermissions(...) override that replaced the claims' permission set for a
// single request.
func withPerms(claims jwt.AppClaims, perms ...string) jwt.AppClaims {
	claims.Permissions = perms
	return claims
}

// cleanupGuardian cleans up a guardian profile and related records
func cleanupGuardian(t *testing.T, db *bun.DB, guardianID int64) {
	t.Helper()
	ctx := testpkg.TenantContext(1)

	// Delete phone numbers
	_, _ = db.NewDelete().
		TableExpr("users.guardian_phone_numbers").
		Where("guardian_profile_id = ?", guardianID).
		Exec(ctx)

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

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/", nil,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]any)
	require.True(t, ok, "Expected data to be an array")
}

func TestListGuardians_WithSearchFilter(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/?search=test", nil,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET GUARDIAN TESTS
// =============================================================================

func TestGetGuardian_NotFound(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/99999", nil,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetGuardian_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/invalid", nil,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// CREATE GUARDIAN TESTS
// =============================================================================

func TestCreateGuardian_Success(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	body := map[string]any{
		"first_name":               fmt.Sprintf("TestGuardian-%d", time.Now().UnixNano()),
		"last_name":                "Test",
		"email":                    fmt.Sprintf("guardian-%d@test.com", time.Now().UnixNano()),
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	// Use admin claims with admin:* permission - guardian creation requires admin or group supervisor
	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body,
		bearer(t, testutil.AdminTestClaims(999)),
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

	router := ctx.resource.Router()

	body := map[string]any{
		"first_name":               "Test",
		"last_name":                "Guardian",
		"email":                    "test@test.com",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body,
		bearer(t, nonStaffClaims("users:create")),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestCreateGuardian_Success_MissingFirstName(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	body := map[string]any{
		"last_name":                "Test",
		"email":                    fmt.Sprintf("no-firstname-%d@test.com", time.Now().UnixNano()),
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	// Guardian names are optional (e.g., CSV imports may only have relationship type)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Cleanup created guardian
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if data, ok := response["data"].(map[string]any); ok {
		if id, ok := data["id"].(float64); ok {
			cleanupGuardian(t, ctx.db, int64(id))
		}
	}
}

func TestCreateGuardian_Success_MissingLastName(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	body := map[string]any{
		"first_name":               "Test",
		"email":                    fmt.Sprintf("no-lastname-%d@test.com", time.Now().UnixNano()),
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	// Guardian names are optional (e.g., CSV imports may only have relationship type)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Cleanup created guardian
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if data, ok := response["data"].(map[string]any); ok {
		if id, ok := data["id"].(float64); ok {
			cleanupGuardian(t, ctx.db, int64(id))
		}
	}
}

func TestCreateGuardian_Success_WithoutContactMethod(t *testing.T) {
	// With the flexible phone numbers system, guardians can be created without
	// immediate contact methods - phone numbers are added in a separate step
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	body := map[string]any{
		"first_name":               fmt.Sprintf("NoContact-%d", time.Now().UnixNano()),
		"last_name":                "Guardian",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	// Use admin claims with admin:* permission - guardian creation requires admin or group supervisor
	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should succeed - phone numbers can be added separately
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Cleanup created guardian
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if data, ok := response["data"].(map[string]any); ok {
		if id, ok := data["id"].(float64); ok {
			cleanupGuardian(t, ctx.db, int64(id))
		}
	}
}

// =============================================================================
// UPDATE GUARDIAN TESTS
// =============================================================================

func TestUpdateGuardian_Forbidden_NonStaff(t *testing.T) {
	// Non-staff users cannot update guardian profiles
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	body := map[string]any{
		"first_name": "Updated",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/99999", body,
		bearer(t, nonStaffClaims("users:update")),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestUpdateGuardian_NotFound(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	body := map[string]any{
		"first_name": "Updated",
	}

	// Use admin claims with admin:* permission - guardian update requires admin or group supervisor
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/99999", body,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdateGuardian_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	body := map[string]any{
		"first_name": "Updated",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/invalid", body,
		bearer(t, testutil.DefaultTestClaims()),
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

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/99999", nil,
		bearer(t, nonStaffClaims("users:delete")),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestDeleteGuardian_NotFound(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	// Use admin claims with admin:* permission - guardian delete requires admin or group supervisor
	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/99999", nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestDeleteGuardian_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/invalid", nil,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGuardianDeletePreview_NotFound(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/99999/delete-preview", nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGuardianDeletePreview_SuccessIncludesAffectedLinkIDs(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, studentID := createLinkedGuardian(t, ctx, "delete-preview")
	defer cleanupGuardian(t, ctx.db, guardianID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, studentID)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/%d/delete-preview", guardianID), nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "expected data object in response: %s", rr.Body.String())
	assert.Equal(t, float64(1), data["linked_count"])
	assert.Len(t, data["affected_names"], 1)
	assert.Len(t, data["affected_link_ids"], 1)
}

// createLinkedGuardian creates a guardian profile linked to a fresh student in
// tenant 1 and returns both IDs. The caller is responsible for cleanup
// (cleanupGuardian removes the link + guardian; the student is left for the
// test's own teardown).
func createLinkedGuardian(t *testing.T, ctx *testContext, emailSeed string) (guardianID, studentID int64) {
	t.Helper()
	tenantCtx := testpkg.TenantContext(1)
	guardian := testpkg.CreateTestGuardianProfile(t, ctx.db, emailSeed)
	student := testpkg.CreateTestStudent(t, ctx.db, "Linked", "Child", "1a")
	_, err := ctx.services.Guardian.LinkGuardianToStudent(tenantCtx, usersSvc.StudentGuardianCreateRequest{
		StudentID:         student.ID,
		GuardianProfileID: guardian.ID,
		RelationshipType:  "parent",
		EmergencyPriority: 1,
	})
	require.NoError(t, err)
	return guardian.ID, student.ID
}

func linkedGuardianErrorText(t *testing.T, rrBody string) string {
	t.Helper()
	response := testutil.ParseJSONResponse(t, []byte(rrBody))
	text, ok := response["error"].(string)
	require.True(t, ok, "expected error text in response: %s", rrBody)
	return text
}

// TestDeleteGuardian_WithLinks_Conflict verifies that deleting a guardian that
// is still linked to a student, WITHOUT ?force=true, is refused with 409 rather
// than silently unlinking siblings (#819).
func TestDeleteGuardian_WithLinks_Conflict(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, studentID := createLinkedGuardian(t, ctx, "delete-conflict")
	defer cleanupGuardian(t, ctx.db, guardianID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, studentID)

	sibling := testpkg.CreateTestStudent(t, ctx.db, "Linked", "Sibling", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, sibling.ID)
	_, err := ctx.services.Guardian.LinkGuardianToStudent(testpkg.TenantContext(1), usersSvc.StudentGuardianCreateRequest{
		StudentID:         sibling.ID,
		GuardianProfileID: guardianID,
		RelationshipType:  "parent",
		EmergencyPriority: 2,
	})
	require.NoError(t, err)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d", guardianID), nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusConflict)
	assert.Contains(t, linkedGuardianErrorText(t, rr.Body.String()), "Linked Child")

	// Guardian must still exist after the refused delete.
	survivor, err := ctx.services.Guardian.GetGuardianByID(testpkg.TenantContext(1), guardianID)
	require.NoError(t, err)
	assert.NotNil(t, survivor)
}

func TestDeleteGuardian_WithLinks_NonAdminConflictDoesNotExposeNames(t *testing.T) {
	ctx := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Delete", "Supervisor")
	defer testpkg.CleanupAuthFixtures(t, ctx.db, account.ID)

	group := testpkg.CreateTestEducationGroup(t, ctx.db, "DeletePrivacy")
	testpkg.CreateTestGroupTeacher(t, ctx.db, group.ID, teacher.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, group.ID, teacher.Staff.ID, teacher.ID)

	guardian := testpkg.CreateTestGuardianProfile(t, ctx.db, "delete-privacy")
	defer cleanupGuardian(t, ctx.db, guardian.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, "Private", "Child", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)
	_, err := ctx.db.NewUpdate().
		TableExpr("users.students").
		Set("group_id = ?", group.ID).
		Where("id = ?", student.ID).
		Where("tenant_id = ?", 1).
		Exec(testpkg.TenantContext(1))
	require.NoError(t, err)

	_, err = ctx.services.Guardian.LinkGuardianToStudent(testpkg.TenantContext(1), usersSvc.StudentGuardianCreateRequest{
		StudentID:         student.ID,
		GuardianProfileID: guardian.ID,
		RelationshipType:  "parent",
		EmergencyPriority: 1,
	})
	require.NoError(t, err)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d", guardian.ID), nil,
		bearer(t, withPerms(testutil.TeacherTestClaims(int(account.ID)), "users:delete")),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusConflict)
	errText := linkedGuardianErrorText(t, rr.Body.String())
	assert.Contains(t, errText, "Noch mit Kindern verknüpft")
	assert.NotContains(t, errText, "Private")
	assert.NotContains(t, errText, "Child")
}

// TestDeleteGuardian_WithLinks_ForceAdmin_Success verifies that an admin can
// fully delete a linked guardian with ?force=true: the guardian and all its
// student links are removed.
func TestDeleteGuardian_WithLinks_ForceAdmin_Success(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, studentID := createLinkedGuardian(t, ctx, "delete-force")
	defer cleanupGuardian(t, ctx.db, guardianID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, studentID)

	router := ctx.resource.Router()

	impact, err := ctx.services.Guardian.GetGuardianDeleteImpact(testpkg.TenantContext(1), guardianID)
	require.NoError(t, err)
	require.Len(t, impact.LinkIDs, 1)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d?force=true&expected_link_ids=%d", guardianID, impact.LinkIDs[0]), nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Guardian is gone.
	gone, _ := ctx.services.Guardian.GetGuardianByID(testpkg.TenantContext(1), guardianID)
	assert.Nil(t, gone)
}

func TestDeleteGuardian_WithLinks_ForceAdminRejectsStalePreview(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, studentID := createLinkedGuardian(t, ctx, "delete-force-stale")
	defer cleanupGuardian(t, ctx.db, guardianID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, studentID)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d?force=true&expected_link_ids=999999", guardianID), nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusConflict)
	assert.Contains(t, linkedGuardianErrorText(t, rr.Body.String()), "Vorschau")

	survivor, err := ctx.services.Guardian.GetGuardianByID(testpkg.TenantContext(1), guardianID)
	require.NoError(t, err)
	assert.NotNil(t, survivor)
}

// TestDeleteGuardian_WithLinks_ForceNonAdmin_Forbidden verifies that a
// non-admin supervisor cannot full-delete a linked guardian even with
// ?force=true: the full delete reaches across siblings they may not supervise,
// so the blast radius is restricted to admins (403). The guardian survives.
func TestDeleteGuardian_WithLinks_ForceNonAdmin_Forbidden(t *testing.T) {
	ctx := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Force", "Supervisor")
	defer testpkg.CleanupAuthFixtures(t, ctx.db, account.ID)

	group := testpkg.CreateTestEducationGroup(t, ctx.db, "ForceForbidden")
	testpkg.CreateTestGroupTeacher(t, ctx.db, group.ID, teacher.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, group.ID, teacher.Staff.ID, teacher.ID)

	guardian := testpkg.CreateTestGuardianProfile(t, ctx.db, "force-forbidden")
	defer cleanupGuardian(t, ctx.db, guardian.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, "Force", "Child", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)
	_, err := ctx.db.NewUpdate().
		TableExpr("users.students").
		Set("group_id = ?", group.ID).
		Where("id = ?", student.ID).
		Where("tenant_id = ?", 1).
		Exec(testpkg.TenantContext(1))
	require.NoError(t, err)

	_, err = ctx.services.Guardian.LinkGuardianToStudent(testpkg.TenantContext(1), usersSvc.StudentGuardianCreateRequest{
		StudentID:         student.ID,
		GuardianProfileID: guardian.ID,
		RelationshipType:  "parent",
		EmergencyPriority: 1,
	})
	require.NoError(t, err)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d?force=true", guardian.ID), nil,
		bearer(t, withPerms(testutil.TeacherTestClaims(int(account.ID)), "users:delete")),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)

	// Guardian must still exist after the refused force delete.
	survivor, err := ctx.services.Guardian.GetGuardianByID(testpkg.TenantContext(1), guardian.ID)
	require.NoError(t, err)
	assert.NotNil(t, survivor)
}

// =============================================================================
// SPECIAL LIST ENDPOINTS TESTS
// =============================================================================

func TestListGuardiansWithoutAccount_Success(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/without-account", nil,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]any)
	require.True(t, ok, "Expected data to be an array")
}

func TestListInvitableGuardians_Success(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/invitable", nil,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]any)
	require.True(t, ok, "Expected data to be an array")
}

func TestListPendingInvitations_Success(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/invitations/pending", nil,
		bearer(t, testutil.DefaultTestClaims()),
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

	student := testpkg.CreateTestStudent(t, ctx.db, "Guardian", "TestStudent", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/students/%d/guardians", student.ID), nil,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]any)
	require.True(t, ok, "Expected data to be an array")
}

func TestGetStudentGuardians_InvalidStudentID(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/students/invalid/guardians", nil,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestStudentGuardianEndpoints_AlumnusRejected(t *testing.T) {
	ctx := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, ctx.db, "Former", "Student", "4a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	_, err := ctx.db.NewUpdate().
		TableExpr("users.students").
		Set("status = ?", string(userModels.StudentStatusAlumnus)).
		Where("id = ?", student.ID).
		Exec(t.Context())
	require.NoError(t, err)

	router := ctx.resource.Router()

	t.Run("guardian read is not found", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/students/%d/guardians", student.ID), nil,
			bearer(t, testutil.DefaultTestClaims()),
		)
		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertNotFound(t, rr)
	})

	t.Run("admin cannot link guardian", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/students/%d/guardians", student.ID), map[string]any{},
			bearer(t, testutil.AdminTestClaims(999)),
		)
		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertForbidden(t, rr)
	})
}

func TestGetGuardianStudents_NonExistent_ReturnsEmptyArray(t *testing.T) {
	// API returns 200 with empty array for non-existent guardian (valid design choice)
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/99999/students", nil,
		bearer(t, testutil.DefaultTestClaims()),
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

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/invalid/students", nil,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// INVITATION VALIDATION TESTS (PUBLIC ENDPOINTS)
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()
	assert.NotNil(t, router, "Router should not be nil")
}

// =============================================================================
// LINK GUARDIAN TO STUDENT TESTS
// =============================================================================

func TestLinkGuardianToStudent_Forbidden_NonStaff(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	body := map[string]any{
		"guardian_profile_id":  1,
		"relationship_type":    "parent",
		"is_primary":           true,
		"is_emergency_contact": true,
		"can_pickup":           true,
		"emergency_priority":   1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/students/1/guardians", body,
		bearer(t, nonStaffClaims("users:create")),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestLinkGuardianToStudent_InvalidStudentID(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	body := map[string]any{
		"guardian_profile_id": 1,
		"relationship_type":   "parent",
		"emergency_priority":  1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/students/invalid/guardians", body,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestLinkGuardianToStudent_BadRequest_MissingGuardianID(t *testing.T) {
	ctx := setupTestContext(t)

	// Create a student with a group for the test
	student := testpkg.CreateTestStudent(t, ctx.db, "Link", "TestStudent", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := ctx.resource.Router()

	body := map[string]any{
		"relationship_type":  "parent",
		"emergency_priority": 1,
	}

	// Use admin permissions
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/students/%d/guardians", student.ID), body,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestLinkGuardianToStudent_BadRequest_MissingRelationshipType(t *testing.T) {
	ctx := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, ctx.db, "Link2", "TestStudent", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := ctx.resource.Router()

	body := map[string]any{
		"guardian_profile_id": 1,
		"emergency_priority":  1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/students/%d/guardians", student.ID), body,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestLinkGuardianToStudent_BadRequest_InvalidEmergencyPriority(t *testing.T) {
	ctx := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, ctx.db, "Link3", "TestStudent", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := ctx.resource.Router()

	body := map[string]any{
		"guardian_profile_id": 1,
		"relationship_type":   "parent",
		"emergency_priority":  0, // Invalid - must be at least 1
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/students/%d/guardians", student.ID), body,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// UPDATE STUDENT-GUARDIAN RELATIONSHIP TESTS
// =============================================================================

func TestUpdateStudentGuardianRelationship_InvalidRelationshipID(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	body := map[string]any{
		"is_primary": true,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/relationships/invalid", body,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdateStudentGuardianRelationship_NotFound(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	body := map[string]any{
		"is_primary": true,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/relationships/99999", body,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// REMOVE GUARDIAN FROM STUDENT TESTS
// =============================================================================

func TestRemoveGuardianFromStudent_Forbidden_NonStaff(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/students/1/guardians/1", nil,
		bearer(t, nonStaffClaims("users:delete")),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestRemoveGuardianFromStudent_InvalidStudentID(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/students/invalid/guardians/1", nil,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestRemoveGuardianFromStudent_InvalidGuardianID(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/students/1/guardians/invalid", nil,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// SEND INVITATION TESTS
// =============================================================================

func TestSendInvitation_InvalidGuardianID(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "POST", "/invalid/invite", nil,
		bearer(t, testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestSendInvitation_Unauthorized_NoClaims(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	// Request without an Authorization header — the Verifier/Authenticator
	// middleware rejects it before the handler runs.
	req := testutil.NewJSONRequest(t, "POST", "/1/invite", nil)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertUnauthorized(t, rr)
}

func TestSendInvitation_SeedTokenHeaderDoesNotExposeTokenOutsideLocalDev(t *testing.T) {
	ctx := setupTestContext(t)

	prevEnv := viper.GetString("app_env")
	t.Cleanup(func() { viper.Set("app_env", prevEnv) })

	router := ctx.resource.Router()

	for _, appEnv := range []string{"production", "staging", "preview", "developement", ""} {
		t.Run(fmt.Sprintf("app_env=%q", appEnv), func(t *testing.T) {
			viper.Set("app_env", appEnv)

			guardian := testpkg.CreateTestGuardianProfile(t, ctx.db, "invite-"+appEnv)
			defer cleanupGuardian(t, ctx.db, guardian.ID)

			req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/invite", guardian.ID), nil,
				bearer(t, withPerms(testutil.DefaultTestClaims(), "users:create")),
			)
			req.Header.Set("X-Phoenix-Seed-Token", "true")

			rr := testutil.ExecuteRequest(router, req)

			testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

			response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
			data, ok := response["data"].(map[string]any)
			require.True(t, ok, "Expected data to be an object")
			assert.NotZero(t, data["id"])
			assert.NotContains(t, data, "token")
		})
	}
}

// =============================================================================
// VALIDATE INVITATION TESTS
// =============================================================================

func createTestGuardianWithPhones(t *testing.T, ctx *testContext) (int64, int64, int64) {
	t.Helper()

	router := ctx.resource.Router()
	adminBearer := bearer(t, testutil.AdminTestClaims(999))

	// Create guardian profile
	guardianReq := map[string]any{
		"first_name":               fmt.Sprintf("TestGuardian-%d", time.Now().UnixNano()),
		"last_name":                "PhoneTest",
		"email":                    fmt.Sprintf("phone-%d@test.com", time.Now().UnixNano()),
		"preferred_contact_method": "phone",
		"language_preference":      "de",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", guardianReq, adminBearer)
	rr := testutil.ExecuteRequest(router, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardianID := int64(data["id"].(float64))

	// Primary phone
	phoneReq1 := map[string]any{
		"phone_number": "+49 123 456789",
		"phone_type":   "mobile",
		"is_primary":   true,
	}
	req1 := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/phone-numbers", guardianID), phoneReq1, adminBearer)
	rr1 := testutil.ExecuteRequest(router, req1)
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
	req2 := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/phone-numbers", guardianID), phoneReq2, adminBearer)
	rr2 := testutil.ExecuteRequest(router, req2)
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

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/%d/phone-numbers", guardianID), nil,
		bearer(t, withPerms(testutil.DefaultTestClaims(), "users:read")),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	phones := response["data"].([]any)
	assert.Len(t, phones, 2, "Expected 2 phone numbers")
}

func TestListGuardianPhoneNumbers_InvalidGuardianID(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/invalid/phone-numbers", nil,
		bearer(t, withPerms(testutil.DefaultTestClaims(), "users:read")),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestListGuardianPhoneNumbers_EmptyList(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	// Create guardian without phones
	guardianReq := map[string]any{
		"first_name":               fmt.Sprintf("NoPhone-%d", time.Now().UnixNano()),
		"last_name":                "Test",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", guardianReq,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardianID := int64(data["id"].(float64))
	defer cleanupGuardian(t, ctx.db, guardianID)

	// List phones
	listReq := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/%d/phone-numbers", guardianID), nil,
		bearer(t, withPerms(testutil.AdminTestClaims(999), "users:read")),
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

	router := ctx.resource.Router()
	adminBearer := bearer(t, testutil.AdminTestClaims(999))

	// Create guardian without phones first
	guardianReq := map[string]any{
		"first_name":               fmt.Sprintf("AddPhone-%d", time.Now().UnixNano()),
		"last_name":                "Test",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", guardianReq, adminBearer)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardianID := int64(data["id"].(float64))
	defer cleanupGuardian(t, ctx.db, guardianID)

	// Add phone number
	phoneReq := map[string]any{
		"phone_number": "+49 123 456789",
		"phone_type":   "mobile",
		"label":        "Personal",
		"is_primary":   true,
	}

	phoneAddReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/phone-numbers", guardianID), phoneReq, adminBearer)

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

	router := ctx.resource.Router()

	phoneReq := map[string]any{
		"phone_number": "+49 123 456789",
		"phone_type":   "mobile",
		"is_primary":   true,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/1/phone-numbers", phoneReq,
		bearer(t, nonStaffClaims("users:update")),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestAddPhoneNumber_InvalidGuardianID(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	phoneReq := map[string]any{
		"phone_number": "+49 123 456789",
		"phone_type":   "mobile",
		"is_primary":   true,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/invalid/phone-numbers", phoneReq,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAddPhoneNumber_BadRequest_MissingPhoneNumber(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	phoneReq := map[string]any{
		"phone_type": "mobile",
		"is_primary": true,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/phone-numbers", guardianID), phoneReq,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAddPhoneNumber_BadRequest_InvalidPhoneType(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	phoneReq := map[string]any{
		"phone_number": "+49 123 456789",
		"phone_type":   "invalid_type",
		"is_primary":   true,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/phone-numbers", guardianID), phoneReq,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAddPhoneNumber_DefaultPhoneType(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()
	adminBearer := bearer(t, testutil.AdminTestClaims(999))

	// Create guardian without phones first
	guardianReq := map[string]any{
		"first_name":               fmt.Sprintf("DefaultType-%d", time.Now().UnixNano()),
		"last_name":                "Test",
		"preferred_contact_method": "email",
		"language_preference":      "de",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", guardianReq, adminBearer)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardianID := int64(data["id"].(float64))
	defer cleanupGuardian(t, ctx.db, guardianID)

	// Add phone without specifying type
	phoneReq := map[string]any{
		"phone_number": "+49 123 456789",
		"is_primary":   true,
	}

	phoneAddReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/phone-numbers", guardianID), phoneReq, adminBearer)

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

	guardianID, phone1ID, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	updateReq := map[string]any{
		"phone_number": "+49 111 222333",
		"phone_type":   "home",
		"label":        "Updated Label",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/phone-numbers/%d", guardianID, phone1ID), updateReq,
		bearer(t, testutil.AdminTestClaims(999)),
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

	router := ctx.resource.Router()

	updateReq := map[string]any{
		"phone_number": "+49 111 222333",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/invalid/phone-numbers/1", updateReq,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePhoneNumber_InvalidPhoneID(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	updateReq := map[string]any{
		"phone_number": "+49 111 222333",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/phone-numbers/invalid", guardianID), updateReq,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePhoneNumber_NotFound(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	updateReq := map[string]any{
		"phone_number": "+49 111 222333",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/phone-numbers/99999", guardianID), updateReq,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdatePhoneNumber_Forbidden_NonStaff(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	updateReq := map[string]any{
		"phone_number": "+49 111 222333",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/1/phone-numbers/1", updateReq,
		bearer(t, nonStaffClaims("users:update")),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestUpdatePhoneNumber_BadRequest_EmptyPhoneNumber(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, phone1ID, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	emptyPhone := ""
	updateReq := map[string]any{
		"phone_number": &emptyPhone,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/phone-numbers/%d", guardianID, phone1ID), updateReq,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePhoneNumber_BadRequest_InvalidPhoneType(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, phone1ID, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	invalidType := "invalid_type"
	updateReq := map[string]any{
		"phone_type": &invalidType,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/phone-numbers/%d", guardianID, phone1ID), updateReq,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePhoneNumber_Forbidden_PhoneNotBelongToGuardian(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()
	adminBearer := bearer(t, testutil.AdminTestClaims(999))

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

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", guardianReq, adminBearer)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardian2ID := int64(data["id"].(float64))
	defer cleanupGuardian(t, ctx.db, guardian2ID)

	// Try to update guardian1's phone via guardian2's endpoint
	updateReq := map[string]any{
		"phone_number": "+49 111 222333",
	}

	updatePhoneReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/phone-numbers/%d", guardian2ID, phone1ID), updateReq, adminBearer)

	updateRr := testutil.ExecuteRequest(router, updatePhoneReq)

	testutil.AssertForbidden(t, updateRr)
}

// =============================================================================
// DELETE PHONE NUMBER TESTS
// =============================================================================

func TestDeletePhoneNumber_Success(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, _, phone2ID := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d/phone-numbers/%d", guardianID, phone2ID), nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeletePhoneNumber_InvalidGuardianID(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/invalid/phone-numbers/1", nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestDeletePhoneNumber_InvalidPhoneID(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d/phone-numbers/invalid", guardianID), nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestDeletePhoneNumber_NotFound(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d/phone-numbers/99999", guardianID), nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestDeletePhoneNumber_Forbidden_NonStaff(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/1/phone-numbers/1", nil,
		bearer(t, nonStaffClaims("users:update")),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestDeletePhoneNumber_Forbidden_PhoneNotBelongToGuardian(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()
	adminBearer := bearer(t, testutil.AdminTestClaims(999))

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

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", guardianReq, adminBearer)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardian2ID := int64(data["id"].(float64))
	defer cleanupGuardian(t, ctx.db, guardian2ID)

	// Try to delete guardian1's phone via guardian2's endpoint
	deleteReq := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d/phone-numbers/%d", guardian2ID, phone1ID), nil, adminBearer)

	deleteRr := testutil.ExecuteRequest(router, deleteReq)

	testutil.AssertForbidden(t, deleteRr)
}

// =============================================================================
// SET PRIMARY PHONE TESTS
// =============================================================================

func TestSetPrimaryPhone_Success(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, _, phone2ID := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/phone-numbers/%d/set-primary", guardianID, phone2ID), nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	phoneData := response["data"].(map[string]any)
	assert.True(t, phoneData["is_primary"].(bool), "Expected phone to be set as primary")
}

func TestSetPrimaryPhone_InvalidGuardianID(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "POST", "/invalid/phone-numbers/1/set-primary", nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestSetPrimaryPhone_InvalidPhoneID(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/phone-numbers/invalid/set-primary", guardianID), nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestSetPrimaryPhone_NotFound(t *testing.T) {
	ctx := setupTestContext(t)

	guardianID, _, _ := createTestGuardianWithPhones(t, ctx)
	defer cleanupGuardian(t, ctx.db, guardianID)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/phone-numbers/99999/set-primary", guardianID), nil,
		bearer(t, testutil.AdminTestClaims(999)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestSetPrimaryPhone_Forbidden_NonStaff(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "POST", "/1/phone-numbers/1/set-primary", nil,
		bearer(t, nonStaffClaims("users:update")),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertForbidden(t, rr)
}

func TestSetPrimaryPhone_Forbidden_PhoneNotBelongToGuardian(t *testing.T) {
	ctx := setupTestContext(t)

	router := ctx.resource.Router()
	adminBearer := bearer(t, testutil.AdminTestClaims(999))

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

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", guardianReq, adminBearer)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	guardian2ID := int64(data["id"].(float64))
	defer cleanupGuardian(t, ctx.db, guardian2ID)

	// Try to set guardian1's phone as primary via guardian2's endpoint
	setPrimaryReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/phone-numbers/%d/set-primary", guardian2ID, phone1ID), nil, adminBearer)

	setPrimaryRr := testutil.ExecuteRequest(router, setPrimaryReq)

	testutil.AssertForbidden(t, setPrimaryRr)
}
