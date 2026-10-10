package enrollmenthttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Every refusal the Anmeldung screens can meet answers with a registered code,
// and a rejected value names its field, so the client never reads the
// backend sentence (ADR 0006, #2515).

type codedAnswer struct {
	Code   string `json:"code"`
	Errors []struct {
		Field  string `json:"field"`
		Reason string `json:"reason"`
	} `json:"errors"`
}

func readCodedAnswer(t *testing.T, w *httptest.ResponseRecorder) codedAnswer {
	t.Helper()
	var body codedAnswer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body
}

// renderTo writes a client-error renderer the way the routes answer it; the
// shared runtime adds nothing to a 4xx answer that already carries its code.
func renderTo(renderer render.Renderer) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	_ = render.Render(w, r, renderer)
	return w
}

func TestPhaseWriteError_NamesCodeAndField(t *testing.T) {
	t.Parallel()

	phase := &capability.Phase{Name: " ", ServiceStartDate: "2026-09-01", ServiceEndDate: "2027-07-31"}
	err := fmt.Errorf("%w: %w", capability.ErrInvalidPhase, phase.Validate())

	w := renderTo(phaseWriteErrorRenderer(err))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := readCodedAnswer(t, w)
	assert.Equal(t, "enrollment.phase_name_required", body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "name", body.Errors[0].Field)
}

func TestPhaseWriteError_DuplicateNameMarksName(t *testing.T) {
	t.Parallel()

	w := renderTo(phaseWriteErrorRenderer(capability.ErrPhaseDuplicateName))
	assert.Equal(t, http.StatusConflict, w.Code)
	body := readCodedAnswer(t, w)
	assert.Equal(t, "enrollment.phase_name_exists", body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "name", body.Errors[0].Field)
}

func TestPhaseRequest_UnparsableServiceDateNamesField(t *testing.T) {
	t.Parallel()

	req := &PhaseRequest{Name: "Schuljahr", ServiceStartDate: "", ServiceEndDate: "2027-07-31"}
	_, err := req.toModel(0)
	require.Error(t, err)

	w := executePhaseJSON(t, buildPhaseRouter(&mockPhaseService{}), http.MethodPost, "/enrollment/phases",
		map[string]any{"name": "Schuljahr", "service_start_date": "", "service_end_date": "2027-07-31"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := readCodedAnswer(t, w)
	assert.Equal(t, "enrollment.phase_service_period_invalid", body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "service_start_date", body.Errors[0].Field)
}

func TestSchemaVersionError_NamesListEntryField(t *testing.T) {
	t.Parallel()

	schema := &capability.FormSchema{Name: "Schuljahr", Version: 1, CreatedBy: 1, Fields: []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText},
		{Key: "notes", Label: "", Type: capability.FormFieldText},
	}}
	err := fmt.Errorf("invalid schema: %w", schema.Validate())

	w := httptest.NewRecorder()
	renderSchemaVersionError(w, httptest.NewRequest(http.MethodPut, "/x", nil), err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := readCodedAnswer(t, w)
	assert.Equal(t, "enrollment.form_field_label_required", body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "fields.1.label", body.Errors[0].Field)
}

func TestMapSubmitError_RefusalsCarryCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"window closed", capability.ErrEnrollmentWindowClosed, http.StatusForbidden, "enrollment.window_closed"},
		{"disabled", capability.ErrEnrollmentDisabled, http.StatusForbidden, "enrollment.disabled"},
		{"duplicate", capability.ErrDuplicateEnrollment, http.StatusConflict, "enrollment.request_duplicate"},
		{"other person", capability.ErrExistingStudentAlreadyRequested, http.StatusConflict, "enrollment.child_already_requested"},
		{"offering closed", capability.ErrCareOfferingClosed, http.StatusBadRequest, "enrollment.care_offering_closed"},
		{"rate limited", capability.ErrRateLimited, http.StatusTooManyRequests, "enrollment.submission_rate_limited"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			mapSubmitError(w, httptest.NewRequest(http.MethodPost, "/x", nil), tc.err)
			assert.Equal(t, tc.status, w.Code)
			assert.Equal(t, tc.code, readCodedAnswer(t, w).Code)
		})
	}
}

