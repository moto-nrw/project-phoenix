// Package schedules_test tests the schedules API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package httpintegration_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	schedulesAPI "github.com/moto-nrw/project-phoenix/modules/timetable/compose/httpadapter"
)

// init seeds JWT viper defaults before any test (and before setupSchedulesRoute
// constructs a SchedulesResource via jwt.MustNewTokenAuth). CI runs without a .env so
// AUTH_JWT_SECRET is unset; without a secret jwx refuses HMAC signing.
func init() {
	testutil.SeedTestJWTConfig()
}

// schedulesTestContext holds shared test dependencies.
type schedulesTestContext struct {
	db       *bun.DB
	resource *schedulesAPI.SchedulesResource
	router   chi.Router
}

// setupSchedulesRoute initializes test database, services, and resource. The
// router serves the resource through the production middleware chain (Verifier →
// Authenticator → TenantMiddleware → RequiresPermission → TenantTxMiddleware)
// exactly as the real server does.
func setupSchedulesRoute(t *testing.T) *schedulesTestContext {
	t.Helper()

	db, svc := testutil.SetupScheduleModule(t)

	resource := schedulesAPI.NewSchedulesResource(svc.Schedule, db)

	return &schedulesTestContext{
		db:       db,
		resource: resource,
		router:   resource.Router(),
	}
}

// =============================================================================
// CURRENT DATEFRAME TESTS
// =============================================================================

func TestSchedulesGetCurrentDateframe_Success(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	ctx := setupSchedulesRoute(t)

	// Create a dateframe that spans today
	today := time.Now()
	startDate := today.AddDate(0, 0, -7) // 7 days ago
	endDate := today.AddDate(0, 0, 30)   // 30 days from now

	// Insert the dateframe directly
	dateframe := &schedule.Dateframe{
		StartDate: startDate,
		EndDate:   endDate,
		Name:      fmt.Sprintf("Current Dateframe %d", time.Now().UnixNano()),
	}

	dateframe.SetTenantID(testpkg.Tenant(t))

	_, err := ctx.db.NewInsert().
		Model(dateframe).
		ModelTableExpr("schedule.dateframes").
		Exec(context.Background())
	require.NoError(t, err)

	req := testutil.NewRequest("GET", "/current-dateframe", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.NotZero(t, data["id"])
}

func TestSchedulesGetCurrentDateframe_NotFound(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	ctx := setupSchedulesRoute(t)

	// Ensure no dateframes exist that span today by querying all
	// and deleting any current ones (cleanup)
	today := time.Now()
	todayStr := today.Format("2006-01-02")

	// Delete any dateframes that overlap with today
	_, _ = ctx.db.NewDelete().
		TableExpr("schedule.dateframes").
		Where("start_date <= ? AND end_date >= ?", todayStr, todayStr).
		Exec(context.Background())

	req := testutil.NewRequest("GET", "/current-dateframe", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	// When no current dateframe exists, should return 404
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// DATEFRAME DATE PARSING TESTS
// =============================================================================

func TestSchedulesCreateDateframe_InvalidDateFormat(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	testCases := []struct {
		name      string
		startDate string
		endDate   string
	}{
		{"wrong separator", "2026/02/01", "2026/02/28"},
		{"month out of range", "2026-13-01", "2026-02-28"},
		{"day out of range", "2026-02-32", "2026-02-28"},
		{"letters in date", "2026-0a-01", "2026-02-28"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]interface{}{
				"start_date": tc.startDate,
				"end_date":   tc.endDate,
			}

			req := testutil.NewAuthenticatedRequest(t, "POST", "/dateframes", body)

			rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

			testutil.AssertBadRequest(t, rr)
		})
	}
}

func TestSchedulesCreateDateframe_EndBeforeStart(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_date": "2026-03-01",
		"end_date":   "2026-02-01", // End before start
		"name":       "Invalid Dateframe",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/dateframes", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	// Should fail validation - end date before start date
	// Note: API currently returns 500 for service-level validation errors
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

// =============================================================================
// TIMEFRAME TIME PARSING TESTS
// =============================================================================

func TestSchedulesCreateTimeframe_InvalidTimeFormat(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	testCases := []struct {
		name      string
		startTime string
	}{
		{"not a time", "not-a-time"},
		{"missing seconds", "2026-01-14T08:00"},
		{"wrong format", "14/01/2026 08:00:00"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]interface{}{
				"start_time": tc.startTime,
			}

			req := testutil.NewAuthenticatedRequest(t, "POST", "/timeframes", body)

			rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

			testutil.AssertBadRequest(t, rr)
		})
	}
}

