package enrollment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

func TestCountAssignedPreviewPhases(t *testing.T) {
	schemaID := int64(1234)
	otherSchemaID := int64(5678)
	phases := []*enrollmentModels.Phase{
		{FormSchemaID: &schemaID, IsActive: true},
		{FormSchemaID: &schemaID, IsActive: false},
		{FormSchemaID: &otherSchemaID, IsActive: true},
		{FormSchemaID: nil, IsActive: true},
		nil,
	}

	assigned, active := countAssignedPreviewPhases(phases, &enrollmentModels.FormSchema{
		Model: baseModel.Model{ID: schemaID},
	})
	require.Equal(t, 2, assigned)
	require.Equal(t, 1, active)

	baseAssigned, baseActive := countAssignedPreviewPhases(phases, nil)
	require.Equal(t, 1, baseAssigned)
	require.Equal(t, 1, baseActive)
}

// mockFormSchemaService records inputs + replays outputs. Same pattern
// as mockRolloverService / mockDecisionService in this package.
type mockFormSchemaService struct {
	getActiveErr     error
	getActiveResult  *enrollmentModels.FormSchema
	getByIDID        int64
	getByIDErr       error
	getByIDResult    *enrollmentModels.FormSchema
	listResult       []*enrollmentModels.FormSchema
	listErr          error
	createName       string
	createFields     []enrollmentModels.FormField
	createCreatedBy  int64
	createCore       enrollmentModels.CoreRequirements
	createCoreSet    bool
	createLegal      []enrollmentModels.FormLegalBlock
	createLegalSet   bool
	createResult     *enrollmentModels.FormSchema
	createErr        error
	updateID         int64
	updateFields     []enrollmentModels.FormField
	updateUpdatedBy  int64
	updateCore       enrollmentModels.CoreRequirements
	updateCoreSet    bool
	updateLegal      []enrollmentModels.FormLegalBlock
	updateLegalSet   bool
	updateResult     *enrollmentModels.FormSchema
	updateErr        error
	deleteID         int64
	deleteErr        error
	publishVResult   *enrollmentModels.FormSchema
	publishVErr      error
	publishVFields   []enrollmentModels.FormField
	publishVCreated  int64
	validateSchemaID int64
	validateData     enrollmentModels.SubmissionData
	validateErr      error
}

func (m *mockFormSchemaService) GetActive(_ context.Context) (*enrollmentModels.FormSchema, error) {
	return m.getActiveResult, m.getActiveErr
}
func (m *mockFormSchemaService) GetByID(_ context.Context, id int64) (*enrollmentModels.FormSchema, error) {
	m.getByIDID = id
	return m.getByIDResult, m.getByIDErr
}
func (m *mockFormSchemaService) ListVersions(_ context.Context) ([]*enrollmentModels.FormSchema, error) {
	return m.listResult, m.listErr
}
func (m *mockFormSchemaService) CreateSchema(_ context.Context, name string, fields []enrollmentModels.FormField, createdBy int64, coreRequirements ...enrollmentModels.CoreRequirements) (*enrollmentModels.FormSchema, error) {
	m.createName = name
	m.createFields = fields
	m.createCreatedBy = createdBy
	if len(coreRequirements) > 0 {
		m.createCore = coreRequirements[0]
		m.createCoreSet = true
	}
	return m.createResult, m.createErr
}
func (m *mockFormSchemaService) CreateSchemaWithLegal(_ context.Context, name string, fields []enrollmentModels.FormField, createdBy int64, coreRequirements enrollmentModels.CoreRequirements, legalBlocks []enrollmentModels.FormLegalBlock) (*enrollmentModels.FormSchema, error) {
	m.createName = name
	m.createFields = fields
	m.createCreatedBy = createdBy
	m.createCore = coreRequirements
	m.createCoreSet = true
	m.createLegal = legalBlocks
	m.createLegalSet = true
	return m.createResult, m.createErr
}
func (m *mockFormSchemaService) UpdateSchema(_ context.Context, id int64, fields []enrollmentModels.FormField, updatedBy int64, coreRequirements ...enrollmentModels.CoreRequirements) (*enrollmentModels.FormSchema, error) {
	m.updateID = id
	m.updateFields = fields
	m.updateUpdatedBy = updatedBy
	if len(coreRequirements) > 0 {
		m.updateCore = coreRequirements[0]
		m.updateCoreSet = true
	}
	return m.updateResult, m.updateErr
}
func (m *mockFormSchemaService) UpdateSchemaWithLegal(_ context.Context, id int64, fields []enrollmentModels.FormField, updatedBy int64, coreRequirements *enrollmentModels.CoreRequirements, legalBlocks *[]enrollmentModels.FormLegalBlock) (*enrollmentModels.FormSchema, error) {
	m.updateID = id
	m.updateFields = fields
	m.updateUpdatedBy = updatedBy
	if coreRequirements != nil {
		m.updateCore = *coreRequirements
		m.updateCoreSet = true
	}
	if legalBlocks != nil {
		m.updateLegal = *legalBlocks
		m.updateLegalSet = true
	}
	return m.updateResult, m.updateErr
}
func (m *mockFormSchemaService) DeleteSchema(_ context.Context, id int64) error {
	m.deleteID = id
	return m.deleteErr
}
func (m *mockFormSchemaService) PublishVersion(_ context.Context, fields []enrollmentModels.FormField, createdBy int64, _ ...enrollmentModels.CoreRequirements) (*enrollmentModels.FormSchema, error) {
	m.publishVFields = fields
	m.publishVCreated = createdBy
	return m.publishVResult, m.publishVErr
}
func (m *mockFormSchemaService) ValidateSubmission(_ context.Context, schemaID int64, data enrollmentModels.SubmissionData) error {
	m.validateSchemaID = schemaID
	m.validateData = data
	return m.validateErr
}

