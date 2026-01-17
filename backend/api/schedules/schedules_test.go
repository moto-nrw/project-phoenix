// Package schedules_test tests the schedules API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package schedules_test

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

	schedulesAPI "github.com/moto-nrw/project-phoenix/api/schedules"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *schedulesAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	resource := schedulesAPI.NewResource(svc.Schedule)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// cleanupDateframe cleans up a dateframe by ID
func cleanupDateframe(t *testing.T, db *bun.DB, id int64) {
	t.Helper()
	_, _ = db.NewDelete().
		TableExpr("schedule.dateframes").
		Where("id = ?", id).
		Exec(context.Background())
}

// cleanupTimeframe cleans up a timeframe by ID
func cleanupTimeframe(t *testing.T, db *bun.DB, id int64) {
	t.Helper()
	_, _ = db.NewDelete().
		TableExpr("schedule.timeframes").
		Where("id = ?", id).
		Exec(context.Background())
}

// cleanupRecurrenceRule cleans up a recurrence rule by ID
func cleanupRecurrenceRule(t *testing.T, db *bun.DB, id int64) {
	t.Helper()
	_, _ = db.NewDelete().
		TableExpr("schedule.recurrence_rules").
		Where("id = ?", id).
		Exec(context.Background())
}

// =============================================================================
// CURRENT DATEFRAME TESTS
// =============================================================================

func TestGetCurrentDateframe_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a dateframe that spans today
	today := time.Now()
	startDate := today.AddDate(0, 0, -7)  // 7 days ago
	endDate := today.AddDate(0, 0, 30)    // 30 days from now

	// Insert the dateframe directly
	dateframe := &schedule.Dateframe{
		StartDate: startDate,
		EndDate:   endDate,
		Name:      fmt.Sprintf("Current Dateframe %d", time.Now().UnixNano()),
	}

	_, err := ctx.db.NewInsert().
		Model(dateframe).
		ModelTableExpr("schedule.dateframes").
		Exec(context.Background())
	require.NoError(t, err)
	defer cleanupDateframe(t, ctx.db, dateframe.ID)

	router := chi.NewRouter()
	router.Get("/current-dateframe", ctx.resource.GetCurrentDateframeHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/current-dateframe", nil,
		testutil.WithPermissions("schedules:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.NotZero(t, data["id"])
}

func TestGetCurrentDateframe_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Ensure no dateframes exist that span today by querying all
	// and deleting any current ones (cleanup)
	today := time.Now()
	todayStr := today.Format("2006-01-02")

	// Delete any dateframes that overlap with today
	_, _ = ctx.db.NewDelete().
		TableExpr("schedule.dateframes").
		Where("start_date <= ? AND end_date >= ?", todayStr, todayStr).
		Exec(context.Background())

	router := chi.NewRouter()
	router.Get("/current-dateframe", ctx.resource.GetCurrentDateframeHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/current-dateframe", nil,
		testutil.WithPermissions("schedules:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	// When no current dateframe exists, should return 404
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// DATEFRAME DATE PARSING TESTS
// =============================================================================

func TestCreateDateframe_InvalidDateFormat(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/dateframes", ctx.resource.CreateDateframeHandler())

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

			req := testutil.NewAuthenticatedRequest(t, "POST", "/dateframes", body,
				testutil.WithClaims(testutil.DefaultTestClaims()),
			)

			rr := testutil.ExecuteRequest(router, req)

			testutil.AssertBadRequest(t, rr)
		})
	}
}

func TestCreateDateframe_EndBeforeStart(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/dateframes", ctx.resource.CreateDateframeHandler())

	body := map[string]interface{}{
		"start_date": "2026-03-01",
		"end_date":   "2026-02-01", // End before start
		"name":       "Invalid Dateframe",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/dateframes", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should fail validation - end date before start date
	// Note: API currently returns 500 for service-level validation errors
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

// =============================================================================
// TIMEFRAME TIME PARSING TESTS
// =============================================================================

func TestCreateTimeframe_InvalidTimeFormat(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/timeframes", ctx.resource.CreateTimeframeHandler())

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

			req := testutil.NewAuthenticatedRequest(t, "POST", "/timeframes", body,
				testutil.WithClaims(testutil.DefaultTestClaims()),
			)

			rr := testutil.ExecuteRequest(router, req)

			testutil.AssertBadRequest(t, rr)
		})
	}
}

