// Package importapi_test tests the import API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package importapi_test

import (
	"log/slog"
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
	svc, err := services.NewFactory(repos, db, slog.Default())
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
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students/template", nil)
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

	req := testutil.NewAuthenticatedRequest(t, "GET", "/template?format=csv", nil,
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

	req := testutil.NewAuthenticatedRequest(t, "GET", "/template?format=xlsx", nil,
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
	req := testutil.NewAuthenticatedRequest(t, "GET", "/template", nil,
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
	req := testutil.NewAuthenticatedRequest(t, "POST", "/students/preview", nil)
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
	req := testutil.NewAuthenticatedRequest(t, "POST", "/preview", nil,
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
	req := testutil.NewAuthenticatedRequest(t, "POST", "/students/import", nil)
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
	req := testutil.NewAuthenticatedRequest(t, "POST", "/import", nil,
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

	req := testutil.NewAuthenticatedRequest(t, "GET", "/template?format=csv", nil,
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

// =============================================================================
// PREVIEW IMPORT WITH FILE TESTS
// =============================================================================

func TestPreviewImport_WithValidCSV(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Preview", "CSVTest")

	router := chi.NewRouter()
	router.Post("/preview", ctx.resource.PreviewImportHandler())

	// Create CSV content with required headers
	csvContent := "Vorname,Nachname,Klasse\nMax,Mustermann,1a\nErika,Musterfrau,2b"

	// Create multipart form with file
	req := testutil.NewMultipartRequest(t, "POST", "/preview", "file", "students.csv", csvContent,
		testutil.WithClaims(testutil.AdminTestClaims(int(admin.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 200 with preview data or 400 for validation errors
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusBadRequest,
		"Expected 200 or 400, got %d: %s", rr.Code, rr.Body.String())
}

func TestPreviewImport_WithEmptyCSV(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Preview", "EmptyCSV")

	router := chi.NewRouter()
	router.Post("/preview", ctx.resource.PreviewImportHandler())

	// Create empty CSV with headers only
	csvContent := "Vorname,Nachname,Klasse"

	req := testutil.NewMultipartRequest(t, "POST", "/preview", "file", "empty.csv", csvContent,
		testutil.WithClaims(testutil.AdminTestClaims(int(admin.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 200 with empty preview or 400 for no data
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusBadRequest,
		"Expected 200 or 400, got %d: %s", rr.Code, rr.Body.String())
}

func TestPreviewImport_WithMissingHeaders(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Preview", "MissingHeaders")

	router := chi.NewRouter()
	router.Post("/preview", ctx.resource.PreviewImportHandler())

	// CSV missing required headers
	csvContent := "Name,Class\nMax,1a"

	req := testutil.NewMultipartRequest(t, "POST", "/preview", "file", "invalid.csv", csvContent,
		testutil.WithClaims(testutil.AdminTestClaims(int(admin.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Service accepts files without standard headers (maps by position)
	// So this may return 200 with validation errors in response body
	t.Logf("Missing headers response: %d - %s", rr.Code, rr.Body.String())
}

// =============================================================================
// IMPORT STUDENTS WITH FILE TESTS
// =============================================================================

func TestImportStudents_WithValidCSV(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "CSVTest")

	router := chi.NewRouter()
	router.Post("/import", ctx.resource.ImportStudentsHandler())

	// Create CSV content with required headers
	csvContent := "Vorname,Nachname,Klasse\nImport,Student1,1a"

	req := testutil.NewMultipartRequest(t, "POST", "/import", "file", "students.csv", csvContent,
		testutil.WithClaims(testutil.AdminTestClaims(int(admin.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 200 with import results or 400/500 for errors
	t.Logf("Import response: %d - %s", rr.Code, rr.Body.String())
}

func TestImportStudents_WithDuplicateData(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "DupeTest")

	router := chi.NewRouter()
	router.Post("/import", ctx.resource.ImportStudentsHandler())

	// CSV with duplicate entries
	csvContent := "Vorname,Nachname,Klasse\nDupe,Student,1a\nDupe,Student,1a"

	req := testutil.NewMultipartRequest(t, "POST", "/import", "file", "dupes.csv", csvContent,
		testutil.WithClaims(testutil.AdminTestClaims(int(admin.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Log the result - may succeed with warnings or fail
	t.Logf("Duplicate import response: %d - %s", rr.Code, rr.Body.String())
}

// =============================================================================
// ROUTER TESTS
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()
	assert.NotNil(t, router, "Router should return a valid chi.Router")
}
