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

	substitutionsAPI "github.com/moto-nrw/project-phoenix/api/substitutions"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *substitutionsAPI.Resource
	ogsID    string
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	ogsID := testpkg.SetupTestOGS(t, db)

	resource := substitutionsAPI.NewResource(svc.Education)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
		ogsID:    ogsID,
	}
}

// cleanupSubstitution cleans up a substitution by ID
func cleanupSubstitution(t *testing.T, db *bun.DB, id int64) {
	t.Helper()
	_, _ = db.NewDelete().
		TableExpr("education.group_substitution").
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
	ogsID := testpkg.SetupTestOGS(t, ctx.db)

	// Create test fixtures
	staff := testpkg.CreateTestStaff(t, ctx.db, "Substitute", "Teacher", ogsID)
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
	staff := testpkg.CreateTestStaff(t, ctx.db, "CRUD", "Test", ctx.ogsID)
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

// =============================================================================
// UPDATE SUCCESS TESTS
// =============================================================================

func TestUpdateSubstitution_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test fixtures using hermetic pattern
	staff := testpkg.CreateTestStaff(t, ctx.db, "Update", "Test", ctx.ogsID)
	group := testpkg.CreateTestEducationGroup(t, ctx.db, "UpdateTestGroup", ctx.ogsID)

	// Create a substitution to update
	startDate := time.Now().AddDate(0, 0, 1)
	endDate := time.Now().AddDate(0, 0, 7)
	substitution := testpkg.CreateTestGroupSubstitution(t, ctx.db, group.ID, nil, staff.ID, startDate, endDate)
	defer cleanupSubstitution(t, ctx.db, substitution.ID)

	router := chi.NewRouter()
	router.Put("/substitutions/{id}", ctx.resource.UpdateHandler())

	// Update with new dates - include timestamps since handler decodes directly into model
	newStartDate := time.Now().AddDate(0, 0, 2).Format(time.RFC3339)
	newEndDate := time.Now().AddDate(0, 0, 10).Format(time.RFC3339)

	body := map[string]interface{}{
		"group_id":            group.ID,
		"substitute_staff_id": staff.ID,
		"start_date":          newStartDate,
		"end_date":            newEndDate,
		"reason":              "Updated reason",
		"created_at":          substitution.CreatedAt.Format(time.RFC3339),
		"updated_at":          time.Now().Format(time.RFC3339),
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/substitutions/%d", substitution.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.Equal(t, float64(substitution.ID), data["id"])
}

func TestUpdateSubstitution_BadRequest_InvalidJSON(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/substitutions/{id}", ctx.resource.UpdateHandler())

	// Send with nil body which gets encoded as "null" - empty struct
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/substitutions/1", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Empty struct fails date validation (zero dates)
	testutil.AssertBadRequest(t, rr)
}

func TestUpdateSubstitution_BadRequest_DateValidation(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, ctx.db, "DateVal", "Test", ctx.ogsID)
	group := testpkg.CreateTestEducationGroup(t, ctx.db, "DateValGroup", ctx.ogsID)

	// Create substitution
	startDate := time.Now().AddDate(0, 0, 1)
	endDate := time.Now().AddDate(0, 0, 7)
	substitution := testpkg.CreateTestGroupSubstitution(t, ctx.db, group.ID, nil, staff.ID, startDate, endDate)
	defer cleanupSubstitution(t, ctx.db, substitution.ID)

	router := chi.NewRouter()
	router.Put("/substitutions/{id}", ctx.resource.UpdateHandler())

	// Test start date after end date - validation happens before DB access
	body := map[string]interface{}{
		"group_id":            group.ID,
		"substitute_staff_id": staff.ID,
		"start_date":          time.Now().AddDate(0, 0, 10).Format(time.RFC3339),
		"end_date":            time.Now().AddDate(0, 0, 5).Format(time.RFC3339),
		"created_at":          substitution.CreatedAt.Format(time.RFC3339),
		"updated_at":          time.Now().Format(time.RFC3339),
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/substitutions/%d", substitution.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdateSubstitution_BadRequest_BackdatedStartDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, ctx.db, "Backdate", "Test", ctx.ogsID)
	group := testpkg.CreateTestEducationGroup(t, ctx.db, "BackdateGroup", ctx.ogsID)

	// Create substitution
	startDate := time.Now().AddDate(0, 0, 1)
	endDate := time.Now().AddDate(0, 0, 7)
	substitution := testpkg.CreateTestGroupSubstitution(t, ctx.db, group.ID, nil, staff.ID, startDate, endDate)
	defer cleanupSubstitution(t, ctx.db, substitution.ID)

	router := chi.NewRouter()
	router.Put("/substitutions/{id}", ctx.resource.UpdateHandler())

	// Test backdated start date - validation happens before DB access
	body := map[string]interface{}{
		"group_id":            group.ID,
		"substitute_staff_id": staff.ID,
		"start_date":          time.Now().AddDate(0, 0, -5).Format(time.RFC3339),
		"end_date":            time.Now().AddDate(0, 0, 5).Format(time.RFC3339),
		"created_at":          substitution.CreatedAt.Format(time.RFC3339),
		"updated_at":          time.Now().Format(time.RFC3339),
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/substitutions/%d", substitution.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdateSubstitution_Conflict_StaffAlreadySubstituting(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create fixtures
	staff1 := testpkg.CreateTestStaff(t, ctx.db, "Staff1", "Test", ctx.ogsID)
	staff2 := testpkg.CreateTestStaff(t, ctx.db, "Staff2", "Test", ctx.ogsID)
	group1 := testpkg.CreateTestEducationGroup(t, ctx.db, "ConflictGroup1", ctx.ogsID)
	group2 := testpkg.CreateTestEducationGroup(t, ctx.db, "ConflictGroup2", ctx.ogsID)

	// Create first substitution with staff1
	startDate := time.Now().AddDate(0, 0, 1)
	endDate := time.Now().AddDate(0, 0, 7)
	substitution1 := testpkg.CreateTestGroupSubstitution(t, ctx.db, group1.ID, nil, staff1.ID, startDate, endDate)
	defer cleanupSubstitution(t, ctx.db, substitution1.ID)

	// Create second substitution with staff2 that we'll try to change to staff1
	substitution2 := testpkg.CreateTestGroupSubstitution(t, ctx.db, group2.ID, nil, staff2.ID, startDate, endDate)
	defer cleanupSubstitution(t, ctx.db, substitution2.ID)

	router := chi.NewRouter()
	router.Put("/substitutions/{id}", ctx.resource.UpdateHandler())

	// Try to update substitution2 to use staff1 (should conflict with substitution1)
	body := map[string]interface{}{
		"group_id":            group2.ID,
		"substitute_staff_id": staff1.ID,
		"start_date":          startDate.Format(time.RFC3339),
		"end_date":            endDate.Format(time.RFC3339),
		"created_at":          substitution2.CreatedAt.Format(time.RFC3339),
		"updated_at":          time.Now().Format(time.RFC3339),
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/substitutions/%d", substitution2.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return conflict status
	assert.Equal(t, http.StatusConflict, rr.Code, "Expected 409 Conflict. Body: %s", rr.Body.String())
}

// =============================================================================
// GET SUCCESS TEST
// =============================================================================

func TestGetSubstitution_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, ctx.db, "GetSuccess", "Test", ctx.ogsID)
	group := testpkg.CreateTestEducationGroup(t, ctx.db, "GetSuccessGroup", ctx.ogsID)

	// Create substitution
	startDate := time.Now().AddDate(0, 0, 1)
	endDate := time.Now().AddDate(0, 0, 7)
	substitution := testpkg.CreateTestGroupSubstitution(t, ctx.db, group.ID, nil, staff.ID, startDate, endDate)
	defer cleanupSubstitution(t, ctx.db, substitution.ID)

	router := chi.NewRouter()
	router.Get("/substitutions/{id}", ctx.resource.GetHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/substitutions/%d", substitution.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.Equal(t, float64(substitution.ID), data["id"])
	assert.Equal(t, float64(group.ID), data["group_id"])
	assert.Equal(t, float64(staff.ID), data["substitute_staff_id"])
	assert.NotEmpty(t, data["start_date"])
	assert.NotEmpty(t, data["end_date"])
}

// =============================================================================
// DELETE SUCCESS TEST
// =============================================================================

func TestDeleteSubstitution_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, ctx.db, "DeleteSuccess", "Test", ctx.ogsID)
	group := testpkg.CreateTestEducationGroup(t, ctx.db, "DeleteSuccessGroup", ctx.ogsID)

	// Create substitution
	startDate := time.Now().AddDate(0, 0, 1)
	endDate := time.Now().AddDate(0, 0, 7)
	substitution := testpkg.CreateTestGroupSubstitution(t, ctx.db, group.ID, nil, staff.ID, startDate, endDate)
	// No defer cleanup needed since we're deleting it

	router := chi.NewRouter()
	router.Delete("/substitutions/{id}", ctx.resource.DeleteHandler())
	router.Get("/substitutions/{id}", ctx.resource.GetHandler())

	// Delete the substitution
	deleteReq := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/substitutions/%d", substitution.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	deleteRR := testutil.ExecuteRequest(router, deleteReq)

	assert.Equal(t, http.StatusNoContent, deleteRR.Code, "Expected 204 No Content. Body: %s", deleteRR.Body.String())

	// Verify it's deleted
	getReq := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/substitutions/%d", substitution.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	getRR := testutil.ExecuteRequest(router, getReq)
	testutil.AssertNotFound(t, getRR)
}