func TestCreateTimeframe_EndBeforeStart(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/timeframes", ctx.resource.CreateTimeframeHandler())

	endTime := "2026-01-14T07:00:00Z" // Before start time
	body := map[string]interface{}{
		"start_time": "2026-01-14T08:00:00Z",
		"end_time":   endTime,
		"is_active":  true,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/timeframes", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should fail validation - end time before start time
	// Note: API currently returns 500 for service-level validation errors
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

// =============================================================================
// ROUTER TESTS
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()
	assert.NotNil(t, router, "Router should not be nil")
}

// =============================================================================
// DATEFRAME LIST TESTS
// =============================================================================

func TestListDateframes_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/dateframes", ctx.resource.ListDateframesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/dateframes", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

func TestListDateframes_WithNameFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/dateframes", ctx.resource.ListDateframesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/dateframes?name=test", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// DATEFRAME GET TESTS
// =============================================================================

func TestGetDateframe_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/dateframes/{id}", ctx.resource.GetDateframeHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/dateframes/99999", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetDateframe_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/dateframes/{id}", ctx.resource.GetDateframeHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/dateframes/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DATEFRAME CREATE TESTS
// =============================================================================

func TestCreateDateframe_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/dateframes", ctx.resource.CreateDateframeHandler())

	body := map[string]interface{}{
		"start_date":  "2026-02-01",
		"end_date":    "2026-02-28",
		"name":        fmt.Sprintf("Test Dateframe %d", time.Now().UnixNano()),
		"description": "Test description",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/dateframes", body,
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
		cleanupDateframe(t, ctx.db, int64(id))
	}
}

