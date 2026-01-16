// Package feedback_test tests the IoT feedback API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package feedback_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	feedbackAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/feedback"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/testutil"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
	"github.com/moto-nrw/project-phoenix/internal/adapter/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *feedbackAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	// Create feedback resource
	resource := feedbackAPI.NewResource(
		svc.IoT,
		svc.Users,
		svc.Feedback,
	)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// =============================================================================
// SUBMIT FEEDBACK TESTS
// =============================================================================

func TestSubmitFeedback_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.SubmitFeedbackHandler())

	body := map[string]interface{}{
		"student_id": 1,
		"value":      "positive",
	}

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestSubmitFeedback_InvalidJSON(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-1")

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.SubmitFeedbackHandler())

	// Send invalid JSON body
	req := httptest.NewRequest("POST", "/feedback", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	// Add device context
	reqCtx := context.WithValue(req.Context(), device.CtxDevice, testDevice)
	req = req.WithContext(reqCtx)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestSubmitFeedback_MissingStudentID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-2")

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.SubmitFeedbackHandler())

	body := map[string]interface{}{
		"value": "positive",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestSubmitFeedback_MissingValue(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-3")

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.SubmitFeedbackHandler())

	body := map[string]interface{}{
		"student_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestSubmitFeedback_InvalidStudentID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-4")

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.SubmitFeedbackHandler())

	body := map[string]interface{}{
		"student_id": 0, // Invalid - must be positive
		"value":      "positive",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestSubmitFeedback_StudentNotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-5")

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.SubmitFeedbackHandler())

	body := map[string]interface{}{
		"student_id": 99999, // Non-existent student
		"value":      "positive",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestSubmitFeedback_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-6")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "Student", "1a")

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.SubmitFeedbackHandler())

	body := map[string]interface{}{
		"student_id": student.ID,
		"value":      "positive",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestSubmitFeedback_NeutralValue(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-7")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "Student2", "1b")

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.SubmitFeedbackHandler())

	body := map[string]interface{}{
		"student_id": student.ID,
		"value":      "neutral",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestSubmitFeedback_NegativeValue(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-8")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "Student3", "1c")

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.SubmitFeedbackHandler())

	body := map[string]interface{}{
		"student_id": student.ID,
		"value":      "negative",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestSubmitFeedback_InvalidValue(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-9")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "Student4", "1d")

	router := chi.NewRouter()
	router.Post("/feedback", ctx.resource.SubmitFeedbackHandler())

	body := map[string]interface{}{
		"student_id": student.ID,
		"value":      "invalid_value", // Not a valid feedback value
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error for invalid value (validation happens in service)
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnprocessableEntity}, rr.Code)
}