func TestSchedulesCreateTimeframe_EndBeforeStart(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	endTime := "2026-01-14T07:00:00Z" // Before start time
	body := map[string]interface{}{
		"start_time": "2026-01-14T08:00:00Z",
		"end_time":   endTime,
		"is_active":  true,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/timeframes", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	// Should fail validation - end time before start time
	// Note: API currently returns 500 for service-level validation errors
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

// =============================================================================
// ROUTER TESTS
// =============================================================================

func TestSchedulesRouter_ReturnsValidRouter(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	router := ctx.resource.Router()
	assert.NotNil(t, router, "Router should not be nil")
}

// =============================================================================
// DATEFRAME LIST TESTS
// =============================================================================

func TestSchedulesListDateframes_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/dateframes", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

func TestSchedulesListDateframes_WithNameFilter(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/dateframes?name=test", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// DATEFRAME GET TESTS
// =============================================================================

func TestSchedulesGetDateframe_NotFound(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/dateframes/99999", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertNotFound(t, rr)
}

func TestSchedulesGetDateframe_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/dateframes/invalid", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DATEFRAME CREATE TESTS
// =============================================================================

func TestSchedulesCreateDateframe_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_date":  "2026-02-01",
		"end_date":    "2026-02-28",
		"name":        fmt.Sprintf("Test Dateframe %d", time.Now().UnixNano()),
		"description": "Test description",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/dateframes", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.NotZero(t, data["id"])

	// Cleanup
}

func TestSchedulesCreateDateframe_BadRequest_MissingStartDate(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"end_date": "2026-02-28",
		"name":     "Test Dateframe",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/dateframes", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

func TestSchedulesCreateDateframe_BadRequest_MissingEndDate(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_date": "2026-02-01",
		"name":       "Test Dateframe",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/dateframes", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

func TestSchedulesCreateDateframe_BadRequest_InvalidStartDate(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_date": "invalid-date",
		"end_date":   "2026-02-28",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/dateframes", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DATEFRAME UPDATE TESTS
// =============================================================================

func TestSchedulesUpdateDateframe_NotFound(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_date": "2026-02-01",
		"end_date":   "2026-02-28",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/dateframes/99999", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertNotFound(t, rr)
}

func TestSchedulesUpdateDateframe_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_date": "2026-02-01",
		"end_date":   "2026-02-28",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/dateframes/invalid", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DATEFRAME DELETE TESTS
// =============================================================================

func TestSchedulesDeleteDateframe_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("DELETE", "/dateframes/invalid", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DATEFRAME SPECIAL QUERIES
// =============================================================================

func TestSchedulesGetDateframesByDate_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/dateframes/by-date?date=2026-01-15", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestSchedulesGetDateframesByDate_BadRequest_MissingDate(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/dateframes/by-date", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

func TestSchedulesGetOverlappingDateframes_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/dateframes/overlapping?start_date=2026-01-01&end_date=2026-12-31", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestSchedulesGetOverlappingDateframes_BadRequest_MissingParams(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/dateframes/overlapping", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// TIMEFRAME LIST TESTS
// =============================================================================

func TestSchedulesListTimeframes_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/timeframes", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

// =============================================================================
// TIMEFRAME GET TESTS
// =============================================================================

func TestSchedulesGetTimeframe_NotFound(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/timeframes/99999", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertNotFound(t, rr)
}

func TestSchedulesGetTimeframe_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/timeframes/invalid", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// TIMEFRAME CREATE TESTS
// =============================================================================

func TestSchedulesCreateTimeframe_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	endTime := "2026-01-14T17:00:00Z"
	body := map[string]interface{}{
		"start_time":  "2026-01-14T08:00:00Z",
		"end_time":    endTime,
		"is_active":   true,
		"description": "Test timeframe",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/timeframes", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.NotZero(t, data["id"])

	// Cleanup
}

func TestSchedulesCreateTimeframe_BadRequest_MissingStartTime(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"end_time": "2026-01-14T17:00:00Z",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/timeframes", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

func TestSchedulesCreateTimeframe_BadRequest_InvalidStartTime(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_time": "invalid-time",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/timeframes", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// TIMEFRAME UPDATE TESTS
// =============================================================================

func TestSchedulesUpdateTimeframe_NotFound(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_time": "2026-01-14T08:00:00Z",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/timeframes/99999", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertNotFound(t, rr)
}

func TestSchedulesUpdateTimeframe_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_time": "2026-01-14T08:00:00Z",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/timeframes/invalid", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// TIMEFRAME DELETE TESTS
// =============================================================================

func TestSchedulesDeleteTimeframe_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("DELETE", "/timeframes/invalid", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// TIMEFRAME SPECIAL QUERIES
// =============================================================================

func TestSchedulesGetActiveTimeframes_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/timeframes/active", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestSchedulesGetTimeframesByRange_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/timeframes/by-range?start_time=2026-01-01T00:00:00Z&end_time=2026-12-31T23:59:59Z", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestSchedulesGetTimeframesByRange_BadRequest_MissingParams(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/timeframes/by-range", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// RECURRENCE RULE LIST TESTS
// =============================================================================

func TestSchedulesListRecurrenceRules_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/recurrence-rules", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

func TestSchedulesListRecurrenceRules_WithFrequencyFilter(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/recurrence-rules?frequency=weekly", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// RECURRENCE RULE GET TESTS
// =============================================================================

func TestSchedulesGetRecurrenceRule_NotFound(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/recurrence-rules/99999", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertNotFound(t, rr)
}

func TestSchedulesGetRecurrenceRule_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/recurrence-rules/invalid", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// RECURRENCE RULE CREATE TESTS
// =============================================================================

func TestSchedulesCreateRecurrenceRule_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"frequency":      schedule.FrequencyWeekly,
		"interval_count": 1,
		"weekdays":       []string{"MON", "WED", "FRI"},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/recurrence-rules", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.NotZero(t, data["id"])

	// Cleanup
}

func TestSchedulesCreateRecurrenceRule_BadRequest_MissingFrequency(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"interval_count": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/recurrence-rules", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

func TestSchedulesCreateRecurrenceRule_BadRequest_InvalidFrequency(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"frequency":      "invalid",
		"interval_count": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/recurrence-rules", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// RECURRENCE RULE UPDATE TESTS
// =============================================================================

func TestSchedulesUpdateRecurrenceRule_NotFound(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"frequency":      schedule.FrequencyDaily,
		"interval_count": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/recurrence-rules/99999", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertNotFound(t, rr)
}

func TestSchedulesUpdateRecurrenceRule_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"frequency":      schedule.FrequencyDaily,
		"interval_count": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/recurrence-rules/invalid", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// RECURRENCE RULE DELETE TESTS
// =============================================================================

func TestSchedulesDeleteRecurrenceRule_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("DELETE", "/recurrence-rules/invalid", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// RECURRENCE RULE SPECIAL QUERIES
// =============================================================================

func TestSchedulesGetRecurrenceRulesByFrequency_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/recurrence-rules/by-frequency?frequency=weekly", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestSchedulesGetRecurrenceRulesByFrequency_BadRequest_MissingFrequency(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/recurrence-rules/by-frequency", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

func TestSchedulesGetRecurrenceRulesByWeekday_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/recurrence-rules/by-weekday?weekday=MO", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestSchedulesGetRecurrenceRulesByWeekday_BadRequest_MissingWeekday(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	req := testutil.NewRequest("GET", "/recurrence-rules/by-weekday", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GENERATE EVENTS TESTS
// =============================================================================

func TestSchedulesGenerateEvents_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_date": "2026-01-01",
		"end_date":   "2026-12-31",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/recurrence-rules/invalid/generate-events", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

func TestSchedulesGenerateEvents_BadRequest_MissingDates(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/recurrence-rules/1/generate-events", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// CHECK CONFLICT TESTS
// =============================================================================

func TestSchedulesCheckConflict_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_time": "2026-01-14T09:00:00Z",
		"end_time":   "2026-01-14T10:00:00Z",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/check-conflict", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	_, hasConflictExists := data["has_conflict"]
	assert.True(t, hasConflictExists, "Expected has_conflict field in response")
}

func TestSchedulesCheckConflict_BadRequest_MissingTimes(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/check-conflict", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

func TestSchedulesCheckConflict_BadRequest_InvalidTime(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_time": "invalid-time",
		"end_time":   "2026-01-14T10:00:00Z",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/check-conflict", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// FIND AVAILABLE SLOTS TESTS
// =============================================================================

func TestSchedulesFindAvailableSlots_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_date": "2026-01-01",
		"end_date":   "2026-01-31",
		"duration":   60,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/find-available-slots", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	_, countExists := data["count"]
	assert.True(t, countExists, "Expected count field in response")
}

func TestSchedulesFindAvailableSlots_BadRequest_MissingDuration(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_date": "2026-01-01",
		"end_date":   "2026-01-31",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/find-available-slots", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}

func TestSchedulesFindAvailableSlots_BadRequest_InvalidDuration(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)

	body := map[string]interface{}{
		"start_date": "2026-01-01",
		"end_date":   "2026-01-31",
		"duration":   0, // Invalid: must be >= 1
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/find-available-slots", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertBadRequest(t, rr)
}