// buildSchemaRouter wires the schema endpoints with a mock service.
// db is nil so runInTenantTx short-circuits to the closure body.
func buildSchemaRouter(svc enrollmentService.FormSchemaService) chi.Router {
	rs := &Resource{FormSchemaService: svc}
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/enrollment/schema", rs.getActiveSchema)
	r.Get("/enrollment/schema/versions", rs.listSchemaVersions)
	r.Get("/enrollment/schema/{id}", rs.getSchemaByID)
	r.Post("/enrollment/schema", rs.publishSchema)
	r.Put("/enrollment/schema/{id}", rs.updateSchema)
	r.Delete("/enrollment/schema/{id}", rs.deleteSchema)
	return r
}

// executeSchemaJSON wraps a request in a context with auth claims so
// the publish/update handlers find an actor. Claims.ID > 0 satisfies
// the "missing actor" check.
func executeSchemaJSON(t *testing.T, router chi.Router, method, path string, body any) *httptest.ResponseRecorder {
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
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 4321})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func makeFormSchema(id int64, name string, version int) *enrollmentModels.FormSchema {
	return &enrollmentModels.FormSchema{
		Model:     baseModel.Model{ID: id},
		Name:      name,
		Version:   version,
		IsActive:  true,
		CreatedBy: 4321,
		Fields: []enrollmentModels.FormField{
			{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
		},
	}
}

// --- getActiveSchema ---------------------------------------------------

func TestGetActiveSchemaHandler_NilServiceReturns500(t *testing.T) {
	router := buildSchemaRouter(nil)
	w := executeSchemaJSON(t, router, http.MethodGet, "/enrollment/schema", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetActiveSchemaHandler_HappyPath(t *testing.T) {
	mock := &mockFormSchemaService{getActiveResult: makeFormSchema(1234, "Standardformular", 2)}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodGet, "/enrollment/schema", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":"1234"`)
	assert.Contains(t, w.Body.String(), `"version":2`)
}

func TestGetActiveSchemaHandler_NoActiveSchemaReturns404(t *testing.T) {
	mock := &mockFormSchemaService{getActiveErr: enrollmentService.ErrNoActiveSchema}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodGet, "/enrollment/schema", nil)
	assert.Equal(t, http.StatusNotFound, w.Code,
		"ErrNoActiveSchema → 404 so the frontend can render the empty editor")
}

func TestGetActiveSchemaHandler_GenericErrorReturns500(t *testing.T) {
	mock := &mockFormSchemaService{getActiveErr: errors.New("synthetic boom")}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodGet, "/enrollment/schema", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- listSchemaVersions ------------------------------------------------

func TestListSchemaVersionsHandler_NilServiceReturns500(t *testing.T) {
	router := buildSchemaRouter(nil)
	w := executeSchemaJSON(t, router, http.MethodGet, "/enrollment/schema/versions", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListSchemaVersionsHandler_HappyPath(t *testing.T) {
	mock := &mockFormSchemaService{listResult: []*enrollmentModels.FormSchema{
		makeFormSchema(2222, "A", 3),
		makeFormSchema(1111, "A", 2),
		makeFormSchema(999, "A", 1),
	}}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodGet, "/enrollment/schema/versions", nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"id":"2222"`)
	assert.Contains(t, body, `"id":"1111"`)
	assert.Contains(t, body, `"id":"999"`)
}

func TestListSchemaVersionsHandler_ServiceErrorReturns500(t *testing.T) {
	mock := &mockFormSchemaService{listErr: errors.New("synthetic boom")}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodGet, "/enrollment/schema/versions", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getSchemaByID -----------------------------------------------------

func TestGetSchemaByIDHandler_NilServiceReturns500(t *testing.T) {
	router := buildSchemaRouter(nil)
	w := executeSchemaJSON(t, router, http.MethodGet, "/enrollment/schema/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetSchemaByIDHandler_InvalidIDRejected(t *testing.T) {
	router := buildSchemaRouter(&mockFormSchemaService{})
	w := executeSchemaJSON(t, router, http.MethodGet, "/enrollment/schema/notanumber", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetSchemaByIDHandler_ZeroIDRejected(t *testing.T) {
	router := buildSchemaRouter(&mockFormSchemaService{})
	w := executeSchemaJSON(t, router, http.MethodGet, "/enrollment/schema/0", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetSchemaByIDHandler_HappyPath(t *testing.T) {
	mock := &mockFormSchemaService{getByIDResult: makeFormSchema(1234, "Standardformular", 1)}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodGet, "/enrollment/schema/1234", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(1234), mock.getByIDID, "URL id must be passed to GetByID verbatim")
	assert.Contains(t, w.Body.String(), `"id":"1234"`)
}

func TestGetSchemaByIDHandler_ServiceErrorMappedAs404(t *testing.T) {
	// Schema-by-id misses are rare and we already verified the id is
	// numeric — so any service error here is "not found".
	mock := &mockFormSchemaService{getByIDErr: errors.New("synthetic boom")}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodGet, "/enrollment/schema/1234", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- publishSchema -----------------------------------------------------

func TestPublishSchemaHandler_NilServiceReturns500(t *testing.T) {
	router := buildSchemaRouter(nil)
	w := executeSchemaJSON(t, router, http.MethodPost, "/enrollment/schema",
		map[string]any{"fields": []map[string]any{}})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPublishSchemaHandler_RejectsMissingActor(t *testing.T) {
	// Hit the "missing actor" branch by sending a request without
	// claims in context.
	router := buildSchemaRouter(&mockFormSchemaService{})
	raw, _ := json.Marshal(map[string]any{"fields": []map[string]any{}})
	req := httptest.NewRequest(http.MethodPost, "/enrollment/schema", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPublishSchemaHandler_WithNameCallsCreateSchema(t *testing.T) {
	mock := &mockFormSchemaService{createResult: makeFormSchema(1234, "Klassenanmeldung", 1)}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodPost, "/enrollment/schema",
		map[string]any{
			"name": "Klassenanmeldung",
			"fields": []map[string]any{
				{"key": "allergies", "label": "Allergien", "type": "text", "sort_order": 0},
			},
		})
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "Klassenanmeldung", mock.createName,
		"non-empty name MUST route to CreateSchema, not PublishVersion")
	assert.Equal(t, int64(4321), mock.createCreatedBy)
	assert.Len(t, mock.createFields, 1)
}

func TestPublishSchemaHandler_WithLegalBlocksCallsCreateSchemaWithLegal(t *testing.T) {
	mock := &mockFormSchemaService{createResult: makeFormSchema(1234, "Klassenanmeldung", 1)}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodPost, "/enrollment/schema",
		map[string]any{
			"name": "Klassenanmeldung",
			"fields": []map[string]any{
				{"key": "allergies", "label": "Allergien", "type": "text", "sort_order": 0},
			},
			"legal_blocks": []map[string]any{
				{
					"key":        "custom_pool",
					"kind":       "consent",
					"title":      "Schwimmbad",
					"label":      "Mein Kind darf teilnehmen.",
					"text":       "Details",
					"required":   true,
					"enabled":    true,
					"sort_order": 10,
					"source":     "custom",
				},
			},
		})
	require.Equal(t, http.StatusCreated, w.Code)
	assert.True(t, mock.createLegalSet)
	require.Len(t, mock.createLegal, 1)
	assert.Equal(t, "custom_pool", mock.createLegal[0].Key)
	assert.True(t, mock.createLegal[0].Required)
}

func TestPublishSchemaHandler_NoNameUsesStandardformularFallback(t *testing.T) {
	// Without a name in the body, the handler's legacy path looks for
	// the active schema; ErrNoActiveSchema → falls through to creating
	// a "Standardformular" row.
	mock := &mockFormSchemaService{
		getActiveErr: enrollmentService.ErrNoActiveSchema,
		listResult:   []*enrollmentModels.FormSchema{},
		createResult: makeFormSchema(1234, "Standardformular", 1),
	}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodPost, "/enrollment/schema",
		map[string]any{"fields": []map[string]any{
			{"key": "allergies", "label": "Allergien", "type": "text", "sort_order": 0},
		}})
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "Standardformular", mock.createName,
		"no name + no active schema + no existing Standardformular → CreateSchema with the default name")
}

func TestPublishSchemaHandler_NoNameWithActiveSchemaUpdatesIt(t *testing.T) {
	// No name + an active schema exists → UpdateSchema on that schema's
	// id (legacy single-schema flow).
	mock := &mockFormSchemaService{
		getActiveResult: makeFormSchema(9999, "Standardformular", 2),
		updateResult:    makeFormSchema(9999, "Standardformular", 3),
	}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodPost, "/enrollment/schema",
		map[string]any{"fields": []map[string]any{
			{"key": "diet", "label": "Diät", "type": "text", "sort_order": 0},
		}})
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, int64(9999), mock.updateID,
		"no name + active schema exists → UpdateSchema on active.id")
	assert.False(t, mock.updateCoreSet,
		"omitted core_requirements must preserve the source schema's existing core requirements")
}

func TestPublishSchemaHandler_ValidationErrorReturns400(t *testing.T) {
	mock := &mockFormSchemaService{createErr: errors.New("field 0: form field key is required")}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodPost, "/enrollment/schema",
		map[string]any{"name": "X", "fields": []map[string]any{}})
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"service validation errors land as 400")
}

// --- updateSchema -----------------------------------------------------

func TestUpdateSchemaHandler_NilServiceReturns500(t *testing.T) {
	router := buildSchemaRouter(nil)
	w := executeSchemaJSON(t, router, http.MethodPut, "/enrollment/schema/1234",
		map[string]any{"fields": []map[string]any{}})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUpdateSchemaHandler_InvalidIDRejected(t *testing.T) {
	router := buildSchemaRouter(&mockFormSchemaService{})
	w := executeSchemaJSON(t, router, http.MethodPut, "/enrollment/schema/notanumber",
		map[string]any{"fields": []map[string]any{}})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateSchemaHandler_MissingActorRejected(t *testing.T) {
	router := buildSchemaRouter(&mockFormSchemaService{})
	raw, _ := json.Marshal(map[string]any{"fields": []map[string]any{}})
	req := httptest.NewRequest(http.MethodPut, "/enrollment/schema/1234", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateSchemaHandler_HappyPath(t *testing.T) {
	mock := &mockFormSchemaService{updateResult: makeFormSchema(1234, "Klassenanmeldung", 2)}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodPut, "/enrollment/schema/1234",
		map[string]any{"fields": []map[string]any{
			{"key": "x", "label": "X", "type": "text", "sort_order": 0},
		}})
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, int64(1234), mock.updateID)
	assert.Equal(t, int64(4321), mock.updateUpdatedBy)
	assert.Len(t, mock.updateFields, 1)
}

func TestUpdateSchemaHandler_ForwardsCoreRequirementsWhenPresent(t *testing.T) {
	mock := &mockFormSchemaService{updateResult: makeFormSchema(1234, "Klassenanmeldung", 2)}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodPut, "/enrollment/schema/1234",
		map[string]any{
			"fields":            []map[string]any{{"key": "x", "label": "X", "type": "text", "sort_order": 0}},
			"core_requirements": map[string]bool{"guardian_phone": true},
		})
	require.Equal(t, http.StatusCreated, w.Code)
	require.True(t, mock.updateCoreSet,
		"present core_requirements, even when sparse, must be forwarded so admins can change the matrix")
	assert.True(t, mock.updateCore.Required(enrollmentModels.CoreRequirementGuardianPhone))
}

func TestUpdateSchemaHandler_ForwardsLegalBlocksWhenPresent(t *testing.T) {
	mock := &mockFormSchemaService{updateResult: makeFormSchema(1234, "Klassenanmeldung", 2)}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodPut, "/enrollment/schema/1234",
		map[string]any{
			"fields": []map[string]any{{"key": "x", "label": "X", "type": "text", "sort_order": 0}},
			"legal_blocks": []map[string]any{
				{
					"key":        "custom_pool",
					"kind":       "consent",
					"title":      "Schwimmbad",
					"label":      "Mein Kind darf teilnehmen.",
					"text":       "Details",
					"required":   false,
					"enabled":    true,
					"sort_order": 10,
					"source":     "custom",
				},
			},
		})
	require.Equal(t, http.StatusCreated, w.Code)
	require.True(t, mock.updateLegalSet)
	require.Len(t, mock.updateLegal, 1)
	assert.Equal(t, "custom_pool", mock.updateLegal[0].Key)
}

func TestUpdateSchemaHandler_ServiceErrorReturns400(t *testing.T) {
	mock := &mockFormSchemaService{updateErr: errors.New("invalid schema")}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodPut, "/enrollment/schema/1234",
		map[string]any{"fields": []map[string]any{}})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- deleteSchema -----------------------------------------------------

func TestDeleteSchemaHandler_NilServiceReturns500(t *testing.T) {
	router := buildSchemaRouter(nil)
	w := executeSchemaJSON(t, router, http.MethodDelete, "/enrollment/schema/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDeleteSchemaHandler_InvalidIDRejected(t *testing.T) {
	router := buildSchemaRouter(&mockFormSchemaService{})
	w := executeSchemaJSON(t, router, http.MethodDelete, "/enrollment/schema/notanumber", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteSchemaHandler_HappyPathReturns204(t *testing.T) {
	mock := &mockFormSchemaService{}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodDelete, "/enrollment/schema/1234", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, int64(1234), mock.deleteID)
}

func TestDeleteSchemaHandler_HasPhases409WithCode(t *testing.T) {
	mock := &mockFormSchemaService{deleteErr: enrollmentService.ErrFormSchemaHasPhases}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodDelete, "/enrollment/schema/1234", nil)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeSchemaHasPhases,
		"ErrFormSchemaHasPhases must surface as a stable error code so the frontend can render the friendly message")
}

func TestDeleteSchemaHandler_HasRequests409WithCode(t *testing.T) {
	mock := &mockFormSchemaService{deleteErr: enrollmentService.ErrFormSchemaHasRequests}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodDelete, "/enrollment/schema/1234", nil)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeSchemaHasRequests)
}

func TestDeleteSchemaHandler_NotFoundReturns404(t *testing.T) {
	mock := &mockFormSchemaService{deleteErr: enrollmentService.ErrFormSchemaNotFound}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodDelete, "/enrollment/schema/1234", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteSchemaHandler_GenericErrorReturns500(t *testing.T) {
	mock := &mockFormSchemaService{deleteErr: errors.New("synthetic boom")}
	router := buildSchemaRouter(mock)
	w := executeSchemaJSON(t, router, http.MethodDelete, "/enrollment/schema/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
