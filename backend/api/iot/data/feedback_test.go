// Package data_test tests (feedback) the IoT feedback API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package data_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	dataAPI "github.com/moto-nrw/project-phoenix/api/iot/data"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const feedbackEnabledSetting = "feedback.enabled"

// feedbackTestContext holds shared test dependencies.
type feedbackTestContext struct {
	db         *bun.DB
	resource   *dataAPI.FeedbackResource
	setEnabled func(bool)
}

// setupFeedbackModule initializes the feedback route.
func setupFeedbackModule(t *testing.T) *feedbackTestContext {
	t.Helper()

	db, svc, feedback := testutil.SetupFeedbackAPITest(t)
	setEnabled := func(enabled bool) {
		require.NoError(t, svc.Settings.SetValue(testpkg.Ctx(t), feedbackEnabledSetting, enabled, nil, nil))
	}
	setEnabled(true)

	// Create feedback resource
	resource := dataAPI.NewFeedbackResource(
		svc.Users,
		feedback,
		func(int, string) {},
		nil,
	)

	return &feedbackTestContext{
		db:         db,
		resource:   resource,
		setEnabled: setEnabled,
	}
}

// =============================================================================
// SUBMIT FEEDBACK TESTS
// =============================================================================

func TestSubmitFeedback_NoDevice(t *testing.T) {
	t.Parallel()
	ctx := setupFeedbackModule(t)

	router := ctx.resource.Router()

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
	t.Parallel()
	ctx := setupFeedbackModule(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-1")

	router := ctx.resource.Router()

	// Send invalid JSON body
	req := httptest.NewRequest("POST", "/feedback", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	testutil.WithDeviceContext(testDevice)(req)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestSubmitFeedback_MissingStudentID(t *testing.T) {
	t.Parallel()
	ctx := setupFeedbackModule(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-2")

	router := ctx.resource.Router()

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
	t.Parallel()
	ctx := setupFeedbackModule(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-3")

	router := ctx.resource.Router()

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
	t.Parallel()
	ctx := setupFeedbackModule(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-4")

	router := ctx.resource.Router()

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
	t.Parallel()
	ctx := setupFeedbackModule(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-5")

	router := ctx.resource.Router()

	body := map[string]interface{}{
		"student_id": 99999, // Non-existent student
		"value":      "positive",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
	assert.Equal(t, `{"status":"error","error":"student not found"}`+"\n", rr.Body.String())
}

// A graduated (alumnus) student is soft-deleted and gone from every kiosk and
// staff workflow, but GetStudentByID is unfiltered. A feedback submission that
// races the graduation apply (or arrives from a kiosk holding a stale roster)
// must therefore be rejected here with the same 404 an unknown student gets, so
// no new row is written against a graduate and PyrePortal needs no new error
// mapping (#405).
func TestSubmitFeedback_Alumnus(t *testing.T) {
	t.Parallel()
	ctx := setupFeedbackModule(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-alumnus")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "Graduate", "4a")

	_, err := ctx.db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(usersModel.StudentStatusAlumnus)).
		Where("id = ?", student.ID).
		Exec(t.Context())
	require.NoError(t, err)

	router := ctx.resource.Router()

	body := map[string]interface{}{
		"student_id": student.ID,
		"value":      "positive",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)

	count, err := ctx.db.NewSelect().
		TableExpr(`feedback.entries`).
		Where("student_id = ?", student.ID).
		Count(t.Context())
	require.NoError(t, err)
	assert.Zero(t, count, "no feedback entry may be written for a graduated student")
}

func TestSubmitFeedback_Success(t *testing.T) {
	t.Parallel()
	ctx := setupFeedbackModule(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-6")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "Student", "1a")

	router := ctx.resource.Router()

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
	t.Parallel()
	ctx := setupFeedbackModule(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-7")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "Student2", "1b")

	router := ctx.resource.Router()

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
	t.Parallel()
	ctx := setupFeedbackModule(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-8")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "Student3", "1c")

	router := ctx.resource.Router()

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
	t.Parallel()
	ctx := setupFeedbackModule(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-9")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "Student4", "1d")

	router := ctx.resource.Router()

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
	t.Parallel()
	ctx := setupFeedbackModule(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-disabled")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "DisabledStudent", "2a")

	ctx.setEnabled(false)
	router := ctx.resource.Router()

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
	t.Parallel()
	ctx := setupFeedbackModule(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-enabled")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "EnabledStudent", "2b")

	router := ctx.resource.Router()

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