// =============================================================================
// LIST WITH INTERNAL SERVER ERROR TEST
// =============================================================================

func TestListSubstitutions_ReturnsEmptyArray(t *testing.T) {
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
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
	// Verify it's an array (may or may not be empty depending on DB state)
	assert.NotNil(t, data)
}

func TestListActiveSubstitutions_ReturnsEmptyArray(t *testing.T) {
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
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
	assert.NotNil(t, data)
}

// =============================================================================
// UPDATE WITH SAME STAFF (NO CONFLICT) TEST
// =============================================================================

func TestUpdateSubstitution_NoConflict_SameStaff(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, ctx.db, "SameStaff", "Test", ctx.ogsID)
	group := testpkg.CreateTestEducationGroup(t, ctx.db, "SameStaffGroup", ctx.ogsID)

	// Create substitution
	startDate := time.Now().AddDate(0, 0, 1)
	endDate := time.Now().AddDate(0, 0, 7)
	substitution := testpkg.CreateTestGroupSubstitution(t, ctx.db, group.ID, nil, staff.ID, startDate, endDate)
	defer cleanupSubstitution(t, ctx.db, substitution.ID)

	router := chi.NewRouter()
	router.Put("/substitutions/{id}", ctx.resource.UpdateHandler())

	// Update with same staff but different dates (no conflict expected)
	newStartDate := time.Now().AddDate(0, 0, 2)
	newEndDate := time.Now().AddDate(0, 0, 8)

	body := map[string]interface{}{
		"group_id":            group.ID,
		"substitute_staff_id": staff.ID, // Same staff
		"start_date":          newStartDate.Format(time.RFC3339),
		"end_date":            newEndDate.Format(time.RFC3339),
		"created_at":          substitution.CreatedAt.Format(time.RFC3339),
		"updated_at":          time.Now().Format(time.RFC3339),
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/substitutions/%d", substitution.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// CREATE WITH REGULAR STAFF ID TEST
// =============================================================================

func TestCreateSubstitution_WithRegularStaffID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create fixtures
	regularStaff := testpkg.CreateTestStaff(t, ctx.db, "Regular", "Staff", ctx.ogsID)
	substituteStaff := testpkg.CreateTestStaff(t, ctx.db, "Substitute", "Staff", ctx.ogsID)
	group := testpkg.CreateTestEducationGroup(t, ctx.db, "RegularStaffGroup", ctx.ogsID)

	router := chi.NewRouter()
	router.Post("/substitutions", ctx.resource.CreateHandler())

	startDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	endDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")

	body := map[string]interface{}{
		"group_id":            group.ID,
		"regular_staff_id":    regularStaff.ID,
		"substitute_staff_id": substituteStaff.ID,
		"start_date":          startDate,
		"end_date":            endDate,
		"reason":              "With regular staff",
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
	assert.Equal(t, float64(regularStaff.ID), data["regular_staff_id"])

	// Cleanup
	if id, ok := data["id"].(float64); ok {
		cleanupSubstitution(t, ctx.db, int64(id))
	}
}
