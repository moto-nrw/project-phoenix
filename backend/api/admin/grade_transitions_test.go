// Package admin_test tests the admin API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package admin_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	adminAPI "github.com/moto-nrw/project-phoenix/api/admin"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func init() {
	// Router() calls jwt.MustNewTokenAuth(); MintTestJWT signs with the same
	// secret. Seed the deterministic JWT config before any Router construction.
	testutil.SeedTestJWTConfig()
}

// testContext holds shared test resources
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *adminAPI.GradeTransitionResource
}

// setupTestContext creates test resources for grade transition handler tests
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	resource := adminAPI.NewGradeTransitionResource(svc.GradeTransition, db)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// createAdminClaims creates admin JWT claims for testing
func createAdminClaims(accountID int) jwt.AppClaims {
	return jwt.AppClaims{
		ID:          accountID,
		TenantID:    1,
		Sub:         "admin@example.com",
		Username:    "admin",
		FirstName:   "Admin",
		LastName:    "User",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:*", permissions.GradeTransitionsRead, permissions.GradeTransitionsCreate, permissions.GradeTransitionsUpdate, permissions.GradeTransitionsDelete, permissions.GradeTransitionsApply},
		IsAdmin:     true,
	}
}

// mintAdminToken signs a real JWT carrying admin claims (tenant 1, full grade
// transition permissions) so requests pass the production auth chain in Router().
func mintAdminToken(t *testing.T, accountID int64) string {
	t.Helper()
	return testutil.MintTestJWT(t, createAdminClaims(int(accountID)))
}

// mintNoPermissionToken signs a real JWT for the same authenticated account but
// without any grade transition permissions, exercising the RequiresPermission
// middleware's 403 path.
func mintNoPermissionToken(t *testing.T, accountID int64) string {
	t.Helper()
	claims := createAdminClaims(int(accountID))
	claims.Permissions = nil
	claims.Roles = []string{"user"}
	claims.IsAdmin = false
	return testutil.MintTestJWT(t, claims)
}

// ============================================================================
// List Tests
// ============================================================================

func TestGradeTransitionResource_List(t *testing.T) {
	tc := setupTestContext(t)

	// Create test account and transitions
	account := testpkg.CreateTestAccount(t, tc.db, "list-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	t1 := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
	t2 := testpkg.CreateTestGradeTransition(t, tc.db, "2026-2027", account.ID)
	defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, t1.ID, t2.ID)

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	t.Run("list returns transitions", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/", nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		assert.NotNil(t, response["data"])
	})

	t.Run("list with status filter", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/?status=draft", nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("list with academic_year filter", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/?academic_year=2025-2026", nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("list with pagination", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/?page=1&page_size=1", nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("list requires permission", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/", nil,
			testutil.WithJWTBearer(mintNoPermissionToken(t, account.ID)),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertForbidden(t, rr)
	})
}

// ============================================================================
// Create Tests
// ============================================================================

func TestGradeTransitionResource_Create(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "create-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	t.Run("create transition without mappings", func(t *testing.T) {
		body := map[string]interface{}{
			"academic_year": "2030-2031",
		}

		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

		// Parse response to get ID for cleanup
		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		if data, ok := response["data"].(map[string]interface{}); ok {
			if id, ok := data["id"].(float64); ok {
				defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, int64(id))
			}
		}
	})

	t.Run("create transition with mappings", func(t *testing.T) {
		toClass := "2a"
		body := map[string]interface{}{
			"academic_year": "2031-2032",
			"mappings": []map[string]interface{}{
				{"from_class": "1a", "to_class": toClass},
				{"from_class": "4a", "to_class": nil},
			},
		}

		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

		// Parse response to get ID for cleanup
		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		if data, ok := response["data"].(map[string]interface{}); ok {
			if id, ok := data["id"].(float64); ok {
				defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, int64(id))
			}
		}
	})

	t.Run("create transition with notes", func(t *testing.T) {
		notes := "Test notes for transition"
		body := map[string]interface{}{
			"academic_year": "2032-2033",
			"notes":         notes,
		}

		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

		// Parse response to get ID for cleanup
		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		if data, ok := response["data"].(map[string]interface{}); ok {
			if id, ok := data["id"].(float64); ok {
				defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, int64(id))
				assert.Equal(t, notes, data["notes"])
			}
		}
	})

	t.Run("create fails with empty academic_year", func(t *testing.T) {
		body := map[string]interface{}{
			"academic_year": "",
		}

		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertBadRequest(t, rr)
	})

	t.Run("create requires permission", func(t *testing.T) {
		body := map[string]interface{}{
			"academic_year": "2033-2034",
		}

		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body,
			testutil.WithJWTBearer(mintNoPermissionToken(t, account.ID)),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertForbidden(t, rr)
	})
}

