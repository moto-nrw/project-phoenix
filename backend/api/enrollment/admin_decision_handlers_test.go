package enrollment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// mockDecisionService is a hand-rolled fake that records inputs and
// replays canned outputs. Lives in this file because every admin-
// decision test uses it; sharing it with other suites is a refactor
// for when the second consumer shows up.
type mockDecisionService struct {
	// List
	listFilters  enrollmentService.RequestFilters
	listResult   []*enrollmentService.RequestSummary
	listErr      error
	listCalls    int
	getRequestID int64
	getResult    *enrollmentService.RequestSummary
	getErr       error

	listChildOffResult map[int64][]enrollmentService.ChildOfferingRow
	listChildOffErr    error

	decideInput  enrollmentService.DecideInput
	decideResult *enrollmentService.DecideOutcome
	decideErr    error

	// ExportPhase: records the args the handler forwards (so a handler
	// test can assert the format + actor were threaded through) and
	// replays a canned payload/error.
	exportPhaseID     int64
	exportActorID     int64
	exportActorRole   string
	exportFormat      string
	exportChildStatus string
	exportCalls       int
	exportResult      *enrollmentService.PhaseExport
	exportErr         error
}

func (m *mockDecisionService) List(_ context.Context, f enrollmentService.RequestFilters) ([]*enrollmentService.RequestSummary, error) {
	m.listFilters = f
	m.listCalls++
	return m.listResult, m.listErr
}

func (m *mockDecisionService) Get(_ context.Context, id int64) (*enrollmentService.RequestSummary, error) {
	m.getRequestID = id
	return m.getResult, m.getErr
}

func (m *mockDecisionService) Decide(_ context.Context, input enrollmentService.DecideInput) (*enrollmentService.DecideOutcome, error) {
	m.decideInput = input
	return m.decideResult, m.decideErr
}

func (m *mockDecisionService) ListChildOfferings(_ context.Context, _ int64) (map[int64][]enrollmentService.ChildOfferingRow, error) {
	return m.listChildOffResult, m.listChildOffErr
}

func (m *mockDecisionService) ExportPhase(_ context.Context, phaseID, actorAccountID int64, actorRole, format, childStatusFilter string) (*enrollmentService.PhaseExport, error) {
	m.exportCalls++
	m.exportPhaseID = phaseID
	m.exportActorID = actorAccountID
	m.exportActorRole = actorRole
	m.exportFormat = format
	m.exportChildStatus = childStatusFilter
	return m.exportResult, m.exportErr
}

func (m *mockDecisionService) RecordPhaseExportAudit(_ context.Context, _ int64, _ string, _ *enrollmentModels.Phase, _, _ string, _, _ int) error {
	return nil
}

func buildAdminDecisionRouter(svc enrollmentService.DecisionService) chi.Router {
	rs := &Resource{
		DecisionService: svc,
		// db nil → runInTenantTx short-circuits straight to the closure.
	}
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/enrollment/admin/requests", rs.listAdminRequests)
	r.Get("/enrollment/admin/requests/{id}", rs.getAdminRequest)
	r.Post("/enrollment/admin/requests/{id}/children/{childId}/decide", rs.decideAdminChild)
	return r
}

