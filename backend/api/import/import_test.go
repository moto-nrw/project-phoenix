// Package importapi_test tests the import API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package importapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	importAPI "github.com/moto-nrw/project-phoenix/api/import"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	repos    *repositories.Factory
	resource *importAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc, err := services.NewFactory(repos, db)
	if err != nil {
		t.Fatalf("Failed to create services factory: %v", err)
	}

	// Create import resource
	resource := importAPI.NewResource(svc.Import, repos.DataImport)

	return &testContext{
		db:       db,
		services: svc,
		repos:    repos,
		resource: resource,
	}
}

// =============================================================================
// DOWNLOAD TEMPLATE TESTS
// =============================================================================

func TestDownloadTemplate_NoAuth(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Use the full router which has JWT middleware
	router := ctx.resource.Router()

	// Request without JWT token should return 401
	req := testutil.NewAuthenticatedRequest("GET", "/students/template", nil)
	req.Header.Del("Authorization")

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing authentication")
}

func TestDownloadTemplate_CSV(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "Admin")

	router := chi.NewRouter()
	router.Get("/template", ctx.resource.DownloadTemplateHandler())

	req := testutil.NewAuthenticatedRequest("GET", "/template?format=csv", nil,
		testutil.WithClaims(testutil.AdminTestClaims(int(admin.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), ".csv")
}

func TestDownloadTemplate_XLSX(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "Admin2")

	router := chi.NewRouter()
	router.Get("/template", ctx.resource.DownloadTemplateHandler())

	req := testutil.NewAuthenticatedRequest("GET", "/template?format=xlsx", nil,
		testutil.WithClaims(testutil.AdminTestClaims(int(admin.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "spreadsheetml")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), ".xlsx")
}

func TestDownloadTemplate_DefaultFormat(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "Admin3")

	router := chi.NewRouter()
	router.Get("/template", ctx.resource.DownloadTemplateHandler())

	// No format parameter - should default to CSV
	req := testutil.NewAuthenticatedRequest("GET", "/template", nil,
		testutil.WithClaims(testutil.AdminTestClaims(int(admin.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/csv")
}

// =============================================================================
// PREVIEW IMPORT TESTS
// =============================================================================

func TestPreviewImport_NoAuth(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()

	// Request without JWT token should return 401
	req := testutil.NewAuthenticatedRequest("POST", "/students/preview", nil)
	req.Header.Del("Authorization")

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing authentication")
}

func TestPreviewImport_NoFile(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "Admin4")

	router := chi.NewRouter()
	router.Post("/preview", ctx.resource.PreviewImportHandler())

	// Request without file upload
	req := testutil.NewAuthenticatedRequest("POST", "/preview", nil,
		testutil.WithClaims(testutil.AdminTestClaims(int(admin.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error for missing file
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusInternalServerError}, rr.Code)
}

// =============================================================================
// IMPORT STUDENTS TESTS
// =============================================================================

func TestImportStudents_NoAuth(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()

	// Request without JWT token should return 401
	req := testutil.NewAuthenticatedRequest("POST", "/students/import", nil)
	req.Header.Del("Authorization")

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing authentication")
}

func TestImportStudents_NoFile(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "Admin5")

	router := chi.NewRouter()
	router.Post("/import", ctx.resource.ImportStudentsHandler())

	// Request without file upload
	req := testutil.NewAuthenticatedRequest("POST", "/import", nil,
		testutil.WithClaims(testutil.AdminTestClaims(int(admin.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error for missing file
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusInternalServerError}, rr.Code)
}

// =============================================================================
// TEMPLATE CONTENT TESTS
// =============================================================================

func TestDownloadTemplate_HasRequiredHeaders(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "Admin6")

	router := chi.NewRouter()
	router.Get("/template", ctx.resource.DownloadTemplateHandler())

	req := testutil.NewAuthenticatedRequest("GET", "/template?format=csv", nil,
		testutil.WithClaims(testutil.AdminTestClaims(int(admin.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Check that required headers are present
	body := rr.Body.String()
	assert.True(t, strings.Contains(body, "Vorname"), "Template should contain Vorname header")
	assert.True(t, strings.Contains(body, "Nachname"), "Template should contain Nachname header")
	assert.True(t, strings.Contains(body, "Klasse"), "Template should contain Klasse header")
}
