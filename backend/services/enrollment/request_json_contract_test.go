package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/require"
)

type carePeriodReadResult struct {
	periods []*capability.StudentCarePeriod
	err     error
}

func (r carePeriodReadResult) StudentCarePeriods(context.Context, int64) ([]*capability.StudentCarePeriod, error) {
	return r.periods, r.err
}

func TestCarePeriodReadPreservesFailureWithoutPartialResults(t *testing.T) {
	t.Parallel()
	failure := errors.New("care period read failed")
	valid := &capability.StudentCarePeriod{ServiceStartDate: "2026-09-01", ServiceEndDate: "2027-08-31"}
	for _, tc := range []struct {
		name   string
		result carePeriodReadResult
	}{
		{name: "owner error with partial rows", result: carePeriodReadResult{periods: []*capability.StudentCarePeriod{valid}, err: failure}},
		{name: "invalid start after valid row", result: carePeriodReadResult{periods: []*capability.StudentCarePeriod{valid, {ServiceStartDate: "2026-02-30", ServiceEndDate: "2027-08-31"}}}},
		{name: "invalid end after valid row", result: carePeriodReadResult{periods: []*capability.StudentCarePeriod{valid, {ServiceStartDate: "2026-09-01", ServiceEndDate: "2027-02-30"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			periods, err := ReadStudentCarePeriods(t.Context(), tc.result, 0)
			require.Error(t, err)
			require.Nil(t, periods, "failed reads must not expose a partial care-period history")
			if tc.result.err != nil {
				require.ErrorIs(t, err, failure)
			}
		})
	}
}

func TestRequestBatchRejectsMalformedStoredJSON(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"consent", "legal", "custom", "source"} {
		t.Run(field, func(t *testing.T) {
			invalid := json.RawMessage(`{"unfinished":`)
			row := &capability.Request{ID: 2}
			switch field {
			case "consent":
				row.ConsentFlags = invalid
			case "legal":
				row.LegalBlocksSnapshot = invalid
			case "custom":
				row.CustomData = invalid
			case "source":
				row.SourceMetadata = invalid
			}
			rows, err := intakeRequestValues([]*capability.Request{{ID: 1}, row})
			var syntaxError *json.SyntaxError
			require.ErrorAs(t, err, &syntaxError)
			require.Nil(t, rows, "a corrupt row must not return a successful partial batch")
		})
	}
}

func TestChildBatchRejectsMalformedStoredJSON(t *testing.T) {
	t.Parallel()
	rows, err := intakeChildValues([]*capability.RequestChild{
		{ID: 1, DateOfBirth: "2018-04-15"},
		{ID: 2, DateOfBirth: "2018-04-15", CustomData: json.RawMessage(`{"unfinished":`)},
	})
	var syntaxError *json.SyntaxError
	require.ErrorAs(t, err, &syntaxError)
	require.Nil(t, rows, "a corrupt row must not return a successful partial batch")
}

func TestChildBatchRejectsInvalidDates(t *testing.T) {
	t.Parallel()
	invalid := capability.Date("2027-02-30")
	for _, row := range []*capability.RequestChild{
		{ID: 2, DateOfBirth: invalid},
		{ID: 2, DateOfBirth: "2018-04-15", ActivateOn: &invalid},
	} {
		rows, err := intakeChildValues([]*capability.RequestChild{
			{ID: 1, DateOfBirth: "2018-04-15"}, row,
		})
		require.Error(t, err)
		require.Nil(t, rows, "invalid dates must not yield a partial child batch")
	}
}

func TestOwnerRequestJSONPreservesServiceContract(t *testing.T) {
	t.Parallel()
	mode := "immediate"
	value := &capability.Request{
		ID: 9007199254740993, TenantID: 42, PhaseID: 17,
		GuardianFirstName: "Anna", GuardianLastName: "Beispiel",
		GuardianEmail: "anna@example.test", StatusToken: "fixture-token",
		SubmittedAt:              time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC),
		ConsentFlags:             json.RawMessage(`{"photo":false}`),
		CustomData:               json.RawMessage(`{"answer":"yes","count":3}`),
		SourceMetadata:           json.RawMessage(`{}`),
		LegalBlocksSnapshot:      json.RawMessage(`[]`),
		DecisionNotificationMode: &mode,
	}
	legacy, err := intakeRequestValue(value)
	require.NoError(t, err)
	ownerJSON, err := json.Marshal(value)
	require.NoError(t, err)
	legacyJSON, err := json.Marshal(legacy)
	require.NoError(t, err)
	require.JSONEq(t, string(legacyJSON), string(ownerJSON))
	require.Contains(t, string(ownerJSON), `"id":9007199254740993`)
	require.NotContains(t, string(ownerJSON), "decision_notification_mode")
	require.NotContains(t, string(ownerJSON), "legal_blocks_snapshot")
	require.NotContains(t, string(ownerJSON), "guardian_phone")
	assertEnrollmentContractGolden(t, "request", ownerJSON)
}

func TestOwnerChildJSONPreservesServiceContract(t *testing.T) {
	t.Parallel()
	on := capability.Date("2027-09-01")
	for _, activateOn := range []*capability.Date{nil, &on} {
		value := &capability.RequestChild{
			ID: 9007199254740993, TenantID: 42, RequestID: 17,
			FirstName: "Lina", LastName: "Beispiel",
			DateOfBirth:    capability.Date("2018-04-15"),
			CustomData:     json.RawMessage(`{"answer":"yes","count":3}`),
			Status:         capability.ChildStatusSubmitted,
			ActivationMode: capability.ChildActivationScheduled, ActivateOn: activateOn,
		}
		legacy, err := intakeChildValue(value)
		require.NoError(t, err)
		ownerJSON, err := json.Marshal(value)
		require.NoError(t, err)
		legacyJSON, err := json.Marshal(legacy)
		require.NoError(t, err)
		require.JSONEq(t, string(legacyJSON), string(ownerJSON))
		require.Contains(t, string(ownerJSON), `"id":9007199254740993`)
		require.Contains(t, string(ownerJSON), `"date_of_birth":"2018-04-15"`)
		require.NotContains(t, string(ownerJSON), "matched_student_id")
		if activateOn == nil {
			require.NotContains(t, string(ownerJSON), "activate_on")
			assertEnrollmentContractGolden(t, "request_child", ownerJSON)
		} else {
			require.Contains(t, string(ownerJSON), `"activate_on":"2027-09-01"`)
			assertEnrollmentContractGolden(t, "request_child_scheduled", ownerJSON)
		}
	}
}

func assertEnrollmentContractGolden(t *testing.T, name string, actual []byte) {
	t.Helper()
	expected, err := os.ReadFile("testdata/" + name + ".golden")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), string(actual))
}
