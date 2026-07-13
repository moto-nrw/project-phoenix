package enrollment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	baseModel "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

type mockCareOfferingService struct {
	listResult              []*enrollmentModels.CareOffering
	listErr                 error
	listByPhaseID           int64
	listByPhaseResult       []*enrollmentModels.CareOffering
	listByPhaseErr          error
	listActiveByPhaseID     int64
	listActiveByPhaseResult []*enrollmentModels.CareOffering
	listActiveByPhaseErr    error
	getByIDID               int64
	getByIDResult           *enrollmentModels.CareOffering
	getByIDErr              error
	createInput             *enrollmentModels.CareOffering
	createResult            *enrollmentModels.CareOffering
	createErr               error
	updateInput             *enrollmentModels.CareOffering
	updateErr               error
	deleteID                int64
	deleteErr               error
	cloneSource             int64
	cloneTarget             int64
	cloneResult             *enrollmentModels.CareOffering
	cloneErr                error
}

func (m *mockCareOfferingService) List(_ context.Context) ([]*enrollmentModels.CareOffering, error) {
	return m.listResult, m.listErr
}
func (m *mockCareOfferingService) ListByPhase(_ context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	m.listByPhaseID = phaseID
	return m.listByPhaseResult, m.listByPhaseErr
}
func (m *mockCareOfferingService) ListActiveByPhase(_ context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	m.listActiveByPhaseID = phaseID
	return m.listActiveByPhaseResult, m.listActiveByPhaseErr
}
func (m *mockCareOfferingService) GetByID(_ context.Context, id int64) (*enrollmentModels.CareOffering, error) {
	m.getByIDID = id
	return m.getByIDResult, m.getByIDErr
}
func (m *mockCareOfferingService) Create(_ context.Context, o *enrollmentModels.CareOffering) (*enrollmentModels.CareOffering, error) {
	m.createInput = o
	return m.createResult, m.createErr
}
func (m *mockCareOfferingService) Update(_ context.Context, o *enrollmentModels.CareOffering) error {
	m.updateInput = o
	return m.updateErr
}
func (m *mockCareOfferingService) Delete(_ context.Context, id int64) error {
	m.deleteID = id
	return m.deleteErr
}
func (m *mockCareOfferingService) Clone(_ context.Context, sourceID int64, targetPhaseID int64) (*enrollmentModels.CareOffering, error) {
	m.cloneSource = sourceID
	m.cloneTarget = targetPhaseID
	return m.cloneResult, m.cloneErr
}

func buildCareOfferingRouter(svc enrollmentService.CareOfferingService) chi.Router {
	rs := &Resource{CareOfferingService: svc}
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/enrollment/care-offerings", rs.listCareOfferings)
	r.Get("/enrollment/care-offerings/{id}", rs.getCareOffering)
	r.Post("/enrollment/care-offerings", rs.createCareOffering)
	r.Put("/enrollment/care-offerings/{id}", rs.updateCareOffering)
	r.Delete("/enrollment/care-offerings/{id}", rs.deleteCareOffering)
	r.Post("/enrollment/care-offerings/{id}/clone", rs.cloneCareOffering)
	return r
}

