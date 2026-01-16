// Package substitutions_test tests the substitutions API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package substitutions_test

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

	substitutionsAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/substitutions"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/testutil"
	"github.com/moto-nrw/project-phoenix/internal/adapter/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *substitutionsAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	resource := substitutionsAPI.NewResource(svc.Education)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// cleanupSubstitution cleans up a substitution by ID
func cleanupSubstitution(t *testing.T, db *bun.DB, id int64) {
	t.Helper()
	_, _ = db.NewDelete().
		TableExpr("education.group_substitutions").
		Where("id = ?", id).
		Exec(context.Background())
}

// =============================================================================
// LIST TESTS
// =============================================================================

func TestListSubstitutions_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/substitutions", ctx.resource.ListHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

func TestListSubstitutions_WithPagination(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/substitutions", ctx.resource.ListHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions?page=1&page_size=10", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// LIST ACTIVE TESTS
// =============================================================================

func TestListActiveSubstitutions_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/substitutions/active", ctx.resource.ListActiveHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions/active", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

func TestListActiveSubstitutions_WithDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/substitutions/active", ctx.resource.ListActiveHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions/active?date=2026-01-15", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListActiveSubstitutions_BadRequest_InvalidDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/substitutions/active", ctx.resource.ListActiveHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions/active?date=invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET TESTS
// =============================================================================

func TestGetSubstitution_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/substitutions/{id}", ctx.resource.GetHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions/99999", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetSubstitution_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/substitutions/{id}", ctx.resource.GetHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// CREATE TESTS
// =============================================================================

func TestCreateSubstitution_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test fixtures
	staff := testpkg.CreateTestStaff(t, ctx.db, "Substitute", "Teacher")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Get a group ID from the database
	var groupID int64
	err := ctx.db.NewSelect().
		TableExpr("education.groups").
		Column("id").
		Limit(1).
		Scan(context.Background(), &groupID)
	if err != nil || groupID == 0 {
		t.Skip("No groups found in database")
	}

	router := chi.NewRouter()
	router.Post("/substitutions", ctx.resource.CreateHandler())

	// Use future dates to avoid backdating error
	startDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	endDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")

	body := map[string]interface{}{
		"group_id":            groupID,
		"substitute_staff_id": staff.ID,
		"start_date":          startDate,
		"end_date":            endDate,
		"reason":              "Test substitution",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.NotZero(t, data["id"])

	// Cleanup
	if id, ok := data["id"].(float64); ok {
		cleanupSubstitution(t, ctx.db, int64(id))
	}
}

func TestCreateSubstitution_BadRequest_MissingGroupID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/substitutions", ctx.resource.CreateHandler())

	body := map[string]interface{}{
		"substitute_staff_id": 1,
		"start_date":          "2026-01-15",
		"end_date":            "2026-01-22",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateSubstitution_BadRequest_MissingSubstituteStaffID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/substitutions", ctx.resource.CreateHandler())

	body := map[string]interface{}{
		"group_id":   1,
		"start_date": "2026-01-15",
		"end_date":   "2026-01-22",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateSubstitution_BadRequest_InvalidStartDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/substitutions", ctx.resource.CreateHandler())

	body := map[string]interface{}{
		"group_id":            1,
		"substitute_staff_id": 1,
		"start_date":          "invalid-date",
		"end_date":            "2026-01-22",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateSubstitution_BadRequest_InvalidEndDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/substitutions", ctx.resource.CreateHandler())

	startDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	body := map[string]interface{}{
		"group_id":            1,
		"substitute_staff_id": 1,
		"start_date":          startDate,
		"end_date":            "invalid-date",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateSubstitution_BadRequest_StartDateAfterEndDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/substitutions", ctx.resource.CreateHandler())

	// Start date is after end date
	startDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	endDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	body := map[string]interface{}{
		"group_id":            1,
		"substitute_staff_id": 1,
		"start_date":          startDate,
		"end_date":            endDate,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateSubstitution_BadRequest_BackdatedStartDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/substitutions", ctx.resource.CreateHandler())

	// Start date is in the past
	startDate := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	endDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")

	body := map[string]interface{}{
		"group_id":            1,
		"substitute_staff_id": 1,
		"start_date":          startDate,
		"end_date":            endDate,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateSubstitution_BadRequest_InvalidJSON(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/substitutions", ctx.resource.CreateHandler())

	// Create request with invalid JSON (nil body gets JSON encoded to "null")
	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	// With nil body, the JSON decoder gets "null" which decodes to empty struct
	// This results in missing required fields (group_id = 0)
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// UPDATE TESTS
// =============================================================================

func TestUpdateSubstitution_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/substitutions/{id}", ctx.resource.UpdateHandler())

	// Update handler decodes directly into GroupSubstitution model
	// which expects RFC3339 format for time.Time fields
	startDate := time.Now().AddDate(0, 0, 1).Format(time.RFC3339)
	endDate := time.Now().AddDate(0, 0, 7).Format(time.RFC3339)

	body := map[string]interface{}{
		"group_id":            1,
		"substitute_staff_id": 1,
		"start_date":          startDate,
		"end_date":            endDate,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/substitutions/99999", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdateSubstitution_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/substitutions/{id}", ctx.resource.UpdateHandler())

	// Update handler decodes directly into GroupSubstitution model
	// which expects RFC3339 format for time.Time fields
	startDate := time.Now().AddDate(0, 0, 1).Format(time.RFC3339)
	endDate := time.Now().AddDate(0, 0, 7).Format(time.RFC3339)

	body := map[string]interface{}{
		"group_id":            1,
		"substitute_staff_id": 1,
		"start_date":          startDate,
		"end_date":            endDate,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/substitutions/invalid", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DELETE TESTS
// =============================================================================

func TestDeleteSubstitution_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/substitutions/{id}", ctx.resource.DeleteHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/substitutions/99999", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestDeleteSubstitution_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/substitutions/{id}", ctx.resource.DeleteHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/substitutions/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// FULL CRUD WORKFLOW TEST
// =============================================================================

func TestSubstitutionCRUDWorkflow(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test fixtures
	staff := testpkg.CreateTestStaff(t, ctx.db, "CRUD", "Test")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Get a group ID from the database
	var groupID int64
	err := ctx.db.NewSelect().
		TableExpr("education.groups").
		Column("id").
		Limit(1).
		Scan(context.Background(), &groupID)
	if err != nil || groupID == 0 {
		t.Skip("No groups found in database")
	}

	router := chi.NewRouter()
	router.Post("/substitutions", ctx.resource.CreateHandler())
	router.Get("/substitutions/{id}", ctx.resource.GetHandler())
	router.Delete("/substitutions/{id}", ctx.resource.DeleteHandler())

	// Step 1: Create
	startDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	endDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")

	createBody := map[string]interface{}{
		"group_id":            groupID,
		"substitute_staff_id": staff.ID,
		"start_date":          startDate,
		"end_date":            endDate,
		"reason":              fmt.Sprintf("CRUD test %d", time.Now().UnixNano()),
	}

	createReq := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", createBody,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)
	createRR := testutil.ExecuteRequest(router, createReq)
	testutil.AssertSuccessResponse(t, createRR, http.StatusCreated)

	createResponse := testutil.ParseJSONResponse(t, createRR.Body.Bytes())
	createData, ok := createResponse["data"].(map[string]interface{})
	require.True(t, ok)
	subID := int64(createData["id"].(float64))
	defer cleanupSubstitution(t, ctx.db, subID)

	// Step 2: Get
	getReq := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/substitutions/%d", subID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)
	getRR := testutil.ExecuteRequest(router, getReq)
	testutil.AssertSuccessResponse(t, getRR, http.StatusOK)

	getResponse := testutil.ParseJSONResponse(t, getRR.Body.Bytes())
	getData, ok := getResponse["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(subID), getData["id"])
	assert.Equal(t, float64(groupID), getData["group_id"])
	assert.Equal(t, float64(staff.ID), getData["substitute_staff_id"])

	// Step 3: Delete
	deleteReq := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/substitutions/%d", subID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)
	deleteRR := testutil.ExecuteRequest(router, deleteReq)
	assert.Equal(t, http.StatusNoContent, deleteRR.Code)

	// Step 4: Verify deleted
	verifyReq := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/substitutions/%d", subID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)
	verifyRR := testutil.ExecuteRequest(router, verifyReq)
	testutil.AssertNotFound(t, verifyRR)
}
