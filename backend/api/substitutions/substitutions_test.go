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
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults before any test (and before setupTestContext
// constructs a Resource via jwt.MustNewTokenAuth). CI runs without a .env so
// AUTH_JWT_SECRET is unset; without a secret jwx refuses HMAC signing.
func init() {
	testutil.SeedTestJWTConfig()
}

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *substitutionsAPI.Resource
	router   chi.Router
}

// setupTestContext initializes test database, services, and resource. The
// router mounts the production Resource.Router() at /substitutions, so requests
// run through the real middleware chain (Verifier → Authenticator →
// TenantMiddleware → RequiresPermission → TenantTxMiddleware).
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	resource := substitutionsAPI.NewResource(svc.Education, db)

	router := chi.NewRouter()
	router.Mount("/substitutions", resource.Router())

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
		router:   router,
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
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

func TestListSubstitutions_WithPagination(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions?page=1&page_size=10", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// LIST ACTIVE TESTS
// =============================================================================

func TestListActiveSubstitutions_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions/active", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

func TestListActiveSubstitutions_WithDate(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions/active?date=2026-01-15", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListActiveSubstitutions_BadRequest_InvalidDate(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions/active?date=invalid", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET TESTS
// =============================================================================

func TestGetSubstitution_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions/99999", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetSubstitution_InvalidID(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/substitutions/invalid", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// CREATE TESTS
// =============================================================================

func TestCreateSubstitution_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Create test fixtures
	staff := testpkg.CreateTestStaff(t, ctx.db, "Substitute", "Teacher")

	// Create a group fixture
	group := testpkg.CreateTestEducationGroup(t, ctx.db, "SubstitutionCreate")

	// Use future dates to avoid backdating error
	startDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	endDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")

	body := map[string]interface{}{
		"group_id":            group.ID,
		"substitute_staff_id": staff.ID,
		"start_date":          startDate,
		"end_date":            endDate,
		"reason":              "Test substitution",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

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
	t.Parallel()
	ctx := setupTestContext(t)

	body := map[string]interface{}{
		"substitute_staff_id": 1,
		"start_date":          "2026-01-15",
		"end_date":            "2026-01-22",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateSubstitution_BadRequest_MissingSubstituteStaffID(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	body := map[string]interface{}{
		"group_id":   1,
		"start_date": "2026-01-15",
		"end_date":   "2026-01-22",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateSubstitution_BadRequest_InvalidStartDate(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	body := map[string]interface{}{
		"group_id":            1,
		"substitute_staff_id": 1,
		"start_date":          "invalid-date",
		"end_date":            "2026-01-22",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateSubstitution_BadRequest_InvalidEndDate(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	startDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	body := map[string]interface{}{
		"group_id":            1,
		"substitute_staff_id": 1,
		"start_date":          startDate,
		"end_date":            "invalid-date",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateSubstitution_BadRequest_StartDateAfterEndDate(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

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
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateSubstitution_BadRequest_BackdatedStartDate(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

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
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateSubstitution_BadRequest_InvalidJSON(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Create request with invalid JSON (nil body gets JSON encoded to "null")
	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	// With nil body, the JSON decoder gets "null" which decodes to empty struct
	// This results in missing required fields (group_id = 0)
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// UPDATE TESTS
// =============================================================================

func TestUpdateSubstitution_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Update handler decodes directly into GroupSubstitution model
	// which expects "YYYY-MM-DD" format for timezone.Date fields
	startDate := timezone.TodayDate().AddDays(1).String()
	endDate := timezone.TodayDate().AddDays(7).String()

	body := map[string]interface{}{
		"group_id":            1,
		"substitute_staff_id": 1,
		"start_date":          startDate,
		"end_date":            endDate,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/substitutions/99999", body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdateSubstitution_InvalidID(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Update handler decodes directly into GroupSubstitution model
	// which expects "YYYY-MM-DD" format for timezone.Date fields
	startDate := timezone.TodayDate().AddDays(1).String()
	endDate := timezone.TodayDate().AddDays(7).String()

	body := map[string]interface{}{
		"group_id":            1,
		"substitute_staff_id": 1,
		"start_date":          startDate,
		"end_date":            endDate,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/substitutions/invalid", body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DELETE TESTS
// =============================================================================

func TestDeleteSubstitution_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/substitutions/99999", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

func TestDeleteSubstitution_InvalidID(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/substitutions/invalid", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// FULL CRUD WORKFLOW TEST
// =============================================================================

func TestSubstitutionCRUDWorkflow(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Create test fixtures
	staff := testpkg.CreateTestStaff(t, ctx.db, "CRUD", "Test")

	// Create a group fixture
	group := testpkg.CreateTestEducationGroup(t, ctx.db, "SubstitutionCRUD")

	// Step 1: Create
	startDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	endDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")

	createBody := map[string]interface{}{
		"group_id":            group.ID,
		"substitute_staff_id": staff.ID,
		"start_date":          startDate,
		"end_date":            endDate,
		"reason":              fmt.Sprintf("CRUD test %d", time.Now().UnixNano()),
	}

	createReq := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", createBody,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)
	createRR := testutil.ExecuteRequest(ctx.router, createReq)
	testutil.AssertSuccessResponse(t, createRR, http.StatusCreated)

	createResponse := testutil.ParseJSONResponse(t, createRR.Body.Bytes())
	createData, ok := createResponse["data"].(map[string]interface{})
	require.True(t, ok)
	subID := int64(createData["id"].(float64))
	defer cleanupSubstitution(t, ctx.db, subID)

	// Step 2: Get
	getReq := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/substitutions/%d", subID), nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)
	getRR := testutil.ExecuteRequest(ctx.router, getReq)
	testutil.AssertSuccessResponse(t, getRR, http.StatusOK)

	getResponse := testutil.ParseJSONResponse(t, getRR.Body.Bytes())
	getData, ok := getResponse["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(subID), getData["id"])
	assert.Equal(t, float64(group.ID), getData["group_id"])
	assert.Equal(t, float64(staff.ID), getData["substitute_staff_id"])

	// Step 3: Delete
	deleteReq := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/substitutions/%d", subID), nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)
	deleteRR := testutil.ExecuteRequest(ctx.router, deleteReq)
	assert.Equal(t, http.StatusNoContent, deleteRR.Code)

	// Step 4: Verify deleted
	verifyReq := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/substitutions/%d", subID), nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)
	verifyRR := testutil.ExecuteRequest(ctx.router, verifyReq)
	testutil.AssertNotFound(t, verifyRR)
}
