// Package admin_test tests the admin API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package admin_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
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
	resource := adminAPI.NewGradeTransitionResource(svc.GradeTransition)

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
		Sub:         "admin@example.com",
		Username:    "admin",
		FirstName:   "Admin",
		LastName:    "User",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:*", permissions.GradeTransitionsRead, permissions.GradeTransitionsCreate, permissions.GradeTransitionsUpdate, permissions.GradeTransitionsDelete, permissions.GradeTransitionsApply},
		IsAdmin:     true,
	}
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

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/admin/grade-transitions", tc.resource.Router())

	t.Run("list returns transitions", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/admin/grade-transitions/", nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		assert.NotNil(t, response["data"])
	})

	t.Run("list with status filter", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/admin/grade-transitions/?status=draft", nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("list with academic_year filter", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/admin/grade-transitions/?academic_year=2025-2026", nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("list with pagination", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/admin/grade-transitions/?page=1&page_size=1", nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("list requires permission", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/admin/grade-transitions/", nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(), // No permissions
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

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/admin/grade-transitions", tc.resource.Router())

	t.Run("create transition without mappings", func(t *testing.T) {
		body := map[string]interface{}{
			"academic_year": "2030-2031",
		}

		req := testutil.NewAuthenticatedRequest(t, "POST", "/admin/grade-transitions/", body,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsCreate),
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

		req := testutil.NewAuthenticatedRequest(t, "POST", "/admin/grade-transitions/", body,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsCreate),
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

		req := testutil.NewAuthenticatedRequest(t, "POST", "/admin/grade-transitions/", body,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsCreate),
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

		req := testutil.NewAuthenticatedRequest(t, "POST", "/admin/grade-transitions/", body,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsCreate),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertBadRequest(t, rr)
	})

	t.Run("create requires permission", func(t *testing.T) {
		body := map[string]interface{}{
			"academic_year": "2033-2034",
		}

		req := testutil.NewAuthenticatedRequest(t, "POST", "/admin/grade-transitions/", body,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(), // No permissions
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
	testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, "1a", strPtr("2a"))
	defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/admin/grade-transitions", tc.resource.Router())

	t.Run("get transition by ID", func(t *testing.T) {
		url := fmt.Sprintf("/admin/grade-transitions/%d", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "GET", url, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data := response["data"].(map[string]interface{})
		assert.Equal(t, float64(transition.ID), data["id"])
		assert.Equal(t, "2025-2026", data["academic_year"])
	})

	t.Run("get non-existent transition returns 404", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/admin/grade-transitions/999999", nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertNotFound(t, rr)
	})

	t.Run("get with invalid ID returns 400", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/admin/grade-transitions/invalid", nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
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

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/admin/grade-transitions", tc.resource.Router())

	t.Run("update transition notes", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		notes := "Updated notes"
		body := map[string]interface{}{
			"academic_year": "2025-2026",
			"notes":         notes,
		}

		url := fmt.Sprintf("/admin/grade-transitions/%d", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "PUT", url, body,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsUpdate),
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

		url := fmt.Sprintf("/admin/grade-transitions/%d", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "PUT", url, body,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsUpdate),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("update non-existent transition returns error", func(t *testing.T) {
		body := map[string]interface{}{
			"academic_year": "2025-2026",
		}

		req := testutil.NewAuthenticatedRequest(t, "PUT", "/admin/grade-transitions/999999", body,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsUpdate),
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

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/admin/grade-transitions", tc.resource.Router())

	t.Run("delete draft transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		// No defer cleanup needed - we're testing delete

		url := fmt.Sprintf("/admin/grade-transitions/%d", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "DELETE", url, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsDelete),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("delete non-existent transition returns error", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "DELETE", "/admin/grade-transitions/999999", nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsDelete),
		)

		rr := testutil.ExecuteRequest(router, req)
		assert.NotEqual(t, http.StatusOK, rr.Code)
	})

	t.Run("delete requires permission", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		url := fmt.Sprintf("/admin/grade-transitions/%d", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "DELETE", url, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(), // No permissions
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

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/admin/grade-transitions", tc.resource.Router())

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

		url := fmt.Sprintf("/admin/grade-transitions/%d/preview", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "GET", url, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data := response["data"].(map[string]interface{})
		assert.NotNil(t, data["transition_id"])
		assert.NotNil(t, data["total_students"])
	})

	t.Run("preview non-existent transition returns error", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/admin/grade-transitions/999999/preview", nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
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

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/admin/grade-transitions", tc.resource.Router())

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

		url := fmt.Sprintf("/admin/grade-transitions/%d/apply", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "POST", url, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsApply),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data := response["data"].(map[string]interface{})
		assert.Equal(t, education.TransitionStatusApplied, data["status"])
	})

	t.Run("apply requires permission", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, "9x", strPtr("10x"))
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		url := fmt.Sprintf("/admin/grade-transitions/%d/apply", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "POST", url, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(), // No permissions
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertForbidden(t, rr)
	})

	t.Run("apply non-existent transition returns error", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "POST", "/admin/grade-transitions/999999/apply", nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsApply),
		)

		rr := testutil.ExecuteRequest(router, req)
		assert.NotEqual(t, http.StatusOK, rr.Code)
	})
}

// ============================================================================
// Revert Tests
// ============================================================================

func TestGradeTransitionResource_Revert(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "revert-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/admin/grade-transitions", tc.resource.Router())

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
		applyURL := fmt.Sprintf("/admin/grade-transitions/%d/apply", transition.ID)
		applyReq := testutil.NewAuthenticatedRequest(t, "POST", applyURL, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsApply),
		)
		applyRR := testutil.ExecuteRequest(router, applyReq)
		require.Equal(t, http.StatusOK, applyRR.Code)

		// Now revert
		revertURL := fmt.Sprintf("/admin/grade-transitions/%d/revert", transition.ID)
		revertReq := testutil.NewAuthenticatedRequest(t, "POST", revertURL, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsApply),
		)

		revertRR := testutil.ExecuteRequest(router, revertReq)
		testutil.AssertSuccessResponse(t, revertRR, http.StatusOK)

		response := testutil.ParseJSONResponse(t, revertRR.Body.Bytes())
		data := response["data"].(map[string]interface{})
		assert.Equal(t, education.TransitionStatusReverted, data["status"])
	})

	t.Run("revert draft transition fails", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, "8x", strPtr("9x"))
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		url := fmt.Sprintf("/admin/grade-transitions/%d/revert", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "POST", url, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsApply),
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

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/admin/grade-transitions", tc.resource.Router())

	t.Run("get distinct classes", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/admin/grade-transitions/classes", nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
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

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/admin/grade-transitions", tc.resource.Router())

	t.Run("suggest mappings", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "GET", "/admin/grade-transitions/suggest", nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
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

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/admin/grade-transitions", tc.resource.Router())

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
		applyURL := fmt.Sprintf("/admin/grade-transitions/%d/apply", transition.ID)
		applyReq := testutil.NewAuthenticatedRequest(t, "POST", applyURL, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsApply),
		)
		applyRR := testutil.ExecuteRequest(router, applyReq)
		require.Equal(t, http.StatusOK, applyRR.Code)

		// Get history
		historyURL := fmt.Sprintf("/admin/grade-transitions/%d/history", transition.ID)
		historyReq := testutil.NewAuthenticatedRequest(t, "GET", historyURL, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
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

		url := fmt.Sprintf("/admin/grade-transitions/%d/history", transition.ID)
		req := testutil.NewAuthenticatedRequest(t, "GET", url, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
		)

		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data := response["data"].([]interface{})
		assert.Empty(t, data)
	})
}