// ============================================================================
// GetByID Tests
// ============================================================================

func TestGradeTransitionResource_GetByID(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "getbyid-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, "1a", testpkg.StrPtr("2a"))
	defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	t.Run("get transition by ID", func(t *testing.T) {
		url := fmt.Sprintf("/%d", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "GET", url, nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data := response["data"].(map[string]interface{})
		assert.Equal(t, float64(transition.ID), data["id"])
		assert.Equal(t, "2025-2026", data["academic_year"])
	})

	t.Run("get non-existent transition returns 404", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/999999", nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertNotFound(t, rr)
	})

	t.Run("get with invalid ID returns 400", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/invalid", nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertBadRequest(t, rr)
	})
}

// ============================================================================
// Update Tests
// ============================================================================

func TestGradeTransitionResource_Update(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "update-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	t.Run("update transition notes", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		notes := "Updated notes"
		body := map[string]interface{}{
			"academic_year": "2025-2026",
			"notes":         notes,
		}

		url := fmt.Sprintf("/%d", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "PUT", url, body,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data := response["data"].(map[string]interface{})
		assert.Equal(t, notes, data["notes"])
	})

	t.Run("update transition mappings", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		body := map[string]interface{}{
			"academic_year": "2025-2026",
			"mappings": []map[string]interface{}{
				{"from_class": "2a", "to_class": "3a"},
			},
		}

		url := fmt.Sprintf("/%d", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "PUT", url, body,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("update non-existent transition returns error", func(t *testing.T) {
		body := map[string]interface{}{
			"academic_year": "2025-2026",
		}

		req := testutil.NewAuthenticatedRequest(t, "PUT", "/999999", body,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		assert.NotEqual(t, http.StatusOK, rr.Code)
	})
}

// ============================================================================
// Delete Tests
// ============================================================================

func TestGradeTransitionResource_Delete(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "delete-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	t.Run("delete draft transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		// No defer cleanup needed - we're testing delete

		url := fmt.Sprintf("/%d", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "DELETE", url, nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("delete non-existent transition returns error", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "DELETE", "/999999", nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		assert.NotEqual(t, http.StatusOK, rr.Code)
	})

	t.Run("delete requires permission", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		url := fmt.Sprintf("/%d", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "DELETE", url, nil,
			testutil.WithJWTBearer(mintNoPermissionToken(t, account.ID)),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertForbidden(t, rr)
	})
}

// ============================================================================
// Preview Tests
// ============================================================================

func TestGradeTransitionResource_Preview(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "preview-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	t.Run("preview transition", func(t *testing.T) {
		// Create unique class names for test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1a-%s", suffix)
		toClass := fmt.Sprintf("2a-%s", suffix)

		// Create students in fromClass
		student := testpkg.CreateTestStudent(t, tc.db, "Preview", "Test", fromClass)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, fromClass, &toClass)
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		url := fmt.Sprintf("/%d/preview", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "GET", url, nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data := response["data"].(map[string]interface{})
		assert.NotNil(t, data["transition_id"])
		assert.NotNil(t, data["total_students"])
	})

	t.Run("preview non-existent transition returns error", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/999999/preview", nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		assert.NotEqual(t, http.StatusOK, rr.Code)
	})
}

