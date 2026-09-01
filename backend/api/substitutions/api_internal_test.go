package substitutions

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAssignAdditionalSupervisionAcceptsOnlyTargetIDs(t *testing.T) {
	t.Parallel()
	request, err := decodeAssignment(strings.NewReader(`{
		"type":"additional_supervision",
		"additional_supervision":{"active_group_id":41,"target_staff_id":73}
	}`))

	require.NoError(t, err)
	assignment, err := request.toAssignment()
	require.NoError(t, err)
	require.Equal(t, "additional_supervision", string(request.Type))
	require.Equal(t, int64(41), assignment.AdditionalSupervision.ActiveGroupID)
	require.Equal(t, int64(73), assignment.AdditionalSupervision.TargetStaffID)
}

func TestAssignAdditionalSupervisionPreservesLargeStringIDs(t *testing.T) {
	t.Parallel()
	request, err := decodeAssignment(strings.NewReader(`{
		"type":"additional_supervision",
		"additional_supervision":{"active_group_id":"9007199254740993","target_staff_id":"9007199254740995"}
	}`))

	require.NoError(t, err)
	assignment, err := request.toAssignment()
	require.NoError(t, err)
	require.Equal(t, int64(9007199254740993), assignment.AdditionalSupervision.ActiveGroupID)
	require.Equal(t, int64(9007199254740995), assignment.AdditionalSupervision.TargetStaffID)
}

func TestAssignAdditionalSupervisionRejectsClientRoleAndStart(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"role": `{
			"type":"additional_supervision",
			"additional_supervision":{"active_group_id":41,"target_staff_id":73,"role":"supervisor"}
		}`,
		"start": `{
			"type":"additional_supervision",
			"additional_supervision":{"active_group_id":41,"target_staff_id":73,"start_time":"2026-08-30T10:00:00Z"}
		}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := decodeAssignment(strings.NewReader(body))
			require.Error(t, err)
		})
	}
}

func TestAssignAdditionalSupervisionRejectsTrailingJSON(t *testing.T) {
	t.Parallel()

	_, err := decodeAssignment(strings.NewReader(
		`{"type":"additional_supervision","additional_supervision":{"active_group_id":41,"target_staff_id":73}} {}`,
	))
	require.Error(t, err)
}

func TestRenderModuleErrorUsesStableContract(t *testing.T) {
	t.Parallel()

	for _, spec := range moduleErrorSpecs {
		t.Run(spec.code, func(t *testing.T) {
			t.Parallel()
			response := renderErrorResponse(t, fmt.Errorf("wrapped: %w", spec.target))
			require.Equal(t, spec.status, response.status)
			require.Equal(t, spec.code, response.body.Code)
			require.Equal(t, spec.message, response.body.Error)
		})
	}
}

func TestScheduleAssignmentRequestPreservesAppointmentScope(t *testing.T) {
	t.Parallel()

	var request assignmentRequest
	err := json.NewDecoder(strings.NewReader(`{
		"type":"schedule_substitution",
		"schedule_substitution":{
			"instance_id":42,
			"substitutions":[{"absent_staff_id":7,"substitute_staff_id":8,"instance_ids":[42,43]}]
		}
	}`)).Decode(&request)
	require.NoError(t, err)
	assignment, err := request.toAssignment()
	require.NoError(t, err)
	require.Equal(t, int64(42), assignment.ScheduleSubstitution.InstanceID)
	require.Equal(t, []int64{42, 43}, *assignment.ScheduleSubstitution.Substitutions[0].InstanceIDs)
}

func TestGroupAssignmentRequestPreservesDateErrors(t *testing.T) {
	t.Parallel()

	var request assignmentRequest
	require.NoError(t, json.NewDecoder(strings.NewReader(`{
		"type":"group_handover",
		"group_handover":{"start_date":"kein-datum"}
	}`)).Decode(&request))

	_, err := request.toAssignment()
	response := renderErrorResponse(t, err)
	require.Equal(t, 400, response.status)
	require.Equal(t, "invalid_period", response.body.Code)
	require.Equal(t, "Das Startdatum ist ungültig.", response.body.Error)
}

func TestRenderModuleErrorHidesInternalCause(t *testing.T) {
	t.Parallel()

	response := renderErrorResponse(t, errors.New("postgres password leaked"))
	require.Equal(t, 500, response.status)
	require.Equal(t, "internal", response.body.Code)
	require.Equal(t, "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.", response.body.Error)
	require.NotContains(t, response.rawBody, "postgres")
}

type renderedError struct {
	status  int
	rawBody string
	body    struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Code   string `json:"code"`
	}
}

func renderErrorResponse(t *testing.T, err error) renderedError {
	t.Helper()
	request := httptest.NewRequest("GET", "/api/substitutions", nil)
	recorder := httptest.NewRecorder()
	renderModuleError(recorder, request, err)
	response := renderedError{status: recorder.Code, rawBody: recorder.Body.String()}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response.body))
	return response
}
