package enrollment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// mockRolloverService records its inputs and replays canned outputs.
// Lives in this file because every test in here uses it; making it
// shared with other rollover-handler suites later is a refactor for
// when the second consumer shows up.
type mockRolloverService struct {
	createRequest enrollmentService.CreatePhaseFromSourceRequest
	createResult  *enrollmentService.RolloverResult
	createErr     error

	listPhaseID int64
	listResult  []*enrollmentService.ReviewQueueItem
	listErr     error

	decideInput enrollmentService.DecideReviewRequest
	decideErr   error
}

func (m *mockRolloverService) CreatePhaseFromSource(_ context.Context, req enrollmentService.CreatePhaseFromSourceRequest) (*enrollmentService.RolloverResult, error) {
	m.createRequest = req
	return m.createResult, m.createErr
}

func (m *mockRolloverService) ListReviewQueue(_ context.Context, phaseID int64) ([]*enrollmentService.ReviewQueueItem, error) {
	m.listPhaseID = phaseID
	return m.listResult, m.listErr
}

func (m *mockRolloverService) DecideReview(_ context.Context, req enrollmentService.DecideReviewRequest) error {
	m.decideInput = req
	return m.decideErr
}

func (m *mockRolloverService) RunDeadlineWorker(_ context.Context, _ time.Time) (*enrollmentService.DeadlineWorkerSummary, error) {
	return nil, nil
}

// buildRolloverRouter wires a minimal chi router that mirrors the
// production routes for the rollover surface. Uses nil DB so
// Resource.runInTenantTx short-circuits straight into the handler —
// the mock service handles the business logic.
func buildRolloverRouter(svc enrollmentService.RolloverService) chi.Router {
	rs := &Resource{
		RolloverService: svc,
		// db: nil so runInTenantTx skips the tenant.WithTenantTx wrap.
	}
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Post("/enrollment/phases/{id}/rollover", rs.createRollover)
	r.Get("/enrollment/phases/{id}/review", rs.listRolloverReview)
	r.Post("/enrollment/admin/request-children/{id}/rollover-review", rs.decideRolloverReview)
	return r
}

func executeJSON(t *testing.T, router chi.Router, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		req = httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// --- createRollover ---

func TestCreateRolloverHandler_HappyPathReturns201(t *testing.T) {
	mock := &mockRolloverService{
		createResult: &enrollmentService.RolloverResult{
			Phase: &enrollmentModels.Phase{
				Name:             "Schuljahr 2027",
				Kind:             enrollmentModels.PhaseKindSchoolYear,
				ServiceStartDate: timezone.NewDate(2027, 9, 1),
				ServiceEndDate:   timezone.NewDate(2028, 7, 31),
				IsActive:         true,
				CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
			},
			SourceChildCount: 5,
			RolledCount:      4,
			ReviewCount:      1,
			ReviewByReason:   map[string]int{"grade_above_max": 1},
			RequestCount:     3,
			EnqueuedEmails:   3,
		},
	}
	mock.createResult.Phase.SetTenantID(1)
	router := buildRolloverRouter(mock)

	body := map[string]any{
		"name":               "Schuljahr 2027",
		"kind":               "school_year",
		"service_start_date": "2027-09-01",
		"service_end_date":   "2028-07-31",
		"rollover_mode":      "opt_out",
		"rollover_deadline":  "2027-08-15T00:00:00Z",
	}
	w := executeJSON(t, router, http.MethodPost, "/enrollment/phases/42/rollover", body)

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, int64(42), mock.createRequest.SourcePhaseID, "source id must be parsed from URL")
	assert.Equal(t, "Schuljahr 2027", mock.createRequest.Name)
	assert.Equal(t, "opt_out", mock.createRequest.RolloverMode)
	assert.True(t, mock.createRequest.RolloverBumpsGrade, "bumps_grade defaults to true when omitted")
	assert.Equal(t, timezone.NewDate(2027, 9, 1), mock.createRequest.ServiceStartDate)
	assert.Equal(t, time.Date(2027, 8, 15, 0, 0, 0, 0, time.UTC), mock.createRequest.RolloverDeadline)
}

