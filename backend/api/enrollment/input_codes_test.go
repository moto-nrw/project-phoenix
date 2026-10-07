package enrollment

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Every refusal the Anmeldung screens can meet answers with a registered code,
// and a rejected value names its field, so the client never reads the
// backend sentence (ADR 0006, #2515).

type codedAnswer struct {
	Code   string              `json:"code"`
	Errors []common.FieldError `json:"errors"`
}

func readCodedAnswer(t *testing.T, w *httptest.ResponseRecorder) codedAnswer {
	t.Helper()
	var body codedAnswer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body
}

func renderTo(renderer render.Renderer) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	common.RenderError(w, r, renderer)
	return w
}

func TestPhaseWriteError_NamesCodeAndField(t *testing.T) {
	t.Parallel()

	phase := &capability.Phase{Name: " ", ServiceStartDate: "2026-09-01", ServiceEndDate: "2027-07-31"}
	err := fmt.Errorf("%w: %w", capability.ErrInvalidPhase, phase.Validate())

	w := renderTo(phaseWriteErrorRenderer(err))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := readCodedAnswer(t, w)
	assert.Equal(t, common.CodeEnrollmentPhaseNameRequired, body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "name", body.Errors[0].Field)
}

func TestPhaseWriteError_DuplicateNameMarksName(t *testing.T) {
	t.Parallel()

	w := renderTo(phaseWriteErrorRenderer(capability.ErrPhaseDuplicateName))
	assert.Equal(t, http.StatusConflict, w.Code)
	body := readCodedAnswer(t, w)
	assert.Equal(t, common.CodeEnrollmentPhaseNameExists, body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "name", body.Errors[0].Field)
}

func TestPhaseRequest_UnparsableServiceDateNamesField(t *testing.T) {
	t.Parallel()

	req := &PhaseRequest{Name: "Schuljahr", ServiceStartDate: "", ServiceEndDate: "2027-07-31"}
	_, err := req.toModel(0)
	require.Error(t, err)

	body := readCodedAnswer(t, renderTo(common.ErrorInvalidRequest(err)))
	assert.Equal(t, common.CodeEnrollmentPhaseServicePeriodInvalid, body.Code)
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
	assert.Equal(t, common.CodeEnrollmentFormFieldLabelRequired, body.Code)
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
		{"window closed", capability.ErrEnrollmentWindowClosed, http.StatusForbidden, common.CodeEnrollmentWindowClosed},
		{"disabled", capability.ErrEnrollmentDisabled, http.StatusForbidden, common.CodeEnrollmentDisabled},
		{"duplicate", capability.ErrDuplicateEnrollment, http.StatusConflict, common.CodeEnrollmentRequestDuplicate},
		{"other person", capability.ErrExistingStudentAlreadyRequested, http.StatusConflict, common.CodeEnrollmentChildAlreadyRequested},
		{"offering closed", capability.ErrCareOfferingClosed, http.StatusBadRequest, common.CodeEnrollmentCareOfferingClosed},
		{"rate limited", capability.ErrRateLimited, http.StatusTooManyRequests, common.CodeEnrollmentSubmissionRateLimited},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			MapSubmitError(w, httptest.NewRequest(http.MethodPost, "/x", nil), tc.err)
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
	MapSubmitError(w, httptest.NewRequest(http.MethodPost, "/x", nil), err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := readCodedAnswer(t, w)
	assert.Equal(t, common.CodeEnrollmentChildNameRequired, body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "children.1.first_name", body.Errors[0].Field)
}

func TestBuildServiceRequest_UnparsableBirthDateIsRejectedValue(t *testing.T) {
	t.Parallel()

	_, err := BuildServiceRequest(&SubmitEnrollmentRequest{
		Children: []SubmitChildRequest{{FirstName: "Mia", LastName: "Muster", DateOfBirth: "31.02.2019"}},
	}, 1, "")
	require.Error(t, err)

	// The mapper does not know the parse failure; the rejected value still
	// answers 400 with its code and field instead of a server error.
	w := httptest.NewRecorder()
	MapSubmitError(w, httptest.NewRequest(http.MethodPost, "/x", nil), err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := readCodedAnswer(t, w)
	assert.Equal(t, common.CodeEnrollmentChildBirthDateInvalid, body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "children.0.date_of_birth", body.Errors[0].Field)
}

func TestStatusLinkErrors_CarryCodes(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	mapEditError(w, httptest.NewRequest(http.MethodPut, "/x", nil), capability.ErrRequestNotFound)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, common.CodeEnrollmentStatusLinkInvalid, readCodedAnswer(t, w).Code)

	w = httptest.NewRecorder()
	mapEditError(w, httptest.NewRequest(http.MethodPut, "/x", nil), capability.ErrEditNotAllowed)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, common.CodeEnrollmentEditNotAllowed, readCodedAnswer(t, w).Code)
}

func TestDecideErrors_CarryCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err    error
		status int
		code   string
	}{
		{capability.ErrDecisionRequestNotFound, http.StatusNotFound, common.CodeEnrollmentRequestNotFound},
		{capability.ErrDecisionAlreadyTerminal, http.StatusBadRequest, common.CodeEnrollmentDecisionAlreadyFinal},
		{fmt.Errorf("decision: %w: %w", capability.ErrDecisionInvalidData, errors.New("contact requires a name")), http.StatusBadRequest, common.CodeEnrollmentApprovalDataInvalid},
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

	body := readCodedAnswer(t, renderTo(common.ErrorInvalidRequest(err)))
	assert.Equal(t, common.CodeRolloverDeadlineRequired, body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "rollover_deadline", body.Errors[0].Field)

	rs := &Resource{}
	w := httptest.NewRecorder()
	wrapped := capability.InvalidInput(capability.CodePhaseNameRequired, "name",
		fmt.Errorf("%w: name is required", capability.ErrRolloverInvalidRequest))
	rs.mapRolloverError(w, httptest.NewRequest(http.MethodPost, "/x", nil), wrapped)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, common.CodeEnrollmentPhaseNameRequired, readCodedAnswer(t, w).Code)
}