// ============================================================================
// TransitionRequest Bind Tests
// ============================================================================

func TestTransitionRequest_Bind(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "bind-test@example.com")
	defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/admin/grade-transitions", tc.resource.Router())

	t.Run("bind fails with missing academic_year", func(t *testing.T) {
		body := map[string]interface{}{
			"notes": "Some notes",
		}

		req := testutil.NewAuthenticatedRequest(t, "POST", "/admin/grade-transitions/", body,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsCreate),
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

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/admin/grade-transitions", tc.resource.Router())

	t.Run("response includes applied_at and applied_by", func(t *testing.T) {
		// Create unique class names
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1e-%s", suffix)
		toClass := fmt.Sprintf("2e-%s", suffix)

		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, fromClass, &toClass)
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		// Apply transition
		applyURL := fmt.Sprintf("/admin/grade-transitions/%d/apply", transition.ID)
		applyReq := testutil.NewAuthenticatedRequest(t, "POST", applyURL, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsApply),
		)
		applyRR := testutil.ExecuteRequest(router, applyReq)
		require.Equal(t, http.StatusOK, applyRR.Code)

		// Get the transition
		getURL := fmt.Sprintf("/admin/grade-transitions/%d", transition.ID)
		getReq := testutil.NewAuthenticatedRequest(t, "GET", getURL, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
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
		applyURL := fmt.Sprintf("/admin/grade-transitions/%d/apply", transition.ID)
		applyReq := testutil.NewAuthenticatedRequest(t, "POST", applyURL, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsApply),
		)
		testutil.ExecuteRequest(router, applyReq)

		revertURL := fmt.Sprintf("/admin/grade-transitions/%d/revert", transition.ID)
		revertReq := testutil.NewAuthenticatedRequest(t, "POST", revertURL, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsApply),
		)
		testutil.ExecuteRequest(router, revertReq)

		// Get the transition
		getURL := fmt.Sprintf("/admin/grade-transitions/%d", transition.ID)
		getReq := testutil.NewAuthenticatedRequest(t, "GET", getURL, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
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
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, "1g", strPtr("2g"))
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, "4g", nil) // Graduate
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		getURL := fmt.Sprintf("/admin/grade-transitions/%d", transition.ID)
		getReq := testutil.NewAuthenticatedRequest(t, "GET", getURL, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
		)

		getRR := testutil.ExecuteRequest(router, getReq)
		testutil.AssertSuccessResponse(t, getRR, http.StatusOK)

		response := testutil.ParseJSONResponse(t, getRR.Body.Bytes())
		data := response["data"].(map[string]interface{})
		mappings := data["mappings"].([]interface{})
		assert.Len(t, mappings, 2)

		// Check actions
		actions := make(map[string]bool)
		for _, m := range mappings {
			mapping := m.(map[string]interface{})
			actions[mapping["action"].(string)] = true
		}
		assert.True(t, actions["promote"])
		assert.True(t, actions["graduate"])
	})

	t.Run("response includes can_modify, can_apply, can_revert", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, "1h", strPtr("2h"))
		defer testpkg.CleanupGradeTransitionFixtures(t, tc.db, transition.ID)

		getURL := fmt.Sprintf("/admin/grade-transitions/%d", transition.ID)
		getReq := testutil.NewAuthenticatedRequest(t, "GET", getURL, nil,
			testutil.WithClaims(createAdminClaims(int(account.ID))),
			testutil.WithPermissions(permissions.GradeTransitionsRead),
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

// strPtr returns a pointer to a string
func strPtr(s string) *string {
	return &s
}