func TestMapSubmitError_RejectedValueNamesChildField(t *testing.T) {
	t.Parallel()

	err := capability.InvalidInput(capability.CodeChildNameRequired, "children.1.first_name",
		fmt.Errorf("%w: child 1 missing name", capability.ErrInvalidSubmission))
	w := httptest.NewRecorder()
	mapSubmitError(w, httptest.NewRequest(http.MethodPost, "/x", nil), err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := readCodedAnswer(t, w)
	assert.Equal(t, "enrollment.child_name_required", body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "children.1.first_name", body.Errors[0].Field)
}

func TestBuildServiceRequest_UnparsableBirthDateIsRejectedValue(t *testing.T) {
	t.Parallel()

	_, err := buildServiceRequest(&SubmitEnrollmentRequest{
		Children: []SubmitChildRequest{{FirstName: "Mia", LastName: "Muster", DateOfBirth: "31.02.2019"}},
	}, 1, "")
	require.Error(t, err)

	// The mapper does not know the parse failure; the rejected value still
	// answers 400 with its code and field instead of a server error.
	w := httptest.NewRecorder()
	mapSubmitError(w, httptest.NewRequest(http.MethodPost, "/x", nil), err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := readCodedAnswer(t, w)
	assert.Equal(t, "enrollment.child_birth_date_invalid", body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "children.0.date_of_birth", body.Errors[0].Field)
}

func TestStatusLinkErrors_CarryCodes(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	mapEditError(w, httptest.NewRequest(http.MethodPut, "/x", nil), capability.ErrRequestNotFound)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "enrollment.status_link_invalid", readCodedAnswer(t, w).Code)

	w = httptest.NewRecorder()
	mapEditError(w, httptest.NewRequest(http.MethodPut, "/x", nil), capability.ErrEditNotAllowed)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "enrollment.edit_not_allowed", readCodedAnswer(t, w).Code)
}

func TestDecideErrors_CarryCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err    error
		status int
		code   string
	}{
		{capability.ErrDecisionRequestNotFound, http.StatusNotFound, "enrollment.request_not_found"},
		{capability.ErrDecisionAlreadyTerminal, http.StatusBadRequest, "enrollment.decision_already_final"},
		{fmt.Errorf("decision: %w: %w", capability.ErrDecisionInvalidData, errors.New("contact requires a name")), http.StatusBadRequest, "enrollment.approval_data_invalid"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		renderDecideError(w, httptest.NewRequest(http.MethodPost, "/x", nil), tc.err)
		assert.Equal(t, tc.status, w.Code)
		assert.Equal(t, tc.code, readCodedAnswer(t, w).Code)
	}
}

func TestRolloverError_RejectedValueKeepsItsCode(t *testing.T) {
	t.Parallel()

	_, err := parseRolloverCreateRequest(1, &RolloverCreateRequest{
		Name: "Folgejahr", ServiceStartDate: "2027-09-01", ServiceEndDate: "2028-07-31",
	}, 1)
	require.Error(t, err)

	created := &Resource{RolloverService: &mockRolloverService{}}
	router := chi.NewRouter()
	router.Post("/admin/phases/{id}/rollover", created.createRollover)
	createW := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/admin/phases/1/rollover",
		strings.NewReader(`{"name":"Folgejahr","service_start_date":"2027-09-01","service_end_date":"2028-07-31"}`))
	createReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(createW, createReq)
	assert.Equal(t, http.StatusBadRequest, createW.Code)
	body := readCodedAnswer(t, createW)
	assert.Equal(t, "rollover.deadline_required", body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "rollover_deadline", body.Errors[0].Field)

	rs := &Resource{}
	w := httptest.NewRecorder()
	wrapped := capability.InvalidInput(capability.CodePhaseNameRequired, "name",
		fmt.Errorf("%w: name is required", capability.ErrRolloverInvalidRequest))
	rs.mapRolloverError(w, httptest.NewRequest(http.MethodPost, "/x", nil), wrapped)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "enrollment.phase_name_required", readCodedAnswer(t, w).Code)
}