func TestCreateRolloverHandler_PassesBumpsGradeFalseThrough(t *testing.T) {
	mock := &mockRolloverService{
		createResult: &enrollmentService.RolloverResult{
			Phase:          &enrollmentModels.Phase{Name: "h"},
			ReviewByReason: map[string]int{},
		},
	}
	router := buildRolloverRouter(mock)

	bumps := false
	body := map[string]any{
		"name":                 "Halbjahr",
		"kind":                 "custom",
		"service_start_date":   "2027-02-01",
		"service_end_date":     "2027-07-31",
		"rollover_mode":        "opt_in",
		"rollover_deadline":    "2027-01-15T00:00:00Z",
		"rollover_bumps_grade": &bumps,
	}
	w := executeJSON(t, router, http.MethodPost, "/enrollment/phases/7/rollover", body)
	require.Equal(t, http.StatusCreated, w.Code)
	assert.False(t, mock.createRequest.RolloverBumpsGrade, "explicit false must round-trip")
}

func TestCreateRolloverHandler_RejectsInvalidSourceID(t *testing.T) {
	router := buildRolloverRouter(&mockRolloverService{})
	w := executeJSON(t, router, http.MethodPost, "/enrollment/phases/notanumber/rollover", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateRolloverHandler_RejectsBadServiceDate(t *testing.T) {
	router := buildRolloverRouter(&mockRolloverService{})
	body := map[string]any{
		"name":               "x",
		"service_start_date": "not-a-date",
		"service_end_date":   "2027-07-31",
		"rollover_mode":      "opt_out",
		"rollover_deadline":  "2027-08-15T00:00:00Z",
	}
	w := executeJSON(t, router, http.MethodPost, "/enrollment/phases/1/rollover", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateRolloverHandler_MapsSourceNotFound(t *testing.T) {
	mock := &mockRolloverService{
		createErr: enrollmentService.ErrRolloverSourceNotFound,
	}
	router := buildRolloverRouter(mock)
	body := map[string]any{
		"name":               "x",
		"service_start_date": "2027-09-01",
		"service_end_date":   "2028-07-31",
		"rollover_mode":      "opt_out",
		"rollover_deadline":  "2027-08-15T00:00:00Z",
	}
	w := executeJSON(t, router, http.MethodPost, "/enrollment/phases/1/rollover", body)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateRolloverHandler_MapsInvalidRequest(t *testing.T) {
	mock := &mockRolloverService{
		createErr: enrollmentService.ErrRolloverInvalidRequest,
	}
	router := buildRolloverRouter(mock)
	body := map[string]any{
		"name":               "x",
		"service_start_date": "2027-09-01",
		"service_end_date":   "2028-07-31",
		"rollover_mode":      "opt_out",
		"rollover_deadline":  "2027-08-15T00:00:00Z",
	}
	w := executeJSON(t, router, http.MethodPost, "/enrollment/phases/1/rollover", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateRolloverHandler_NilServiceReturns500(t *testing.T) {
	router := buildRolloverRouter(nil)
	w := executeJSON(t, router, http.MethodPost, "/enrollment/phases/1/rollover", map[string]any{})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- listRolloverReview ---

func TestListRolloverReviewHandler_HappyPath(t *testing.T) {
	source := &enrollmentModels.RequestChild{FirstName: "Lina", LastName: "Beispiel"}
	source.ID = 100
	four := int16(4)
	source.TargetGradeLevel = &four
	five := int16(5)

	child := &enrollmentModels.RequestChild{
		FirstName:        "Lina",
		LastName:         "Beispiel",
		Status:           enrollmentModels.ChildStatusPendingAdminReview,
		TargetGradeLevel: &five,
	}
	child.ID = 999
	child.RequestID = 555
	reason := enrollmentModels.ReviewReasonGradeAboveMax
	child.ReviewReason = &reason

	mock := &mockRolloverService{
		listResult: []*enrollmentService.ReviewQueueItem{
			{
				Child:       child,
				Request:     &enrollmentModels.Request{GuardianFirstName: "Anna", GuardianLastName: "Beispiel", GuardianEmail: "anna@example.com", StatusToken: "tok-abc"},
				SourceChild: source,
			},
		},
	}
	router := buildRolloverRouter(mock)

	w := executeJSON(t, router, http.MethodGet, "/enrollment/phases/77/review", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(77), mock.listPhaseID)

	var envelope struct {
		Data []ReviewItemResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data, 1)
	row := envelope.Data[0]
	assert.Equal(t, "999", strconv.FormatInt(child.ID, 10))
	assert.Equal(t, int64(999), row.ChildID)
	assert.Equal(t, "Lina", row.FirstName)
	require.NotNil(t, row.TargetGradeLevel)
	assert.Equal(t, int16(5), *row.TargetGradeLevel)
	require.NotNil(t, row.SourceGradeLevel)
	assert.Equal(t, int16(4), *row.SourceGradeLevel)
	assert.Equal(t, "anna@example.com", row.GuardianEmail)
	assert.Equal(t, "tok-abc", row.StatusToken)
}

func TestListRolloverReviewHandler_RejectsInvalidPhaseID(t *testing.T) {
	router := buildRolloverRouter(&mockRolloverService{})
	w := executeJSON(t, router, http.MethodGet, "/enrollment/phases/abc/review", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListRolloverReviewHandler_ServiceErrorReturns500(t *testing.T) {
	mock := &mockRolloverService{listErr: errors.New("boom")}
	router := buildRolloverRouter(mock)
	w := executeJSON(t, router, http.MethodGet, "/enrollment/phases/1/review", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- decideRolloverReview ---

func TestDecideRolloverReviewHandler_KeepHappyPath(t *testing.T) {
	mock := &mockRolloverService{}
	router := buildRolloverRouter(mock)

	newGrade := 4
	body := map[string]any{
		"decision":        "keep",
		"new_grade_level": newGrade,
	}
	w := executeJSON(t, router, http.MethodPost, "/enrollment/admin/request-children/123/rollover-review", body)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(123), mock.decideInput.RequestChildID)
	assert.Equal(t, "keep", mock.decideInput.Decision)
	require.NotNil(t, mock.decideInput.NewGradeLevel)
	assert.Equal(t, int16(4), *mock.decideInput.NewGradeLevel)
}

func TestDecideRolloverReviewHandler_DropClearsGrade(t *testing.T) {
	mock := &mockRolloverService{}
	router := buildRolloverRouter(mock)
	body := map[string]any{"decision": "drop"}
	w := executeJSON(t, router, http.MethodPost, "/enrollment/admin/request-children/999/rollover-review", body)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "drop", mock.decideInput.Decision)
	assert.Nil(t, mock.decideInput.NewGradeLevel)
}

func TestDecideRolloverReviewHandler_DeferHappyPath(t *testing.T) {
	mock := &mockRolloverService{}
	router := buildRolloverRouter(mock)
	body := map[string]any{"decision": "defer"}
	w := executeJSON(t, router, http.MethodPost, "/enrollment/admin/request-children/55/rollover-review", body)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "defer", mock.decideInput.Decision)
}

func TestDecideRolloverReviewHandler_RejectsInvalidID(t *testing.T) {
	router := buildRolloverRouter(&mockRolloverService{})
	body := map[string]any{"decision": "keep"}
	w := executeJSON(t, router, http.MethodPost, "/enrollment/admin/request-children/zero/rollover-review", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecideRolloverReviewHandler_MapsServiceInvalid(t *testing.T) {
	mock := &mockRolloverService{decideErr: enrollmentService.ErrRolloverReviewInvalid}
	router := buildRolloverRouter(mock)
	body := map[string]any{"decision": "bogus"}
	w := executeJSON(t, router, http.MethodPost, "/enrollment/admin/request-children/1/rollover-review", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecideRolloverReviewHandler_ServiceErrorReturns500(t *testing.T) {
	mock := &mockRolloverService{decideErr: errors.New("boom")}
	router := buildRolloverRouter(mock)
	body := map[string]any{"decision": "keep"}
	w := executeJSON(t, router, http.MethodPost, "/enrollment/admin/request-children/1/rollover-review", body)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
