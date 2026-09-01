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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type mockPhaseService struct {
	listResult        []*enrollmentModels.Phase
	listErr           error
	listPublicOpenRes []*enrollmentModels.Phase
	listPublicOpenErr error
	getByIDCalls      int
	getByIDID         int64
	getByIDResults    []*enrollmentModels.Phase
	getByIDErrs       []error
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
	m.getByIDCalls++
	m.getByIDID = id
	idx := m.getByIDCalls - 1
	if idx < len(m.getByIDErrs) && m.getByIDErrs[idx] != nil {
		return nil, m.getByIDErrs[idx]
	}
	if idx < len(m.getByIDResults) && m.getByIDResults[idx] != nil {
		return m.getByIDResults[idx], nil
	}
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
	req = req.WithContext(testpkg.WithPackageTenantRuntime(req.Context()))
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
	t.Parallel()

	router := buildPhaseRouter(nil)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListPhasesHandler_HappyPath(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	mock := &mockPhaseService{listErr: errors.New("synthetic boom")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListPublicPhasesHandler_DoesNotLeakOtherTenantPhases(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	now := time.Now().UnixNano()
	ctx := context.Background()
	org := &platformModels.Organization{
		Model:  baseModel.Model{ID: now},
		Name:   "Public Phase Tenant Scope Org",
		Slug:   fmt.Sprintf("public-phase-tenant-scope-org-%d", now),
		Active: true,
	}
	testpkg.CreateTestOrganization(t, db, org)

	targetSchool := &platformModels.School{
		Model:          baseModel.Model{ID: now + 100},
		OrganizationID: org.ID,
		Name:           "Target Public Phase School",
		Slug:           fmt.Sprintf("target-public-phase-school-%d", now),
		Subdomain:      fmt.Sprintf("target-public-phase-school-%d", now),
		Active:         true,
	}
	otherSchool := &platformModels.School{
		Model:          baseModel.Model{ID: now + 200},
		OrganizationID: org.ID,
		Name:           "Other Public Phase School",
		Slug:           fmt.Sprintf("other-public-phase-school-%d", now),
		Subdomain:      fmt.Sprintf("other-public-phase-school-%d", now),
		Active:         true,
	}
	schoolRepo := platformRepo.NewSchoolRepository(db)
	require.NoError(t, schoolRepo.Create(ctx, targetSchool))
	require.NoError(t, schoolRepo.Create(ctx, otherSchool))
	t.Cleanup(func() {
		_, _ = db.NewRaw(`DELETE FROM enrollment.phases WHERE tenant_id IN (?, ?)`, targetSchool.ID, otherSchool.ID).Exec(context.Background())
		_, _ = db.NewRaw(`DELETE FROM platform.schools WHERE id IN (?, ?)`, targetSchool.ID, otherSchool.ID).Exec(context.Background())
		_, _ = db.NewRaw(`DELETE FROM platform.organizations WHERE id = ?`, org.ID).Exec(context.Background())
	})

	phaseRepo := enrollmentRepo.NewPhaseRepository(db)
	targetPhase := makePhaseModel(now+300, "Target Tenant Phase")
	otherPhase := makePhaseModel(now+400, "Other Tenant Phase")
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, targetSchool.ID, func(txCtx context.Context, _ bun.Tx) error {
		return phaseRepo.Create(txCtx, targetPhase)
	}))
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, otherSchool.ID, func(txCtx context.Context, _ bun.Tx) error {
		return phaseRepo.Create(txCtx, otherPhase)
	}))

	rs := &Resource{
		SchoolService: platformSvc.NewSchoolService(schoolRepo),
		PhaseService: enrollmentService.NewPhaseService(enrollmentService.PhaseServiceConfig{
			Repo: phaseRepo,
		}),
		db: db,
	}
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Get("/enrollment/phases/public/{tenantSlug}", rs.listPublicPhases)

	w := executePhaseJSON(t, router, http.MethodGet, fmt.Sprintf("/enrollment/phases/public/%s", targetSchool.Slug), nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"name":"Target Tenant Phase"`)
	assert.NotContains(t, w.Body.String(), `"name":"Other Tenant Phase"`)
}

// --- getPhase ---------------------------------------------------------

func TestGetPhaseHandler_NilServiceReturns500(t *testing.T) {
	t.Parallel()

	router := buildPhaseRouter(nil)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetPhaseHandler_InvalidIDRejected(t *testing.T) {
	t.Parallel()

	router := buildPhaseRouter(&mockPhaseService{})
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases/notanumber", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetPhaseHandler_HappyPath(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{getByIDResult: makePhaseModel(1234, "Schuljahr 2026")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases/1234", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(1234), mock.getByIDID)
	assert.Contains(t, w.Body.String(), `"id":"1234"`)
}

func TestGetPhaseHandler_NotFoundReturns404(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{getByIDErr: enrollmentService.ErrPhaseNotFound}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetPhaseHandler_GenericErrorReturns500(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{getByIDErr: errors.New("synthetic boom")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- createPhase ------------------------------------------------------

func TestCreatePhaseHandler_NilServiceReturns500(t *testing.T) {
	t.Parallel()

	router := buildPhaseRouter(nil)
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", validPhaseBody("X"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreatePhaseHandler_HappyPathReturns201(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	router := buildPhaseRouter(&mockPhaseService{})
	body := validPhaseBody("X")
	body["service_start_date"] = "yesterday"
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePhaseHandler_BadServiceEndDateReturns400(t *testing.T) {
	t.Parallel()

	router := buildPhaseRouter(&mockPhaseService{})
	body := validPhaseBody("X")
	body["service_end_date"] = "tomorrow"
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePhaseHandler_BadEnrollmentWindowReturns400(t *testing.T) {
	t.Parallel()

	router := buildPhaseRouter(&mockPhaseService{})
	body := validPhaseBody("X")
	body["enrollment_open_at"] = "not-iso"
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePhaseHandler_BadFormSchemaIDReturns400(t *testing.T) {
	t.Parallel()

	router := buildPhaseRouter(&mockPhaseService{})
	body := validPhaseBody("X")
	body["form_schema_id"] = "not-a-number"
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePhaseHandler_InvalidPhaseReturns400(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{createErr: enrollmentService.ErrInvalidPhase}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", validPhaseBody("X"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePhaseHandler_DuplicateNameReturns409(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{createErr: enrollmentService.ErrPhaseDuplicateName}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", validPhaseBody("X"))
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCreatePhaseHandler_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{createErr: errors.New("synthetic boom")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", validPhaseBody("X"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- updatePhase ------------------------------------------------------

func TestUpdatePhaseHandler_NilServiceReturns500(t *testing.T) {
	t.Parallel()

	router := buildPhaseRouter(nil)
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", validPhaseBody("X"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUpdatePhaseHandler_InvalidIDRejected(t *testing.T) {
	t.Parallel()

	router := buildPhaseRouter(&mockPhaseService{})
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/notanumber", validPhaseBody("X"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdatePhaseHandler_HappyPathRefetchesAfterUpdate(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{getByIDResult: makePhaseModel(1234, "Updated")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", validPhaseBody("Updated"))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, mock.updateInput)
	assert.Equal(t, int64(1234), mock.updateInput.ID, "ID from URL must be stamped onto the model")
	assert.Equal(t, int64(1234), mock.getByIDID, "refetch must call GetByID after the update")
}

func TestUpdatePhaseHandler_OmittedCalendarPeriodIDPreservesExistingLink(t *testing.T) {
	t.Parallel()

	existing := makePhaseModel(1234, "Existing")
	periodID := int64(42)
	existing.CalendarPeriodID = &periodID
	mock := &mockPhaseService{getByIDResult: existing}
	router := buildPhaseRouter(mock)

	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", validPhaseBody("Updated"))

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, mock.updateInput)
	require.NotNil(t, mock.updateInput.CalendarPeriodID)
	assert.Equal(t, periodID, *mock.updateInput.CalendarPeriodID)
}

func TestUpdatePhaseHandler_NullCalendarPeriodIDUnlinks(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{getByIDResult: makePhaseModel(1234, "Updated")}
	router := buildPhaseRouter(mock)
	body := validPhaseBody("Updated")
	body["calendar_period_id"] = nil

	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", body)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, mock.updateInput)
	assert.Nil(t, mock.updateInput.CalendarPeriodID)
}

func TestUpdatePhaseHandler_NullCalendarPeriodIDMissingPhaseReturns404(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{getByIDErr: enrollmentService.ErrPhaseNotFound}
	router := buildPhaseRouter(mock)
	body := validPhaseBody("Updated")
	body["calendar_period_id"] = nil

	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", body)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Nil(t, mock.updateInput, "stale update must fail before PhaseService.Update")
}

func TestUpdatePhaseHandler_BadDateReturns400(t *testing.T) {
	t.Parallel()

	router := buildPhaseRouter(&mockPhaseService{})
	body := validPhaseBody("X")
	body["service_start_date"] = "yesterday"
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdatePhaseHandler_InvalidPhaseReturns400(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{
		getByIDResult: makePhaseModel(1234, "Updated"),
		updateErr:     enrollmentService.ErrInvalidPhase,
	}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", validPhaseBody("X"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdatePhaseHandler_DuplicateNameReturns409(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{
		getByIDResult: makePhaseModel(1234, "Updated"),
		updateErr:     enrollmentService.ErrPhaseDuplicateName,
	}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", validPhaseBody("X"))
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestUpdatePhaseHandler_CareOfferingConflictReturns409(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{
		getByIDResult: makePhaseModel(1234, "Updated"),
		updateErr:     enrollmentService.ErrPhaseCareOfferingConflict,
	}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", validPhaseBody("X"))
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestUpdatePhaseHandler_UpdateErrorReturns500(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{
		getByIDResult: makePhaseModel(1234, "Updated"),
		updateErr:     errors.New("synthetic boom"),
	}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", validPhaseBody("X"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// A pre-#1833 client omits the concrete-class fields entirely. Because
// Update replaces the whole row, the handler must re-hydrate the omitted
// fields from the stored phase so a partial update never wipes an admin's
// class pick list / mandatory toggle.
func TestUpdatePhaseHandler_OmittedClassConfigPreservesExisting(t *testing.T) {
	t.Parallel()

	existing := makePhaseModel(1234, "Updated")
	existing.AvailableSchoolClasses = []string{"2a", "2b"}
	existing.RequireSchoolClass = true
	mock := &mockPhaseService{getByIDResult: existing}
	router := buildPhaseRouter(mock)

	// validPhaseBody carries no available_school_classes / require_school_class.
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", validPhaseBody("Updated"))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, mock.updateInput)
	assert.Equal(t, []string{"2a", "2b"}, mock.updateInput.AvailableSchoolClasses,
		"omitted class list must be preserved from the stored phase")
	assert.True(t, mock.updateInput.RequireSchoolClass,
		"omitted require flag must be preserved from the stored phase")
}

// An up-to-date client that explicitly sends the class config must have it
// applied verbatim — including clearing the list / turning the flag off.
func TestUpdatePhaseHandler_ExplicitClassConfigApplied(t *testing.T) {
	t.Parallel()

	existing := makePhaseModel(1234, "Updated")
	existing.AvailableSchoolClasses = []string{"2a", "2b"}
	existing.RequireSchoolClass = true
	mock := &mockPhaseService{getByIDResult: existing}
	router := buildPhaseRouter(mock)

	body := validPhaseBody("Updated")
	body["available_school_classes"] = []string{"3a"}
	body["require_school_class"] = false
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", body)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, mock.updateInput)
	assert.Equal(t, []string{"3a"}, mock.updateInput.AvailableSchoolClasses,
		"provided class list must overwrite the stored one")
	assert.False(t, mock.updateInput.RequireSchoolClass,
		"provided require=false must overwrite the stored true")
}

// An explicit empty list is a deliberate clear, not an omission, and must be
// applied rather than re-hydrated from the stored phase.
func TestUpdatePhaseHandler_ExplicitEmptyClassListClears(t *testing.T) {
	t.Parallel()

	existing := makePhaseModel(1234, "Updated")
	existing.AvailableSchoolClasses = []string{"2a", "2b"}
	mock := &mockPhaseService{getByIDResult: existing}
	router := buildPhaseRouter(mock)

	body := validPhaseBody("Updated")
	body["available_school_classes"] = []string{}
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", body)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, mock.updateInput)
	assert.Empty(t, mock.updateInput.AvailableSchoolClasses,
		"an explicit empty list must clear the stored classes")
}

func TestCreatePhaseHandler_ClassConfigApplied(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{createResult: makePhaseModel(1234, "Schuljahr 2026")}
	router := buildPhaseRouter(mock)
	body := validPhaseBody("Schuljahr 2026")
	body["available_school_classes"] = []string{"2a", " 2a ", "2b", ""}
	body["require_school_class"] = true
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", body)
	require.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, mock.createInput)
	assert.Equal(t, []string{"2a", "2b"}, mock.createInput.AvailableSchoolClasses,
		"class list must be trimmed, de-duped, and emptied of blanks")
	assert.True(t, mock.createInput.RequireSchoolClass)
}

func TestUpdatePhaseHandler_RefetchErrorReturns500(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{
		getByIDResults: []*enrollmentModels.Phase{makePhaseModel(1234, "Updated")},
		getByIDErrs:    []error{nil, errors.New("synthetic refetch boom")},
	}
	router := buildPhaseRouter(mock)
	body := validPhaseBody("X")
	body["calendar_period_id"] = "42"
	w := executePhaseJSON(t, router, http.MethodPut, "/enrollment/phases/1234", body)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, 2, mock.getByIDCalls)
}

// --- deletePhase ------------------------------------------------------

func TestDeletePhaseHandler_NilServiceReturns500(t *testing.T) {
	t.Parallel()

	router := buildPhaseRouter(nil)
	w := executePhaseJSON(t, router, http.MethodDelete, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDeletePhaseHandler_InvalidIDRejected(t *testing.T) {
	t.Parallel()

	router := buildPhaseRouter(&mockPhaseService{})
	w := executePhaseJSON(t, router, http.MethodDelete, "/enrollment/phases/notanumber", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeletePhaseHandler_HappyPathReturns204(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodDelete, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, int64(1234), mock.deleteID)
}

func TestDeletePhaseHandler_NotFoundReturns404(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{deleteErr: enrollmentService.ErrPhaseNotFound}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodDelete, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeletePhaseHandler_GenericErrorReturns500(t *testing.T) {
	t.Parallel()

	mock := &mockPhaseService{deleteErr: errors.New("synthetic boom")}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodDelete, "/enrollment/phases/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getPhaseDeleteImpact ---------------------------------------------

func TestPhaseDeleteImpactHandler_ReturnsCounts(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	mock := &mockPhaseService{deleteImpactErr: enrollmentService.ErrPhaseNotFound}
	router := buildPhaseRouter(mock)
	w := executePhaseJSON(t, router, http.MethodGet, "/enrollment/phases/1234/delete-impact", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreatePhaseHandler_CalendarPeriodIDRoundtrip(t *testing.T) {
	t.Parallel()

	created := makePhaseModel(1234, "Schuljahr 2026")
	periodID := int64(42)
	created.CalendarPeriodID = &periodID
	mock := &mockPhaseService{createResult: created}
	router := buildPhaseRouter(mock)

	body := validPhaseBody("Schuljahr 2026")
	body["calendar_period_id"] = "42"
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", body)
	require.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, mock.createInput)
	require.NotNil(t, mock.createInput.CalendarPeriodID)
	assert.Equal(t, int64(42), *mock.createInput.CalendarPeriodID)

	var envelope struct {
		Data PhaseResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.NotNil(t, envelope.Data.CalendarPeriodID)
	assert.Equal(t, "42", *envelope.Data.CalendarPeriodID)
}

func TestCreatePhaseHandler_BadCalendarPeriodIDReturns400(t *testing.T) {
	t.Parallel()

	router := buildPhaseRouter(&mockPhaseService{})
	body := validPhaseBody("X")
	body["calendar_period_id"] = "not-a-number"
	w := executePhaseJSON(t, router, http.MethodPost, "/enrollment/phases", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