// ============================================================================
// Apply Tests
// ============================================================================

func TestGradeTransitionResource_Apply(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "apply-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	t.Run("apply transition", func(t *testing.T) {
		// Create unique class names for test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1b-%s", suffix)
		toClass := fmt.Sprintf("2b-%s", suffix)

		// Create students in fromClass
		student := testpkg.CreateTestStudent(t, tc.db, "Apply", "Test", fromClass)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, fromClass, &toClass)
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		url := fmt.Sprintf("/%d/apply", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "POST", url, nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data := response["data"].(map[string]interface{})
		assert.Equal(t, education.TransitionStatusApplied, data["status"])
	})

	t.Run("apply requires permission", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, "9x", testpkg.StrPtr("10x"))
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		url := fmt.Sprintf("/%d/apply", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "POST", url, nil,
			testutil.WithJWTBearer(mintNoPermissionToken(t, account.ID)),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertForbidden(t, rr)
	})

	t.Run("apply non-existent transition returns error", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "POST", "/999999/apply", nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		assert.NotEqual(t, http.StatusOK, rr.Code)
	})

	// #405 P2: a graduating child still checked in is a client-recoverable
	// safety condition — the handler must return 409 with a stable code so the
	// UI can direct the admin to check the child out, not a bare 500.
	t.Run("apply with a checked-in graduate returns 409 with code", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		gradClass := fmt.Sprintf("4apply-%s", suffix)

		activity := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("AG-%s", suffix))
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Room-%s", suffix))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, tc.db, "Checked", "In", gradClass)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, activity.ID, room.ID)

		// Open visit (nil exit time) = currently checked into a room.
		testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, time.Now().Add(-time.Hour), nil)

		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, gradClass, nil) // graduate
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		url := fmt.Sprintf("/%d/apply", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "POST", url, nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		require.Equal(t, http.StatusConflict, rr.Code)
		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		assert.Equal(t, "graduates_checked_in", response["code"])
	})
}

// ============================================================================
// Revert Tests
// ============================================================================

func TestGradeTransitionResource_Revert(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "revert-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	t.Run("revert applied transition", func(t *testing.T) {
		// Create unique class names for test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1c-%s", suffix)
		toClass := fmt.Sprintf("2c-%s", suffix)

		// Create student
		student := testpkg.CreateTestStudent(t, tc.db, "Revert", "Test", fromClass)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		// Create and apply transition
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, fromClass, &toClass)
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		// Apply first
		applyURL := fmt.Sprintf("/%d/apply", transition.ID)
		applyReq := testutil.NewAuthenticatedRequest(t, "POST", applyURL, nil,
			testutil.WithJWTBearer(token),
		)
		applyRR := testutil.ExecuteRequest(router, applyReq)
		require.Equal(t, http.StatusOK, applyRR.Code)

		// Now revert
		revertURL := fmt.Sprintf("/%d/revert", transition.ID)
		revertReq := testutil.NewAuthenticatedRequest(t, "POST", revertURL, nil,
			testutil.WithJWTBearer(token),
		)

		revertRR := testutil.ExecuteRequest(router, revertReq)
		testutil.AssertSuccessResponse(t, revertRR, http.StatusOK)

		response := testutil.ParseJSONResponse(t, revertRR.Body.Bytes())
		data := response["data"].(map[string]interface{})
		assert.Equal(t, education.TransitionStatusReverted, data["status"])
	})

	t.Run("revert draft transition fails", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, "8x", testpkg.StrPtr("9x"))
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		url := fmt.Sprintf("/%d/revert", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "POST", url, nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		assert.NotEqual(t, http.StatusOK, rr.Code)
	})
}

// ============================================================================
// GetDistinctClasses Tests
// ============================================================================