func executeAdminJSON(t *testing.T, router chi.Router, method, path string, body any) *httptest.ResponseRecorder {
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

// makeReqSummary builds a RequestSummary via base.Model so the
// embedded ID field is settable. Mirrors the mkRequest/mkChild
// approach in api/enrollment/admin_handlers_helpers_test.go.
func makeReqSummary(id, phaseID int64, children ...*enrollmentModels.RequestChild) *enrollmentService.RequestSummary {
	return &enrollmentService.RequestSummary{
		Request: &enrollmentModels.Request{
			Model:             baseModel.Model{ID: id},
			PhaseID:           phaseID,
			GuardianFirstName: "Anna",
			GuardianLastName:  "Beispiel",
			GuardianEmail:     "anna@example.test",
			SubmittedAt:       time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		},
		Phase:    &enrollmentModels.Phase{Name: "Schuljahr 2026"},
		Children: children,
	}
}

func makeChildSummary(id int64, firstName, lastName, status string) *enrollmentModels.RequestChild {
	return &enrollmentModels.RequestChild{
		Model:          baseModel.Model{ID: id},
		FirstName:      firstName,
		LastName:       lastName,
		DateOfBirth:    timezone.NewDate(2018, 4, 15),
		Status:         status,
		ActivationMode: enrollmentModels.ChildActivationScheduled,
	}
}

// --- listAdminRequests ---------------------------------------------------

func TestListAdminRequestsHandler_NilServiceReturns500(t *testing.T) {
	router := buildAdminDecisionRouter(nil)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListAdminRequestsHandler_HappyPathReturns200(t *testing.T) {
	mock := &mockDecisionService{
		listResult: []*enrollmentService.RequestSummary{
			makeReqSummary(1234, 5678, makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusSubmitted)),
		},
	}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":"1234"`,
		"int64 ID must be stringified per CLAUDE rule 4")
	assert.Contains(t, w.Body.String(), `"phase_name":"Schuljahr 2026"`)
}

func TestListAdminRequestsHandler_PhaseFilterParsed(t *testing.T) {
	mock := &mockDecisionService{listResult: []*enrollmentService.RequestSummary{}}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests?phase_id=42", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(42), mock.listFilters.PhaseID,
		"phase_id query param must be parsed to filter")
}

func TestListAdminRequestsHandler_ChildStatusFilterParsed(t *testing.T) {
	mock := &mockDecisionService{listResult: []*enrollmentService.RequestSummary{}}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests?child_status=waitlisted", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "waitlisted", mock.listFilters.ChildStatus)
}

func TestListAdminRequestsHandler_InvalidPhaseIDRejected(t *testing.T) {
	mock := &mockDecisionService{}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests?phase_id=notanumber", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 0, mock.listCalls, "service must NOT be called when phase_id is invalid")
}

