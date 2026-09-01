package students_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// Privacy Consent Tests
// =============================================================================

func TestGetStudentPrivacyConsent(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "Test", "PT1")

	t.Run("success_returns_default_consent", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/privacy-consent", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		// Default consent should have renewal_required: true
		assert.Contains(t, rr.Body.String(), "renewal_required")
	})

	t.Run("success_returns_configured_retention_default", func(t *testing.T) {
		require.NoError(t, tc.resource.SettingsService.SetValue(testpkg.Ctx(t), configModel.KeyPrivacyConsentRetentionDays, 12, nil, nil))

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/privacy-consent", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), `"data_retention_days":12`)
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/999999/privacy-consent", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})
}

func TestUpdateStudentPrivacyConsent(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "PrivacyUpdate", "Test", "PU1")

	t.Run("success_creates_consent", func(t *testing.T) {
		body := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"data_retention_days": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/privacy-consent", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "1.0")
	})

	t.Run("bad_request_missing_policy_version", func(t *testing.T) {
		body := map[string]interface{}{
			"accepted":            true,
			"data_retention_days": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/privacy-consent", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_retention_days", func(t *testing.T) {
		body := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"data_retention_days": 0, // Invalid: must be 1-31
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/privacy-consent", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestPrivacyConsent_Extended(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("update_creates_new_consent_for_different_version", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "MultiVersion", "PM1")

		// First consent
		body1 := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"data_retention_days": 30,
		}
		req1 := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/privacy-consent", student.ID), body1)
		rr1 := authExec(t, tc, req1, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, rr1.Code, "First consent should succeed")

		// Second consent with different version
		body2 := map[string]interface{}{
			"policy_version":      "2.0",
			"accepted":            true,
			"data_retention_days": 31,
		}
		req2 := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/privacy-consent", student.ID), body2)
		rr2 := authExec(t, tc, req2, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, rr2.Code, "Second consent should succeed")
	})

	t.Run("update_with_duration_days", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "Duration", "PD1")

		body := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"duration_days":       365,
			"data_retention_days": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/privacy-consent", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Consent with duration should succeed")
	})

	t.Run("update_with_details", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "Details", "PDT1")

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
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/privacy-consent", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Consent with details should succeed")
	})

	t.Run("forbidden_without_full_access", func(t *testing.T) {
		// #2329: staff read every child's consent; the surviving denial is an
		// account without a staff record holding users:read.
		student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "NoAccess", "PNA1")
		guest := testpkg.CreateTestAccount(t, tc.db, "privacy-consent-guest@example.com")

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/privacy-consent", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		testutil.AssertForbidden(t, rr)
	})
}

func TestPrivacyConsent_EdgeCases(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("update_existing_consent_same_version", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "SameVersion", "PSV1")

		// Create first consent
		body := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"data_retention_days": 30,
		}
		req1 := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/privacy-consent", student.ID), body)
		rr1 := authExec(t, tc, req1, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, rr1.Code)

		// Update same version (should update existing)
		body["data_retention_days"] = 15
		req2 := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/privacy-consent", student.ID), body)
		rr2 := authExec(t, tc, req2, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr2.Code, "Should update existing consent")
	})

	t.Run("update_privacy_consent_forbidden", func(t *testing.T) {
		// Only accounts without a staff record are refused now (#2329).
		student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "Forbidden", "PF1")
		guest := testpkg.CreateTestAccount(t, tc.db, "privacy-consent-write-guest@example.com")

		body := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"data_retention_days": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/privacy-consent", student.ID), body)

		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		testutil.AssertForbidden(t, rr)
	})
}