func executeCareJSON(t *testing.T, router chi.Router, method, path string, body any) *httptest.ResponseRecorder {
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

func makeOfferingModel(id, phaseID int64, name string) *enrollmentModels.CareOffering {
	return &enrollmentModels.CareOffering{
		Model:          baseModel.Model{ID: id},
		PhaseID:        phaseID,
		Name:           name,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		IsActive:       true,
	}
}

func validOfferingBody(phaseID int64, name string) map[string]any {
	return map[string]any{
		"phase_id":              phaseID,
		"name":                  name,
		"days_of_week_mode":     "fixed",
		"available_days":        []string{"mon", "tue"},
		"is_active":             true,
		"sort_order":            0,
		"includes_holiday_care": false,
		"includes_lunch":        false,
	}
}

// --- listCareOfferings -----------------------------------------------

func TestListCareOfferingsHandler_NilServiceReturns500(t *testing.T) {
	router := buildCareOfferingRouter(nil)
	w := executeCareJSON(t, router, http.MethodGet, "/enrollment/care-offerings", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListCareOfferingsHandler_HappyPath(t *testing.T) {
	mock := &mockCareOfferingService{listResult: []*enrollmentModels.CareOffering{
		makeOfferingModel(1234, 5678, "OGS"),
	}}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodGet, "/enrollment/care-offerings", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":"1234"`)
}

func TestListCareOfferingsHandler_PhaseFilterParsed(t *testing.T) {
	mock := &mockCareOfferingService{listByPhaseResult: []*enrollmentModels.CareOffering{}}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodGet, "/enrollment/care-offerings?phase_id=42", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(42), mock.listByPhaseID,
		"phase_id query param must route to ListByPhase")
}

func TestListCareOfferingsHandler_InvalidPhaseIDRejected(t *testing.T) {
	router := buildCareOfferingRouter(&mockCareOfferingService{})
	w := executeCareJSON(t, router, http.MethodGet, "/enrollment/care-offerings?phase_id=notanumber", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListCareOfferingsHandler_ServiceErrorReturnsGeneric500(t *testing.T) {
	mock := &mockCareOfferingService{listErr: errors.New("synthetic boom")}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodGet, "/enrollment/care-offerings", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "synthetic boom")
}

// --- getCareOffering -------------------------------------------------

func TestGetCareOfferingHandler_NilServiceReturns500(t *testing.T) {
	router := buildCareOfferingRouter(nil)
	w := executeCareJSON(t, router, http.MethodGet, "/enrollment/care-offerings/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetCareOfferingHandler_InvalidIDRejected(t *testing.T) {
	router := buildCareOfferingRouter(&mockCareOfferingService{})
	w := executeCareJSON(t, router, http.MethodGet, "/enrollment/care-offerings/notanumber", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetCareOfferingHandler_HappyPath(t *testing.T) {
	mock := &mockCareOfferingService{getByIDResult: makeOfferingModel(1234, 5678, "OGS")}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodGet, "/enrollment/care-offerings/1234", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(1234), mock.getByIDID)
}

func TestGetCareOfferingHandler_NotFoundReturns404(t *testing.T) {
	mock := &mockCareOfferingService{getByIDErr: enrollmentService.ErrCareOfferingNotFound}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodGet, "/enrollment/care-offerings/1234", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetCareOfferingHandler_ServiceErrorReturnsGeneric500(t *testing.T) {
	mock := &mockCareOfferingService{getByIDErr: errors.New("synthetic boom")}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodGet, "/enrollment/care-offerings/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "synthetic boom")
}

// --- createCareOffering ---------------------------------------------

func TestCreateCareOfferingHandler_NilServiceReturns500(t *testing.T) {
	router := buildCareOfferingRouter(nil)
	w := executeCareJSON(t, router, http.MethodPost, "/enrollment/care-offerings",
		validOfferingBody(5678, "OGS"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreateCareOfferingHandler_HappyPath(t *testing.T) {
	mock := &mockCareOfferingService{createResult: makeOfferingModel(1234, 5678, "OGS")}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodPost, "/enrollment/care-offerings",
		validOfferingBody(5678, "OGS"))
	require.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, mock.createInput)
	assert.Equal(t, int64(5678), mock.createInput.PhaseID)
	assert.Equal(t, "OGS", mock.createInput.Name)
}

func TestCreateCareOfferingHandler_PreservesCountsAsCareFalse(t *testing.T) {
	mock := &mockCareOfferingService{createResult: makeOfferingModel(1234, 5678, "Randstunde")}
	router := buildCareOfferingRouter(mock)
	body := validOfferingBody(5678, "Randstunde")
	body["counts_as_care"] = false

	w := executeCareJSON(t, router, http.MethodPost, "/enrollment/care-offerings", body)

	require.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, mock.createInput)
	assert.False(t, mock.createInput.CountsAsCare)
	assert.True(t, mock.createInput.CountsAsCareSet)
}

func TestCreateCareOfferingHandler_ServiceErrorReturnsGeneric500(t *testing.T) {
	mock := &mockCareOfferingService{createErr: errors.New("synthetic boom")}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodPost, "/enrollment/care-offerings",
		validOfferingBody(5678, "OGS"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "synthetic boom")
}

func TestCreateCareOfferingHandler_DomainValidationReturns400(t *testing.T) {
	mock := &mockCareOfferingService{createErr: fmt.Errorf("%w: invalid linked template", enrollmentService.ErrCareOfferingInvalid)}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodPost, "/enrollment/care-offerings",
		validOfferingBody(5678, "OGS"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateCareOfferingHandler_TemplatePeriodMismatchReturnsStableCode(t *testing.T) {
	mock := &mockCareOfferingService{createErr: fmt.Errorf("validate linked template: %w", enrollmentService.ErrCareOfferingTemplatePeriodMismatch)}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodPost, "/enrollment/care-offerings",
		validOfferingBody(5678, "OGS"))

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"status":"error","error":"validate linked template: care offering phase must be within the linked timetable template period","code":"enrollment.care_offering_template_period_mismatch"}`, w.Body.String())
}

func TestCreateCareOfferingHandler_MissingDaysReturnsStableCode(t *testing.T) {
	// The service wraps the model validation via wrapCareOfferingInvalid;
	// the renderer must still resolve the specific sentinel so the admin
	// editor can show the localized "pick at least one day" message (#1885).
	mock := &mockCareOfferingService{createErr: fmt.Errorf("%w: validate care offering: %w",
		enrollmentService.ErrCareOfferingInvalid, enrollmentModels.ErrCareOfferingDaysRequired)}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodPost, "/enrollment/care-offerings",
		validOfferingBody(5678, "OGS"))

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeCareOfferingDaysRequired)
}

// --- updateCareOffering ---------------------------------------------

func TestUpdateCareOfferingHandler_NilServiceReturns500(t *testing.T) {
	router := buildCareOfferingRouter(nil)
	w := executeCareJSON(t, router, http.MethodPut, "/enrollment/care-offerings/1234",
		validOfferingBody(5678, "OGS"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUpdateCareOfferingHandler_InvalidIDRejected(t *testing.T) {
	router := buildCareOfferingRouter(&mockCareOfferingService{})
	w := executeCareJSON(t, router, http.MethodPut, "/enrollment/care-offerings/notanumber",
		validOfferingBody(5678, "OGS"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateCareOfferingHandler_HappyPathRefetches(t *testing.T) {
	mock := &mockCareOfferingService{getByIDResult: makeOfferingModel(1234, 5678, "Updated")}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodPut, "/enrollment/care-offerings/1234",
		validOfferingBody(5678, "Updated"))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, mock.updateInput)
	assert.Equal(t, int64(1234), mock.updateInput.ID, "URL id must be stamped onto the model")
	assert.Equal(t, int64(1234), mock.getByIDID, "refetch must call GetByID after the update")
}

func TestUpdateCareOfferingHandler_UpdateErrorReturnsGeneric500(t *testing.T) {
	mock := &mockCareOfferingService{updateErr: errors.New("synthetic boom")}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodPut, "/enrollment/care-offerings/1234",
		validOfferingBody(5678, "X"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "synthetic boom")
}

func TestUpdateCareOfferingHandler_TemplatePeriodMismatchReturnsStableCode(t *testing.T) {
	mock := &mockCareOfferingService{updateErr: enrollmentService.ErrCareOfferingTemplatePeriodMismatch}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodPut, "/enrollment/care-offerings/1234",
		validOfferingBody(5678, "X"))

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"enrollment.care_offering_template_period_mismatch"`)
}

func TestUpdateCareOfferingHandler_RefetchErrorReturns500(t *testing.T) {
	mock := &mockCareOfferingService{getByIDErr: errors.New("synthetic refetch boom")}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodPut, "/enrollment/care-offerings/1234",
		validOfferingBody(5678, "X"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- deleteCareOffering ---------------------------------------------

func TestDeleteCareOfferingHandler_NilServiceReturns500(t *testing.T) {
	router := buildCareOfferingRouter(nil)
	w := executeCareJSON(t, router, http.MethodDelete, "/enrollment/care-offerings/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDeleteCareOfferingHandler_InvalidIDRejected(t *testing.T) {
	router := buildCareOfferingRouter(&mockCareOfferingService{})
	w := executeCareJSON(t, router, http.MethodDelete, "/enrollment/care-offerings/notanumber", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteCareOfferingHandler_HappyPathReturns204(t *testing.T) {
	mock := &mockCareOfferingService{}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodDelete, "/enrollment/care-offerings/1234", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, int64(1234), mock.deleteID)
}

func TestDeleteCareOfferingHandler_FKViolationReturns400(t *testing.T) {
	// PG FK violation when request_child_offerings already references
	// this offering. Admin should see the message + soft-delete instead.
	mock := &mockCareOfferingService{deleteErr: errors.New(`violates foreign key constraint "fk_test"`)}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodDelete, "/enrollment/care-offerings/1234", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"enrollment.care_offering_in_use"`)
	assert.NotContains(t, w.Body.String(), "fk_test")
}

// --- cloneCareOffering ----------------------------------------------

func TestCloneCareOfferingHandler_NilServiceReturns500(t *testing.T) {
	router := buildCareOfferingRouter(nil)
	w := executeCareJSON(t, router, http.MethodPost, "/enrollment/care-offerings/1234/clone",
		map[string]any{"target_phase_id": 5678})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCloneCareOfferingHandler_InvalidIDRejected(t *testing.T) {
	router := buildCareOfferingRouter(&mockCareOfferingService{})
	w := executeCareJSON(t, router, http.MethodPost, "/enrollment/care-offerings/notanumber/clone",
		map[string]any{"target_phase_id": 5678})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCloneCareOfferingHandler_HappyPath(t *testing.T) {
	mock := &mockCareOfferingService{cloneResult: makeOfferingModel(9999, 5678, "Clone")}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodPost, "/enrollment/care-offerings/1234/clone",
		map[string]any{"target_phase_id": 5678})
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, int64(1234), mock.cloneSource, "source id from URL must be forwarded")
	assert.Equal(t, int64(5678), mock.cloneTarget, "target phase from body must be forwarded")
}

func TestCloneCareOfferingHandler_ServiceErrorReturnsGeneric500(t *testing.T) {
	mock := &mockCareOfferingService{cloneErr: errors.New("synthetic boom")}
	router := buildCareOfferingRouter(mock)
	w := executeCareJSON(t, router, http.MethodPost, "/enrollment/care-offerings/1234/clone",
		map[string]any{"target_phase_id": 5678})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "synthetic boom")
}
