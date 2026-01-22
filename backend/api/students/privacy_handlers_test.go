package students_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// Privacy Consent Tests
// =============================================================================

func TestGetStudentPrivacyConsent(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "Test", "PT1", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.GetStudentPrivacyConsentHandler(), "id")

	t.Run("success_returns_default_consent", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		// Default consent should have renewal_required: true
		assert.Contains(t, rr.Body.String(), "renewal_required")
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/999999", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})
}

func TestUpdateStudentPrivacyConsent(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "PrivacyUpdate", "Test", "PU1", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.UpdateStudentPrivacyConsentHandler(), "id")

	t.Run("success_creates_consent", func(t *testing.T) {
		body := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"data_retention_days": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "1.0")
	})

	t.Run("bad_request_missing_policy_version", func(t *testing.T) {
		body := map[string]interface{}{
			"accepted":            true,
			"data_retention_days": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_retention_days", func(t *testing.T) {
		body := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"data_retention_days": 0, // Invalid: must be 1-31
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestPrivacyConsent_Extended(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("update_creates_new_consent_for_different_version", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "MultiVersion", "PM1", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentPrivacyConsentHandler(), "id")

		// First consent
		body1 := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"data_retention_days": 30,
		}
		req1 := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body1)
		rr1 := executeWithAuth(router, req1, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, rr1.Code, "First consent should succeed")

		// Second consent with different version
		body2 := map[string]interface{}{
			"policy_version":      "2.0",
			"accepted":            true,
			"data_retention_days": 31,
		}
		req2 := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body2)
		rr2 := executeWithAuth(router, req2, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, rr2.Code, "Second consent should succeed")
	})

	t.Run("update_with_duration_days", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "Duration", "PD1", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentPrivacyConsentHandler(), "id")

		body := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"duration_days":       365,
			"data_retention_days": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Consent with duration should succeed")
	})

	t.Run("update_with_details", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "Details", "PDT1", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentPrivacyConsentHandler(), "id")

		// Details should be a map, not a JSON string
		body := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"data_retention_days": 30,
			"details": map[string]interface{}{
				"consent_given_by": "guardian",
				"method":           "form",
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Consent with details should succeed")
	})

	t.Run("forbidden_without_full_access", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "NoAccess", "PNA1", tc.ogsID)
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Privacy", "NoAccess", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		router := setupRouter(tc.resource.GetStudentPrivacyConsentHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:read"})

		// Non-supervisor should be forbidden from viewing privacy consent
		testutil.AssertForbidden(t, rr)
	})
}

func TestPrivacyConsent_EdgeCases(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("update_existing_consent_same_version", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "SameVersion", "PSV1", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentPrivacyConsentHandler(), "id")

		// Create first consent
		body := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"data_retention_days": 30,
		}
		req1 := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr1 := executeWithAuth(router, req1, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, rr1.Code)

		// Update same version (should update existing)
		body["data_retention_days"] = 15
		req2 := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr2 := executeWithAuth(router, req2, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr2.Code, "Should update existing consent")
	})

	t.Run("update_privacy_consent_forbidden", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "Forbidden", "PF1", tc.ogsID)
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Privacy", "ForbiddenStaff", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		router := setupRouter(tc.resource.UpdateStudentPrivacyConsentHandler(), "id")
		body := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"data_retention_days": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:write"})

		// Non-supervisor should be forbidden from updating privacy consent
		testutil.AssertForbidden(t, rr)
	})
}