func TestGradeTransitionResource_GetDistinctClasses(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "classes-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Create students with different classes
	s1 := testpkg.CreateTestStudent(t, tc.db, "Class", "Test1", "ClassX")
	s2 := testpkg.CreateTestStudent(t, tc.db, "Class", "Test2", "ClassY")
	defer testpkg.CleanupActivityFixtures(t, tc.db, s1.ID, s2.ID)

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	t.Run("get distinct classes", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/classes", nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data := response["data"].([]interface{})
		assert.NotEmpty(t, data)
	})
}

// ============================================================================
// SuggestMappings Tests
// ============================================================================

func TestGradeTransitionResource_SuggestMappings(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "suggest-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	// Create students in different grades
	s1 := testpkg.CreateTestStudent(t, tc.db, "Suggest", "Test1", "1a")
	s2 := testpkg.CreateTestStudent(t, tc.db, "Suggest", "Test2", "4a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, s1.ID, s2.ID)

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	t.Run("suggest mappings", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/suggest", nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data := response["data"].([]interface{})
		assert.NotEmpty(t, data)
	})
}

// ============================================================================
// GetHistory Tests
// ============================================================================

func TestGradeTransitionResource_GetHistory(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "history-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	t.Run("get history for applied transition", func(t *testing.T) {
		// Create unique class names
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1d-%s", suffix)
		toClass := fmt.Sprintf("2d-%s", suffix)

		// Create student and transition
		student := testpkg.CreateTestStudent(t, tc.db, "History", "Test", fromClass)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, fromClass, &toClass)
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		// Apply transition
		applyURL := fmt.Sprintf("/%d/apply", transition.ID)
		applyReq := testutil.NewAuthenticatedRequest(t, "POST", applyURL, nil,
			testutil.WithJWTBearer(token),
		)
		applyRR := testutil.ExecuteRequest(router, applyReq)
		require.Equal(t, http.StatusOK, applyRR.Code)

		// Get history
		historyURL := fmt.Sprintf("/%d/history", transition.ID)
		historyReq := testutil.NewAuthenticatedRequest(t, "GET", historyURL, nil,
			testutil.WithJWTBearer(token),
		)

		historyRR := testutil.ExecuteRequest(router, historyReq)
		testutil.AssertSuccessResponse(t, historyRR, http.StatusOK)

		response := testutil.ParseJSONResponse(t, historyRR.Body.Bytes())
		data := response["data"].([]interface{})
		assert.NotEmpty(t, data)
	})

	t.Run("get history for transition without apply", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		url := fmt.Sprintf("/%d/history", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "GET", url, nil,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		// API returns null when there's no history, so check for nil or empty array
		if data, ok := response["data"].([]interface{}); ok {
			assert.Empty(t, data)
		} else {
			assert.Nil(t, response["data"], "Expected nil or empty array for history with no records")
		}
	})
}

// ============================================================================
// TransitionRequest Bind Tests
// ============================================================================

func TestTransitionRequest_Bind(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "bind-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	t.Run("bind fails with missing academic_year", func(t *testing.T) {
		body := map[string]interface{}{
			"notes": "Some notes",
		}

		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body,
			testutil.WithJWTBearer(token),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertBadRequest(t, rr)
	})
}

// ============================================================================
// toTransitionResponse Tests
// ============================================================================

