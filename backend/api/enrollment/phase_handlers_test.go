package enrollment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

type mockPhaseService struct {
	listResult        []*enrollmentModels.Phase
	listErr           error
	listPublicOpenRes []*enrollmentModels.Phase
	listPublicOpenErr error
	getByIDID         int64
	getByIDResult     *enrollmentModels.Phase
	getByIDErr        error
	createInput       *enrollmentModels.Phase
	createResult      *enrollmentModels.Phase
	createErr         error
	updateInput       *enrollmentModels.Phase
	updateErr         error
	deleteID          int64
	deleteErr         error
	deleteImpactID    int64
	deleteImpactRes   *enrollmentService.PhaseDeleteImpact
	deleteImpactErr   error
}

func (m *mockPhaseService) List(_ context.Context) ([]*enrollmentModels.Phase, error) {
	return m.listResult, m.listErr
}
func (m *mockPhaseService) ListPublicOpen(_ context.Context, _ time.Time) ([]*enrollmentModels.Phase, error) {
	return m.listPublicOpenRes, m.listPublicOpenErr
}
func (m *mockPhaseService) GetByID(_ context.Context, id int64) (*enrollmentModels.Phase, error) {
	m.getByIDID = id
	return m.getByIDResult, m.getByIDErr
}
func (m *mockPhaseService) Create(_ context.Context, phase *enrollmentModels.Phase) (*enrollmentModels.Phase, error) {
	m.createInput = phase
	return m.createResult, m.createErr
}
func (m *mockPhaseService) Update(_ context.Context, phase *enrollmentModels.Phase) error {
	m.updateInput = phase
	return m.updateErr
}
func (m *mockPhaseService) Delete(_ context.Context, id int64) error {
	m.deleteID = id
	return m.deleteErr
}
func (m *mockPhaseService) DeleteImpact(_ context.Context, id int64) (*enrollmentService.PhaseDeleteImpact, error) {
	m.deleteImpactID = id
	return m.deleteImpactRes, m.deleteImpactErr
}

func buildPhaseRouter(svc enrollmentService.PhaseService) chi.Router {
	rs := &Resource{PhaseService: svc}
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/enrollment/phases", rs.listPhases)
	r.Get("/enrollment/phases/{id}", rs.getPhase)
	r.Post("/enrollment/phases", rs.createPhase)
	r.Put("/enrollment/phases/{id}", rs.updatePhase)
	r.Get("/enrollment/phases/{id}/delete-impact", rs.getPhaseDeleteImpact)
	r.Delete("/enrollment/phases/{id}", rs.deletePhase)
	return r
}