func TestCreateDateframe_BadRequest_MissingStartDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/dateframes", ctx.resource.CreateDateframeHandler())

	body := map[string]interface{}{
		"end_date": "2026-02-28",
		"name":     "Test Dateframe",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/dateframes", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateDateframe_BadRequest_MissingEndDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/dateframes", ctx.resource.CreateDateframeHandler())

	body := map[string]interface{}{
		"start_date": "2026-02-01",
		"name":       "Test Dateframe",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/dateframes", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateDateframe_BadRequest_InvalidStartDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/dateframes", ctx.resource.CreateDateframeHandler())

	body := map[string]interface{}{
		"start_date": "invalid-date",
		"end_date":   "2026-02-28",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/dateframes", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DATEFRAME UPDATE TESTS
// =============================================================================

func TestUpdateDateframe_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/dateframes/{id}", ctx.resource.UpdateDateframeHandler())

	body := map[string]interface{}{
		"start_date": "2026-02-01",
		"end_date":   "2026-02-28",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/dateframes/99999", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdateDateframe_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/dateframes/{id}", ctx.resource.UpdateDateframeHandler())

	body := map[string]interface{}{
		"start_date": "2026-02-01",
		"end_date":   "2026-02-28",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/dateframes/invalid", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DATEFRAME DELETE TESTS
// =============================================================================

func TestDeleteDateframe_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/dateframes/{id}", ctx.resource.DeleteDateframeHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/dateframes/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DATEFRAME SPECIAL QUERIES
// =============================================================================

func TestGetDateframesByDate_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/dateframes/by-date", ctx.resource.GetDateframesByDateHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/dateframes/by-date?date=2026-01-15", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDateframesByDate_BadRequest_MissingDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/dateframes/by-date", ctx.resource.GetDateframesByDateHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/dateframes/by-date", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGetOverlappingDateframes_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/dateframes/overlapping", ctx.resource.GetOverlappingDateframesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/dateframes/overlapping?start_date=2026-01-01&end_date=2026-12-31", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetOverlappingDateframes_BadRequest_MissingParams(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/dateframes/overlapping", ctx.resource.GetOverlappingDateframesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/dateframes/overlapping", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// TIMEFRAME LIST TESTS
// =============================================================================

func TestListTimeframes_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/timeframes", ctx.resource.ListTimeframesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/timeframes", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

// =============================================================================
// TIMEFRAME GET TESTS
// =============================================================================

func TestGetTimeframe_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/timeframes/{id}", ctx.resource.GetTimeframeHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/timeframes/99999", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetTimeframe_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/timeframes/{id}", ctx.resource.GetTimeframeHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/timeframes/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// TIMEFRAME CREATE TESTS
// =============================================================================

func TestCreateTimeframe_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/timeframes", ctx.resource.CreateTimeframeHandler())

	endTime := "2026-01-14T17:00:00Z"
	body := map[string]interface{}{
		"start_time":  "2026-01-14T08:00:00Z",
		"end_time":    endTime,
		"is_active":   true,
		"description": "Test timeframe",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/timeframes", body,
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
		cleanupTimeframe(t, ctx.db, int64(id))
	}
}

func TestCreateTimeframe_BadRequest_MissingStartTime(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/timeframes", ctx.resource.CreateTimeframeHandler())

	body := map[string]interface{}{
		"end_time": "2026-01-14T17:00:00Z",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/timeframes", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateTimeframe_BadRequest_InvalidStartTime(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/timeframes", ctx.resource.CreateTimeframeHandler())

	body := map[string]interface{}{
		"start_time": "invalid-time",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/timeframes", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// TIMEFRAME UPDATE TESTS
// =============================================================================

func TestUpdateTimeframe_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/timeframes/{id}", ctx.resource.UpdateTimeframeHandler())

	body := map[string]interface{}{
		"start_time": "2026-01-14T08:00:00Z",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/timeframes/99999", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdateTimeframe_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/timeframes/{id}", ctx.resource.UpdateTimeframeHandler())

	body := map[string]interface{}{
		"start_time": "2026-01-14T08:00:00Z",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/timeframes/invalid", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// TIMEFRAME DELETE TESTS
// =============================================================================

func TestDeleteTimeframe_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/timeframes/{id}", ctx.resource.DeleteTimeframeHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/timeframes/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// TIMEFRAME SPECIAL QUERIES
// =============================================================================

func TestGetActiveTimeframes_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/timeframes/active", ctx.resource.GetActiveTimeframesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/timeframes/active", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetTimeframesByRange_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/timeframes/by-range", ctx.resource.GetTimeframesByRangeHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/timeframes/by-range?start_time=2026-01-01T00:00:00Z&end_time=2026-12-31T23:59:59Z", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetTimeframesByRange_BadRequest_MissingParams(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/timeframes/by-range", ctx.resource.GetTimeframesByRangeHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/timeframes/by-range", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// RECURRENCE RULE LIST TESTS
// =============================================================================

func TestListRecurrenceRules_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/recurrence-rules", ctx.resource.ListRecurrenceRulesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/recurrence-rules", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

func TestListRecurrenceRules_WithFrequencyFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/recurrence-rules", ctx.resource.ListRecurrenceRulesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/recurrence-rules?frequency=weekly", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// RECURRENCE RULE GET TESTS
// =============================================================================

func TestGetRecurrenceRule_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/recurrence-rules/{id}", ctx.resource.GetRecurrenceRuleHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/recurrence-rules/99999", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetRecurrenceRule_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/recurrence-rules/{id}", ctx.resource.GetRecurrenceRuleHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/recurrence-rules/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// RECURRENCE RULE CREATE TESTS
// =============================================================================

func TestCreateRecurrenceRule_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/recurrence-rules", ctx.resource.CreateRecurrenceRuleHandler())

	body := map[string]interface{}{
		"frequency":      schedule.FrequencyWeekly,
		"interval_count": 1,
		"weekdays":       []string{"MON", "WED", "FRI"},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/recurrence-rules", body,
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
		cleanupRecurrenceRule(t, ctx.db, int64(id))
	}
}

func TestCreateRecurrenceRule_BadRequest_MissingFrequency(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/recurrence-rules", ctx.resource.CreateRecurrenceRuleHandler())

	body := map[string]interface{}{
		"interval_count": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/recurrence-rules", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateRecurrenceRule_BadRequest_InvalidFrequency(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/recurrence-rules", ctx.resource.CreateRecurrenceRuleHandler())

	body := map[string]interface{}{
		"frequency":      "invalid",
		"interval_count": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/recurrence-rules", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// RECURRENCE RULE UPDATE TESTS
// =============================================================================

func TestUpdateRecurrenceRule_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/recurrence-rules/{id}", ctx.resource.UpdateRecurrenceRuleHandler())

	body := map[string]interface{}{
		"frequency":      schedule.FrequencyDaily,
		"interval_count": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/recurrence-rules/99999", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdateRecurrenceRule_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/recurrence-rules/{id}", ctx.resource.UpdateRecurrenceRuleHandler())

	body := map[string]interface{}{
		"frequency":      schedule.FrequencyDaily,
		"interval_count": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/recurrence-rules/invalid", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// RECURRENCE RULE DELETE TESTS
// =============================================================================

func TestDeleteRecurrenceRule_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/recurrence-rules/{id}", ctx.resource.DeleteRecurrenceRuleHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/recurrence-rules/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// RECURRENCE RULE SPECIAL QUERIES
// =============================================================================

func TestGetRecurrenceRulesByFrequency_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/recurrence-rules/by-frequency", ctx.resource.GetRecurrenceRulesByFrequencyHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/recurrence-rules/by-frequency?frequency=weekly", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetRecurrenceRulesByFrequency_BadRequest_MissingFrequency(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/recurrence-rules/by-frequency", ctx.resource.GetRecurrenceRulesByFrequencyHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/recurrence-rules/by-frequency", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGetRecurrenceRulesByWeekday_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/recurrence-rules/by-weekday", ctx.resource.GetRecurrenceRulesByWeekdayHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/recurrence-rules/by-weekday?weekday=MO", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetRecurrenceRulesByWeekday_BadRequest_MissingWeekday(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/recurrence-rules/by-weekday", ctx.resource.GetRecurrenceRulesByWeekdayHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/recurrence-rules/by-weekday", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GENERATE EVENTS TESTS
// =============================================================================

func TestGenerateEvents_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/recurrence-rules/{id}/generate-events", ctx.resource.GenerateEventsHandler())

	body := map[string]interface{}{
		"start_date": "2026-01-01",
		"end_date":   "2026-12-31",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/recurrence-rules/invalid/generate-events", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGenerateEvents_BadRequest_MissingDates(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/recurrence-rules/{id}/generate-events", ctx.resource.GenerateEventsHandler())

	body := map[string]interface{}{}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/recurrence-rules/1/generate-events", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// CHECK CONFLICT TESTS
// =============================================================================

func TestCheckConflict_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/check-conflict", ctx.resource.CheckConflictHandler())

	body := map[string]interface{}{
		"start_time": "2026-01-14T09:00:00Z",
		"end_time":   "2026-01-14T10:00:00Z",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/check-conflict", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	_, hasConflictExists := data["has_conflict"]
	assert.True(t, hasConflictExists, "Expected has_conflict field in response")
}

func TestCheckConflict_BadRequest_MissingTimes(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/check-conflict", ctx.resource.CheckConflictHandler())

	body := map[string]interface{}{}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/check-conflict", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCheckConflict_BadRequest_InvalidTime(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/check-conflict", ctx.resource.CheckConflictHandler())

	body := map[string]interface{}{
		"start_time": "invalid-time",
		"end_time":   "2026-01-14T10:00:00Z",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/check-conflict", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// FIND AVAILABLE SLOTS TESTS
// =============================================================================

func TestFindAvailableSlots_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/find-available-slots", ctx.resource.FindAvailableSlotsHandler())

	body := map[string]interface{}{
		"start_date": "2026-01-01",
		"end_date":   "2026-01-31",
		"duration":   60,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/find-available-slots", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	_, countExists := data["count"]
	assert.True(t, countExists, "Expected count field in response")
}

func TestFindAvailableSlots_BadRequest_MissingDuration(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/find-available-slots", ctx.resource.FindAvailableSlotsHandler())

	body := map[string]interface{}{
		"start_date": "2026-01-01",
		"end_date":   "2026-01-31",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/find-available-slots", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestFindAvailableSlots_BadRequest_InvalidDuration(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/find-available-slots", ctx.resource.FindAvailableSlotsHandler())

	body := map[string]interface{}{
		"start_date": "2026-01-01",
		"end_date":   "2026-01-31",
		"duration":   0, // Invalid: must be >= 1
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/find-available-slots", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}