func TestToTransitionResponse(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "response-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	t.Run("response includes applied_at and applied_by", func(t *testing.T) {
		// Create unique class names
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1e-%s", suffix)
		toClass := fmt.Sprintf("2e-%s", suffix)

		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, fromClass, &toClass)
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		// Apply transition
		applyURL := fmt.Sprintf("/%d/apply", transition.ID)
		applyReq := testutil.NewAuthenticatedRequest(t, "POST", applyURL, nil,
			testutil.WithJWTBearer(token),
		)
		applyRR := testutil.ExecuteRequest(router, applyReq)
		require.Equal(t, http.StatusOK, applyRR.Code)

		// Get the transition
		getURL := fmt.Sprintf("/%d", transition.ID)
		getReq := testutil.NewAuthenticatedRequest(t, "GET", getURL, nil,
			testutil.WithJWTBearer(token),
		)

		getRR := testutil.ExecuteRequest(router, getReq)
		testutil.AssertSuccessResponse(t, getRR, http.StatusOK)

		response := testutil.ParseJSONResponse(t, getRR.Body.Bytes())
		data := response["data"].(map[string]interface{})
		assert.NotNil(t, data["applied_at"])
		assert.NotNil(t, data["applied_by"])
	})

	t.Run("response includes reverted_at and reverted_by", func(t *testing.T) {
		// Create unique class names
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1f-%s", suffix)
		toClass := fmt.Sprintf("2f-%s", suffix)

		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, fromClass, &toClass)
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		// Apply then revert
		applyURL := fmt.Sprintf("/%d/apply", transition.ID)
		applyReq := testutil.NewAuthenticatedRequest(t, "POST", applyURL, nil,
			testutil.WithJWTBearer(token),
		)
		testutil.ExecuteRequest(router, applyReq)

		revertURL := fmt.Sprintf("/%d/revert", transition.ID)
		revertReq := testutil.NewAuthenticatedRequest(t, "POST", revertURL, nil,
			testutil.WithJWTBearer(token),
		)
		testutil.ExecuteRequest(router, revertReq)

		// Get the transition
		getURL := fmt.Sprintf("/%d", transition.ID)
		getReq := testutil.NewAuthenticatedRequest(t, "GET", getURL, nil,
			testutil.WithJWTBearer(token),
		)

		getRR := testutil.ExecuteRequest(router, getReq)
		testutil.AssertSuccessResponse(t, getRR, http.StatusOK)

		response := testutil.ParseJSONResponse(t, getRR.Body.Bytes())
		data := response["data"].(map[string]interface{})
		assert.NotNil(t, data["reverted_at"])
		assert.NotNil(t, data["reverted_by"])
	})

	t.Run("response includes mappings with action", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, "1g", testpkg.StrPtr("2g"))
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, "4g", nil) // Graduate
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		getURL := fmt.Sprintf("/%d", transition.ID)
		getReq := testutil.NewAuthenticatedRequest(t, "GET", getURL, nil,
			testutil.WithJWTBearer(token),
		)

		getRR := testutil.ExecuteRequest(router, getReq)
		testutil.AssertSuccessResponse(t, getRR, http.StatusOK)

		response := testutil.ParseJSONResponse(t, getRR.Body.Bytes())
		data := response["data"].(map[string]interface{})

		mappingsRaw, hasMappings := data["mappings"]
		require.True(t, hasMappings, "Response should include 'mappings' field")
		require.NotNil(t, mappingsRaw, "Mappings should not be nil")

		mappings, ok := mappingsRaw.([]interface{})
		require.True(t, ok, "mappings should be an array")
		require.Len(t, mappings, 2, "Expected 2 mappings")

		// Check actions
		actions := make(map[string]bool)
		for _, m := range mappings {
			mapping := m.(map[string]interface{})
			if action, ok := mapping["action"].(string); ok {
				actions[action] = true
			}
		}
		assert.True(t, actions["promoted"], "Expected 'promoted' action for mapping with to_class")
		assert.True(t, actions["graduated"], "Expected 'graduated' action for mapping without to_class")
	})

	t.Run("response includes can_modify, can_apply, can_revert", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, "1h", testpkg.StrPtr("2h"))
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		getURL := fmt.Sprintf("/%d", transition.ID)
		getReq := testutil.NewAuthenticatedRequest(t, "GET", getURL, nil,
			testutil.WithJWTBearer(token),
		)

		getRR := testutil.ExecuteRequest(router, getReq)
		testutil.AssertSuccessResponse(t, getRR, http.StatusOK)

		response := testutil.ParseJSONResponse(t, getRR.Body.Bytes())
		data := response["data"].(map[string]interface{})

		// Draft transition with mappings
		assert.True(t, data["can_modify"].(bool))
		assert.True(t, data["can_apply"].(bool))
		assert.False(t, data["can_revert"].(bool))
	})
}

// ============================================================================
// Helper Functions
// ============================================================================