func executePhaseJSON(t *testing.T, router chi.Router, method, path string, body any) *httptest.ResponseRecorder {
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

func makePhaseModel(id int64, name string) *enrollmentModels.Phase {
	return &enrollmentModels.Phase{
		Model:            baseModel.Model{ID: id, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		Name:             name,
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: timezone.NewDate(2026, 9, 1),
		ServiceEndDate:   timezone.NewDate(2027, 7, 31),
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
}

func validPhaseBody(name string) map[string]any {
	return map[string]any{
		"name":               name,
		"kind":               "school_year",
		"service_start_date": "2026-09-01",
		"service_end_date":   "2027-07-31",
		"care_overflow_mode": "waitlist",
		"is_active":          true,
	}
}

// --- listPhases --------------------------------------------------------

func TestListPhasesHandler_NilServiceReturns500(t *testing.T) {
	router := buildPhaseRouter(nil)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListPhasesHandler_HappyPath(t *testing.T) {
	mock := &mockPhaseService{listResult: []*enrollmentModels.Phase{
		makePhaseModel(1234, "Schuljahr 2026"),
	}}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":"1234"`)
	assert.Contains(t, w.Body.String(), `"name":"Schuljahr 2026"`)
}

func TestListPhasesHandler_ServiceErrorReturns500(t *testing.T) {
	mock := &mockPhaseService{listErr: errors.New("synthetic boom")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getPhase ---------------------------------------------------------

func TestGetPhaseHandler_NilServiceReturns500(t *testing.T) {
	router := buildPhaseRouter(nil)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetPhaseHandler_InvalidIDRejected(t *testing.T) {
	router := buildPhaseRouter(&mockPhaseService{})
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases/notanumber", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetPhaseHandler_HappyPath(t *testing.T) {
	mock := &mockPhaseService{getByIDResult: makePhaseModel(1234, "Schuljahr 2026")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases/1234", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(1234), mock.getByIDID)
	assert.Contains(t, w.Body.String(), `"id":"1234"`)
}

func TestGetPhaseHandler_NotFoundReturns404(t *testing.T) {
	mock := &mockPhaseService{getByIDErr: enrollmentService.ErrPhaseNotFound}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetPhaseHandler_GenericErrorReturns500(t *testing.T) {
	mock := &mockPhaseService{getByIDErr: errors.New("synthetic boom")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- createPhase ------------------------------------------------------

func TestCreatePhaseHandler_NilServiceReturns500(t *testing.T) {
	router := buildPhaseRouter(nil)
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", validPhaseBody("X"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreatePhaseHandler_HappyPathReturns201(t *testing.T) {
	mock := &mockPhaseService{createResult: makePhaseModel(1234, "Schuljahr 2026")}
	router := buildPhaseRouter(mock)
	body := validPhaseBody("Schuljahr 2026")
	body["form_schema_id"] = "7777"
	body["enrollment_open_at"] = "2026-08-01T00:00:00Z"
	body["enrollment_close_at"] = "2026-08-31T23:59:00Z"
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", body)
	require.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, mock.createInput)
	assert.Equal(t, "Schuljahr 2026", mock.createInput.Name)
	require.NotNil(t, mock.createInput.FormSchemaID)
	assert.Equal(t, int64(7777), *mock.createInput.FormSchemaID)
	require.NotNil(t, mock.createInput.EnrollmentOpenAt)
	require.NotNil(t, mock.createInput.EnrollmentCloseAt)
}

func TestCreatePhaseHandler_BadServiceStartDateReturns400(t *testing.T) {
	router := buildPhaseRouter(&mockPhaseService{})
	body := validPhaseBody("X")
	body["service_start_date"] = "yesterday"
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePhaseHandler_BadServiceEndDateReturns400(t *testing.T) {
	router := buildPhaseRouter(&mockPhaseService{})
	body := validPhaseBody("X")
	body["service_end_date"] = "tomorrow"
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePhaseHandler_BadEnrollmentWindowReturns400(t *testing.T) {
	router := buildPhaseRouter(&mockPhaseService{})
	body := validPhaseBody("X")
	body["enrollment_open_at"] = "not-iso"
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePhaseHandler_BadFormSchemaIDReturns400(t *testing.T) {
	router := buildPhaseRouter(&mockPhaseService{})
	body := validPhaseBody("X")
	body["form_schema_id"] = "not-a-number"
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePhaseHandler_ServiceErrorReturns400(t *testing.T) {
	mock := &mockPhaseService{createErr: errors.New("synthetic boom")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", validPhaseBody("X"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- updatePhase ------------------------------------------------------

func TestUpdatePhaseHandler_NilServiceReturns500(t *testing.T) {
	router := buildPhaseRouter(nil)
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", validPhaseBody("X"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUpdatePhaseHandler_InvalidIDRejected(t *testing.T) {
	router := buildPhaseRouter(&mockPhaseService{})
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/notanumber", validPhaseBody("X"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdatePhaseHandler_HappyPathRefetchesAfterUpdate(t *testing.T) {
	mock := &mockPhaseService{getByIDResult: makePhaseModel(1234, "Updated")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", validPhaseBody("Updated"))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, mock.updateInput)
	assert.Equal(t, int64(1234), mock.updateInput.ID, "ID from URL must be stamped onto the model")
	assert.Equal(t, int64(1234), mock.getByIDID, "refetch must call GetByID after the update")
}

func TestUpdatePhaseHandler_BadDateReturns400(t *testing.T) {
	router := buildPhaseRouter(&mockPhaseService{})
	body := validPhaseBody("X")
	body["service_start_date"] = "yesterday"
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdatePhaseHandler_UpdateErrorReturns400(t *testing.T) {
	mock := &mockPhaseService{updateErr: errors.New("synthetic boom")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", validPhaseBody("X"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdatePhaseHandler_RefetchErrorReturns500(t *testing.T) {
	mock := &mockPhaseService{getByIDErr: errors.New("synthetic refetch boom")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", validPhaseBody("X"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- deletePhase ------------------------------------------------------

func TestDeletePhaseHandler_NilServiceReturns500(t *testing.T) {
	router := buildPhaseRouter(nil)
	w := executePhaseJSON(t, router, http.MethodDelete, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDeletePhaseHandler_InvalidIDRejected(t *testing.T) {
	router := buildPhaseRouter(&mockPhaseService{})
	w := executePhaseJSON(t, router, http.MethodDelete, "/enrollment/phases/notanumber", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeletePhaseHandler_HappyPathReturns204(t *testing.T) {
	mock := &mockPhaseService{}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodDelete, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, int64(1234), mock.deleteID)
}

func TestDeletePhaseHandler_NotFoundReturns404(t *testing.T) {
	mock := &mockPhaseService{deleteErr: enrollmentService.ErrPhaseNotFound}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodDelete, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeletePhaseHandler_GenericErrorReturns500(t *testing.T) {
	mock := &mockPhaseService{deleteErr: errors.New("synthetic boom")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodDelete, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getPhaseDeleteImpact ---------------------------------------------

func TestPhaseDeleteImpactHandler_ReturnsCounts(t *testing.T) {
	mock := &mockPhaseService{
		deleteImpactRes: &enrollmentService.PhaseDeleteImpact{
			Requests:      3,
			CareOfferings: 2,
			StudentsKept:  5,
		},
	}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases/1234/delete-impact", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(1234), mock.deleteImpactID)
	body := w.Body.String()
	assert.Contains(t, body, `"requests":3`)
	assert.Contains(t, body, `"care_offerings":2`)
	assert.Contains(t, body, `"students_kept":5`)
}

func TestPhaseDeleteImpactHandler_NotFoundReturns404(t *testing.T) {
	mock := &mockPhaseService{deleteImpactErr: enrollmentService.ErrPhaseNotFound}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases/1234/delete-impact", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
