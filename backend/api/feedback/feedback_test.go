package feedback_test

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

	feedbackAPI "github.com/moto-nrw/project-phoenix/api/feedback"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/feedback"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *feedbackAPI.Resource
	ogsID    string
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	ogsID := testpkg.SetupTestOGS(t, db)

	resource := feedbackAPI.NewResource(svc.Feedback)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
		ogsID:    ogsID,
	}
}

// =============================================================================
// LIST FEEDBACK TESTS
// =============================================================================

func TestListFeedback_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback", ctx.resource.ListFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListFeedback_WithStudentFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback", ctx.resource.ListFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback?student_id=1", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListFeedback_WithDateFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback", ctx.resource.ListFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback?date=2026-01-14", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListFeedback_WithMensaFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback", ctx.resource.ListFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback?is_mensa=true", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET FEEDBACK BY ID TESTS
// =============================================================================

func TestGetFeedback_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/{id}", ctx.resource.GetFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/999999", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetFeedback_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/{id}", ctx.resource.GetFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/invalid", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET STUDENT FEEDBACK TESTS
// =============================================================================

func TestGetStudentFeedback_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, ctx.db)

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "Student", "1a", ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Get("/feedback/student/{id}", ctx.resource.GetStudentFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/feedback/student/%d", student.ID), nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStudentFeedback_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/student/{id}", ctx.resource.GetStudentFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/student/invalid", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET DATE FEEDBACK TESTS
// =============================================================================

func TestGetDateFeedback_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/date/{date}", ctx.resource.GetDateFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/date/2026-01-14", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDateFeedback_InvalidDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/date/{date}", ctx.resource.GetDateFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/date/invalid-date", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET MENSA FEEDBACK TESTS
// =============================================================================

func TestGetMensaFeedback_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/mensa", ctx.resource.GetMensaFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/mensa", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetMensaFeedback_WithFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/mensa", ctx.resource.GetMensaFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/mensa?is_mensa=false", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET DATE RANGE FEEDBACK TESTS
// =============================================================================

func TestGetDateRangeFeedback_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/date-range", ctx.resource.GetDateRangeFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/date-range?start_date=2026-01-01&end_date=2026-01-31", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDateRangeFeedback_WithStudentID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Range", "Student", "2b", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Get("/feedback/date-range", ctx.resource.GetDateRangeFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET",
		fmt.Sprintf("/feedback/date-range?start_date=2026-01-01&end_date=2026-01-31&student_id=%d", student.ID), nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDateRangeFeedback_InvalidStartDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/date-range", ctx.resource.GetDateRangeFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/date-range?start_date=invalid&end_date=2026-01-31", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGetDateRangeFeedback_InvalidEndDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/date-range", ctx.resource.GetDateRangeFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/date-range?start_date=2026-01-01&end_date=invalid", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGetDateRangeFeedback_InvalidStudentID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/date-range", ctx.resource.GetDateRangeFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET",
		"/feedback/date-range?start_date=2026-01-01&end_date=2026-01-31&student_id=invalid", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// CREATE FEEDBACK TESTS
// =============================================================================

func TestCreateFeedback_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Create", "Feedback", "3c", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	// Value must be 'positive', 'neutral', or 'negative'
	body := map[string]interface{}{
		"value":             "positive",
		"day":               time.Now().Format("2006-01-02"),
		"time":              "12:30:00",
		"student_id":        student.ID,
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestCreateFeedback_MissingValue(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Missing", "Value", "3c", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	body := map[string]interface{}{
		"day":               time.Now().Format("2006-01-02"),
		"time":              "12:30:00",
		"student_id":        student.ID,
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateFeedback_MissingStudentID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	body := map[string]interface{}{
		"value":             "Great day!",
		"day":               time.Now().Format("2006-01-02"),
		"time":              "12:30:00",
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateFeedback_InvalidDateFormat(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Invalid", "Date", "3c", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	body := map[string]interface{}{
		"value":             "Great day!",
		"day":               "01-14-2026", // Wrong format
		"time":              "12:30:00",
		"student_id":        student.ID,
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateFeedback_InvalidTimeFormat(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Invalid", "Time", "3c", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	body := map[string]interface{}{
		"value":             "Great day!",
		"day":               time.Now().Format("2006-01-02"),
		"time":              "12:30", // Wrong format - missing seconds
		"student_id":        student.ID,
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// CREATE BATCH FEEDBACK TESTS
// =============================================================================

func TestCreateBatchFeedback_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test students
	student1 := testpkg.CreateTestStudent(t, ctx.db, "Batch", "One", "4a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student1.ID)

	student2 := testpkg.CreateTestStudent(t, ctx.db, "Batch", "Two", "4a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student2.ID)

	router := chi.NewRouter()
	router.Post("/feedback/batch", ctx.resource.CreateBatchFeedbackHandler())

	// Value must be 'positive', 'neutral', or 'negative'
	body := map[string]interface{}{
		"entries": []map[string]interface{}{
			{
				"value":             "positive",
				"day":               time.Now().Format("2006-01-02"),
				"time":              "10:00:00",
				"student_id":        student1.ID,
				"is_mensa_feedback": false,
			},
			{
				"value":             "neutral",
				"day":               time.Now().Format("2006-01-02"),
				"time":              "12:30:00",
				"student_id":        student2.ID,
				"is_mensa_feedback": true,
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback/batch", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestCreateBatchFeedback_EmptyEntries(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/feedback/batch", ctx.resource.CreateBatchFeedbackHandler())

	body := map[string]interface{}{
		"entries": []map[string]interface{}{},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback/batch", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateBatchFeedback_InvalidEntry(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Batch", "Invalid", "4a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback/batch", ctx.resource.CreateBatchFeedbackHandler())

	// Value must be 'positive', 'neutral', or 'negative'
	body := map[string]interface{}{
		"entries": []map[string]interface{}{
			{
				"value":             "positive",
				"day":               time.Now().Format("2006-01-02"),
				"time":              "10:00:00",
				"student_id":        student.ID,
				"is_mensa_feedback": false,
			},
			{
				// Missing required fields (day, time, student_id)
				"value": "negative",
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback/batch", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DELETE FEEDBACK TESTS
// =============================================================================

func TestDeleteFeedback_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/feedback/{id}", ctx.resource.DeleteFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/feedback/999999", nil,
		testutil.WithPermissions("feedback:delete"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestDeleteFeedback_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/feedback/{id}", ctx.resource.DeleteFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/feedback/invalid", nil,
		testutil.WithPermissions("feedback:delete"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET FEEDBACK SUCCESS TEST
// =============================================================================

func TestGetFeedback_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Get", "FeedbackSuccess", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	// Create feedback entry via service
	entry := createTestFeedbackEntry(t, ctx, student.ID)
	defer cleanupFeedbackEntry(t, ctx.db, entry.ID)

	router := chi.NewRouter()
	router.Get("/feedback/{id}", ctx.resource.GetFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/feedback/%d", entry.ID), nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains the feedback data
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be a map")
	assert.Equal(t, float64(entry.ID), data["id"])
	assert.Equal(t, "positive", data["value"])
}

// =============================================================================
// DELETE FEEDBACK SUCCESS TEST
// =============================================================================

func TestDeleteFeedback_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Delete", "FeedbackSuccess", "2b", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	// Create feedback entry via service
	entry := createTestFeedbackEntry(t, ctx, student.ID)
	// No defer cleanup - we're deleting it

	router := chi.NewRouter()
	router.Delete("/feedback/{id}", ctx.resource.DeleteFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/feedback/%d", entry.ID), nil,
		testutil.WithPermissions("feedback:delete"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify entry was deleted by trying to retrieve it
	_, err := ctx.services.Feedback.GetEntryByID(context.Background(), entry.ID)
	assert.Error(t, err, "Expected error when getting deleted feedback")
}

// =============================================================================
// FEEDBACK REQUEST BIND VALIDATION TESTS
// =============================================================================

func TestCreateFeedback_MissingDay(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Missing", "Day", "3c", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	body := map[string]interface{}{
		"value":             "positive",
		"time":              "12:30:00",
		"student_id":        student.ID,
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateFeedback_MissingTime(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Missing", "Time", "3c", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	body := map[string]interface{}{
		"value":             "positive",
		"day":               time.Now().Format("2006-01-02"),
		"student_id":        student.ID,
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateFeedback_InvalidFeedbackValue(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Invalid", "Value", "3c", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	// Value must be 'positive', 'neutral', or 'negative' - using invalid value
	body := map[string]interface{}{
		"value":             "invalid_value",
		"day":               time.Now().Format("2006-01-02"),
		"time":              "12:30:00",
		"student_id":        student.ID,
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	// This should fail in service validation (value must be positive/neutral/negative)
	assert.NotEqual(t, http.StatusCreated, rr.Code, "Should not create feedback with invalid value")
}

func TestCreateFeedback_ZeroStudentID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	body := map[string]interface{}{
		"value":             "positive",
		"day":               time.Now().Format("2006-01-02"),
		"time":              "12:30:00",
		"student_id":        0,
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateFeedback_NegativeStudentID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	body := map[string]interface{}{
		"value":             "positive",
		"day":               time.Now().Format("2006-01-02"),
		"time":              "12:30:00",
		"student_id":        -1,
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// LIST FEEDBACK WITH SERVICE ERROR TESTS
// =============================================================================

func TestListFeedback_WithInvalidStudentIDFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback", ctx.resource.ListFeedbackHandler())

	// Test with invalid student_id - should be ignored silently (not parsed to int)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback?student_id=invalid", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should still return success - invalid filters are ignored
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListFeedback_WithInvalidDateFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback", ctx.resource.ListFeedbackHandler())

	// Test with invalid date - should be ignored silently
	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback?date=not-a-date", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should still return success - invalid filters are ignored
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListFeedback_WithZeroStudentIDFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback", ctx.resource.ListFeedbackHandler())

	// Test with zero student_id - should be ignored (studentID > 0 check)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback?student_id=0", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should still return success - zero is filtered out
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListFeedback_WithMensaFalseFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback", ctx.resource.ListFeedbackHandler())

	// Test is_mensa=false branch
	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback?is_mensa=false", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListFeedback_WithMensa1Filter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback", ctx.resource.ListFeedbackHandler())

	// Test is_mensa=1 branch (truthy value)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback?is_mensa=1", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// MENSA FEEDBACK EDGE CASES
// =============================================================================

func TestGetMensaFeedback_With0Filter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/mensa", ctx.resource.GetMensaFeedbackHandler())

	// Test is_mensa=0 branch (falsy value)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/mensa?is_mensa=0", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetMensaFeedback_DefaultToTrue(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/mensa", ctx.resource.GetMensaFeedbackHandler())

	// Test with random value - should default to true (not false or 0)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/mensa?is_mensa=random", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// BATCH FEEDBACK EDGE CASES
// =============================================================================

func TestCreateBatchFeedback_WithInvalidEntryInMiddle(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test students
	student1 := testpkg.CreateTestStudent(t, ctx.db, "BatchMid", "One", "4a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student1.ID)

	router := chi.NewRouter()
	router.Post("/feedback/batch", ctx.resource.CreateBatchFeedbackHandler())

	// First entry valid, second invalid (missing time)
	body := map[string]interface{}{
		"entries": []map[string]interface{}{
			{
				"value":             "positive",
				"day":               time.Now().Format("2006-01-02"),
				"time":              "10:00:00",
				"student_id":        student1.ID,
				"is_mensa_feedback": false,
			},
			{
				"value":             "negative",
				"day":               time.Now().Format("2006-01-02"),
				"student_id":        student1.ID,
				"is_mensa_feedback": false,
				// Missing time - should fail validation
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback/batch", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateBatchFeedback_MensaFeedback(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "BatchMensa", "Student", "4a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback/batch", ctx.resource.CreateBatchFeedbackHandler())

	// Create mensa feedback
	body := map[string]interface{}{
		"entries": []map[string]interface{}{
			{
				"value":             "positive",
				"day":               time.Now().Format("2006-01-02"),
				"time":              "12:00:00",
				"student_id":        student.ID,
				"is_mensa_feedback": true, // Mensa feedback
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback/batch", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

// =============================================================================
// CREATE FEEDBACK WITH VALID VALUES
// =============================================================================

func TestCreateFeedback_NeutralValue(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Neutral", "Feedback", "3c", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	body := map[string]interface{}{
		"value":             "neutral",
		"day":               time.Now().Format("2006-01-02"),
		"time":              "12:30:00",
		"student_id":        student.ID,
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestCreateFeedback_NegativeValue(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Negative", "Feedback", "3c", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	body := map[string]interface{}{
		"value":             "negative",
		"day":               time.Now().Format("2006-01-02"),
		"time":              "12:30:00",
		"student_id":        student.ID,
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestCreateFeedback_MensaFeedback(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "Mensa", "Feedback", "3c", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	body := map[string]interface{}{
		"value":             "positive",
		"day":               time.Now().Format("2006-01-02"),
		"time":              "12:00:00",
		"student_id":        student.ID,
		"is_mensa_feedback": true, // Testing mensa feedback creation
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

// =============================================================================
// CRUD END-TO-END TEST
// =============================================================================

func TestFeedback_CRUD_EndToEnd(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "CRUD", "E2E", "5a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())
	router.Get("/feedback/{id}", ctx.resource.GetFeedbackHandler())
	router.Delete("/feedback/{id}", ctx.resource.DeleteFeedbackHandler())

	// CREATE
	createBody := map[string]interface{}{
		"value":             "positive",
		"day":               time.Now().Format("2006-01-02"),
		"time":              "14:30:00",
		"student_id":        student.ID,
		"is_mensa_feedback": false,
	}

	createReq := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", createBody,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	createRR := testutil.ExecuteRequest(router, createReq)
	testutil.AssertSuccessResponse(t, createRR, http.StatusCreated)

	// Extract created ID
	createResponse := testutil.ParseJSONResponse(t, createRR.Body.Bytes())
	data := createResponse["data"].(map[string]interface{})
	feedbackID := int64(data["id"].(float64))

	// READ
	getReq := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/feedback/%d", feedbackID), nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	getRR := testutil.ExecuteRequest(router, getReq)
	testutil.AssertSuccessResponse(t, getRR, http.StatusOK)

	getResponse := testutil.ParseJSONResponse(t, getRR.Body.Bytes())
	getData := getResponse["data"].(map[string]interface{})
	assert.Equal(t, "positive", getData["value"])
	assert.Equal(t, float64(student.ID), getData["student_id"])

	// DELETE
	deleteReq := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/feedback/%d", feedbackID), nil,
		testutil.WithPermissions("feedback:delete"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	deleteRR := testutil.ExecuteRequest(router, deleteReq)
	testutil.AssertSuccessResponse(t, deleteRR, http.StatusOK)

	// VERIFY DELETED
	verifyReq := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/feedback/%d", feedbackID), nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	verifyRR := testutil.ExecuteRequest(router, verifyReq)
	testutil.AssertNotFound(t, verifyRR)
}

// =============================================================================
// ROUTER TEST
// =============================================================================

func TestNewResource(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Test that NewResource creates a valid resource
	resource := feedbackAPI.NewResource(ctx.services.Feedback)
	assert.NotNil(t, resource)

	// Test that Router returns a valid router
	router := resource.Router()
	assert.NotNil(t, router)
}

// =============================================================================
// SERVICE ERROR PATH TESTS
// =============================================================================

func TestGetStudentFeedback_ServiceError(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/feedback/student/{id}", ctx.resource.GetStudentFeedbackHandler())

	// Use a non-existent student ID that could trigger a service-level error or empty result
	// This tests the error path when the service returns an empty result (no error but no data)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/student/999999999", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return success with empty list (no feedback for non-existent student)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDateFeedback_ServiceSuccess(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create student and feedback
	student := testpkg.CreateTestStudent(t, ctx.db, "DateService", "Test", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	entry := createTestFeedbackEntry(t, ctx, student.ID)
	defer cleanupFeedbackEntry(t, ctx.db, entry.ID)

	router := chi.NewRouter()
	router.Get("/feedback/date/{date}", ctx.resource.GetDateFeedbackHandler())

	// Use the same date as the created entry
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/feedback/date/%s", time.Now().Format("2006-01-02")), nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetMensaFeedback_ServiceSuccess(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create student and mensa feedback
	student := testpkg.CreateTestStudent(t, ctx.db, "MensaService", "Test", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	entry := createTestMensaFeedbackEntry(t, ctx, student.ID)
	defer cleanupFeedbackEntry(t, ctx.db, entry.ID)

	router := chi.NewRouter()
	router.Get("/feedback/mensa", ctx.resource.GetMensaFeedbackHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/feedback/mensa", nil,
		testutil.WithPermissions("feedback:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// REQUEST TO MODEL ERROR PATHS
// =============================================================================

func TestCreateFeedback_InvalidDayFormatInModel(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "ModelError", "Day", "3c", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	// Invalid day format that passes basic validation but fails in requestToModel
	body := map[string]any{
		"value":             "positive",
		"day":               "2026-13-45", // Invalid date (month 13, day 45)
		"time":              "12:30:00",
		"student_id":        student.ID,
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateFeedback_InvalidTimeFormatInModel(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "ModelError", "Time", "3c", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.CreateFeedbackHandler())

	// Invalid time format that passes basic validation but fails in requestToModel
	body := map[string]any{
		"value":             "positive",
		"day":               time.Now().Format("2006-01-02"),
		"time":              "25:99:99", // Invalid time (hour 25, minute 99)
		"student_id":        student.ID,
		"is_mensa_feedback": false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// BATCH FEEDBACK WITH REQUESTTOMODEL ERROR
// =============================================================================

func TestCreateBatchFeedback_WithInvalidDayInLoop(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "BatchModel", "Day", "4a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback/batch", ctx.resource.CreateBatchFeedbackHandler())

	// First entry valid (passes Bind), but with invalid day format that fails in requestToModel
	body := map[string]any{
		"entries": []map[string]any{
			{
				"value":             "positive",
				"day":               "2026-13-45", // Invalid date
				"time":              "10:00:00",
				"student_id":        student.ID,
				"is_mensa_feedback": false,
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback/batch", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateBatchFeedback_WithInvalidTimeInLoop(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test student
	student := testpkg.CreateTestStudent(t, ctx.db, "BatchModel", "Time", "4a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/feedback/batch", ctx.resource.CreateBatchFeedbackHandler())

	// Entry with valid day but invalid time format
	body := map[string]any{
		"entries": []map[string]any{
			{
				"value":             "positive",
				"day":               time.Now().Format("2006-01-02"),
				"time":              "25:99:99", // Invalid time
				"student_id":        student.ID,
				"is_mensa_feedback": false,
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback/batch", body,
		testutil.WithPermissions("feedback:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// createTestFeedbackEntry creates a feedback entry in the database for testing
func createTestFeedbackEntry(t *testing.T, ctx *testContext, studentID int64) *feedback.Entry {
	t.Helper()

	entry := &feedback.Entry{
		Value:           "positive",
		Day:             time.Now().Truncate(24 * time.Hour),
		Time:            time.Date(1970, 1, 1, 12, 30, 0, 0, time.UTC),
		StudentID:       studentID,
		IsMensaFeedback: false,
	}

	err := ctx.services.Feedback.CreateEntry(context.Background(), entry)
	require.NoError(t, err, "Failed to create test feedback entry")

	return entry
}

// createTestMensaFeedbackEntry creates a mensa feedback entry in the database for testing
func createTestMensaFeedbackEntry(t *testing.T, ctx *testContext, studentID int64) *feedback.Entry {
	t.Helper()

	entry := &feedback.Entry{
		Value:           "positive",
		Day:             time.Now().Truncate(24 * time.Hour),
		Time:            time.Date(1970, 1, 1, 12, 0, 0, 0, time.UTC),
		StudentID:       studentID,
		IsMensaFeedback: true, // Mensa feedback
	}

	err := ctx.services.Feedback.CreateEntry(context.Background(), entry)
	require.NoError(t, err, "Failed to create test mensa feedback entry")

	return entry
}

// cleanupFeedbackEntry removes a feedback entry from the database
func cleanupFeedbackEntry(t *testing.T, db *bun.DB, entryID int64) {
	t.Helper()

	_, err := db.NewDelete().
		Model((*feedback.Entry)(nil)).
		TableExpr("feedback.entries").
		Where("id = ?", entryID).
		Exec(context.Background())
	if err != nil {
		t.Logf("cleanup feedback entry %d: %v", entryID, err)
	}
}
