// Package feedback_test tests the IoT feedback API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package feedback_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	feedbackAPI "github.com/moto-nrw/project-phoenix/api/iot/feedback"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/device"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// fakeSettingsService implements configSvc.SettingsService for testing.
type fakeSettingsService struct {
	boolValues map[string]bool
}

func (f *fakeSettingsService) GetSchema(_ context.Context, _ []string) (*configSvc.SettingsSchema, error) {
	return nil, nil
}
func (f *fakeSettingsService) GetSchemaForOperator(_ context.Context, _ []string) (*configSvc.SettingsSchema, error) {
	return nil, nil
}
func (f *fakeSettingsService) Resolve(_ context.Context, _ string) (any, error) { return nil, nil }
func (f *fakeSettingsService) ResolveString(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (f *fakeSettingsService) ResolveStringForTenant(_ context.Context, _ int64, _ string) (string, error) {
	return "", nil
}
func (f *fakeSettingsService) ResolveBool(_ context.Context, key string) (bool, error) {
	if val, ok := f.boolValues[key]; ok {
		return val, nil
	}
	return true, nil
}
func (f *fakeSettingsService) ResolveInt(_ context.Context, _ string) (int, error) { return 0, nil }
func (f *fakeSettingsService) HasTenantOverride(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (f *fakeSettingsService) SetValue(_ context.Context, _ string, _ any, _ *int64, _ []string) error {
	return nil
}
func (f *fakeSettingsService) ResetValue(_ context.Context, _ string, _ *int64, _ []string) error {
	return nil
}
func (f *fakeSettingsService) GetLoginImageURL(_ context.Context, _ int64) (string, error) {
	return "", nil
}
func (f *fakeSettingsService) SetLoginImageURL(_ context.Context, _ int64, _ string) (string, error) {
	return "", nil
}
func (f *fakeSettingsService) ClearLoginImageURL(_ context.Context, _ int64) (string, error) {
	return "", nil
}

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
		nil,
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

// =============================================================================
// FEEDBACK DISABLED (SETTINGS GUARD) TESTS
// =============================================================================

func TestSubmitFeedback_FeedbackDisabled(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-disabled")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "DisabledStudent", "2a")

	// Create resource with fake SettingsService that returns feedback.enabled = false
	disabledSettings := &fakeSettingsService{
		boolValues: map[string]bool{
			configModel.KeyFeedbackEnabled: false,
		},
	}
	resource := feedbackAPI.NewResource(
		ctx.services.IoT,
		ctx.services.Users,
		ctx.services.Feedback,
		disabledSettings,
	)

	router := chi.NewRouter()
	router.Post("/feedback", resource.SubmitFeedbackHandler())

	body := map[string]interface{}{
		"student_id": student.ID,
		"value":      "positive",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 200 with status "skipped"
	assert.Equal(t, http.StatusOK, rr.Code)

	var response struct {
		Status string `json:"status"`
		Data   struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"data"`
	}
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "skipped", response.Data.Status)
	assert.Equal(t, "feedback_disabled", response.Data.Reason)
}

func TestSubmitFeedback_FeedbackEnabled(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-enabled")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "EnabledStudent", "2b")

	// Create resource with fake SettingsService that returns feedback.enabled = true
	enabledSettings := &fakeSettingsService{
		boolValues: map[string]bool{
			configModel.KeyFeedbackEnabled: true,
		},
	}
	resource := feedbackAPI.NewResource(
		ctx.services.IoT,
		ctx.services.Users,
		ctx.services.Feedback,
		enabledSettings,
	)

	router := chi.NewRouter()
	router.Post("/feedback", resource.SubmitFeedbackHandler())

	body := map[string]interface{}{
		"student_id": student.ID,
		"value":      "positive",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should proceed normally and create the entry
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}