func TestListAdminRequestsHandler_ZeroPhaseIDRejected(t *testing.T) {
	// phase_id=0 is logically invalid (postgres ids are positive) —
	// the handler rejects it as 400.
	mock := &mockDecisionService{}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests?phase_id=0", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListAdminRequestsHandler_ServiceErrorReturns500(t *testing.T) {
	mock := &mockDecisionService{listErr: errors.New("synthetic boom")}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getAdminRequest -----------------------------------------------------

func TestGetAdminRequestHandler_NilServiceReturns500(t *testing.T) {
	router := buildAdminDecisionRouter(nil)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetAdminRequestHandler_InvalidIDRejected(t *testing.T) {
	router := buildAdminDecisionRouter(&mockDecisionService{})
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/notanumber", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAdminRequestHandler_NegativeIDRejected(t *testing.T) {
	router := buildAdminDecisionRouter(&mockDecisionService{})
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/-1", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAdminRequestHandler_NotFoundMapped(t *testing.T) {
	mock := &mockDecisionService{getErr: enrollmentService.ErrDecisionRequestNotFound}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetAdminRequestHandler_GenericErrorMappedAs500(t *testing.T) {
	mock := &mockDecisionService{getErr: errors.New("synthetic boom")}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetAdminRequestHandler_HappyPathReturnsDetail(t *testing.T) {
	mock := &mockDecisionService{
		getResult: makeReqSummary(1234, 5678,
			makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusSubmitted),
		),
	}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"id":"1234"`)
	assert.Contains(t, body, `"id":"99"`)
	assert.Contains(t, body, `"date_of_birth":"2018-04-15"`)
}

func TestGetAdminRequestHandler_StitchesChildOfferings(t *testing.T) {
	mock := &mockDecisionService{
		getResult: makeReqSummary(1234, 5678,
			makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusSubmitted),
		),
		listChildOffResult: map[int64][]enrollmentService.ChildOfferingRow{
			99: {{
				OfferingID:     7777,
				OfferingName:   "OGS-Nachmittag",
				DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
				SelectedDays:   nil,
				AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			}},
		},
	}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"offering_id":"7777"`,
		"per-child offerings must be stitched onto the detail response")
	assert.Contains(t, body, `"offering_name":"OGS-Nachmittag"`)
}

func TestGetAdminRequestHandler_TolerantOfMissingChildOfferings(t *testing.T) {
	// ListChildOfferings is best-effort — failure must NOT kill the
	// detail response, the admin just sees no Betreuungsangebote
	// section.
	mock := &mockDecisionService{
		getResult: makeReqSummary(1234, 5678,
			makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusSubmitted),
		),
		listChildOffErr: errors.New("synthetic offerings error"),
	}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	assert.Equal(t, http.StatusOK, w.Code,
		"detail must succeed even when ListChildOfferings fails")
	assert.NotContains(t, w.Body.String(), `"offering_id"`,
		"failed offerings lookup → no offerings in response")
}

// --- decideAdminChild ----------------------------------------------------

func TestDecideAdminChildHandler_NilServiceReturns500(t *testing.T) {
	router := buildAdminDecisionRouter(nil)
	body := map[string]any{"status": "approved"}
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide", body)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDecideAdminChildHandler_InvalidRequestIDRejected(t *testing.T) {
	router := buildAdminDecisionRouter(&mockDecisionService{})
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/notanumber/children/99/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecideAdminChildHandler_InvalidChildIDRejected(t *testing.T) {
	router := buildAdminDecisionRouter(&mockDecisionService{})
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/notanumber/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecideAdminChildHandler_HappyPathReturns200(t *testing.T) {
	mock := &mockDecisionService{
		decideResult: &enrollmentService.DecideOutcome{
			Child: makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusApproved),
		},
	}
	router := buildAdminDecisionRouter(mock)
	body := map[string]any{"status": "approved", "reason": "OK"}
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide", body)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(1234), mock.decideInput.RequestID)
	assert.Equal(t, int64(99), mock.decideInput.ChildID)
	assert.Equal(t, enrollmentService.DecisionApproved, mock.decideInput.Status)
	assert.Equal(t, "OK", mock.decideInput.Reason)
	assert.Contains(t, w.Body.String(), `"status":"approved"`)
}

func TestDecideAdminChildHandler_ChildNotFoundMapsTo404(t *testing.T) {
	mock := &mockDecisionService{decideErr: enrollmentService.ErrDecisionChildNotFound}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDecideAdminChildHandler_RequestNotFoundMapsTo404(t *testing.T) {
	mock := &mockDecisionService{decideErr: enrollmentService.ErrDecisionRequestNotFound}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDecideAdminChildHandler_InvalidStatusMapsTo400(t *testing.T) {
	mock := &mockDecisionService{decideErr: enrollmentService.ErrDecisionInvalidStatus}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "bogus"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecideAdminChildHandler_AlreadyTerminalMapsTo400(t *testing.T) {
	mock := &mockDecisionService{decideErr: enrollmentService.ErrDecisionAlreadyTerminal}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecideAdminChildHandler_InvalidDataMapsTo400(t *testing.T) {
	mock := &mockDecisionService{decideErr: enrollmentService.ErrDecisionInvalidData}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecideAdminChildHandler_GenericErrorMapsTo500(t *testing.T) {
	mock := &mockDecisionService{decideErr: errors.New("synthetic boom")}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDecideAdminChildHandler_MalformedJSONReturns400(t *testing.T) {
	router := buildAdminDecisionRouter(&mockDecisionService{})
	req := httptest.NewRequest(http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		strings.NewReader("{not valid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
