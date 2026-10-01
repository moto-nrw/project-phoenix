package enrollment

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/require"
)

// These routes render the owner's requests and children through the decoded
// values of owner_values.go. The owner's JSON and the decoded value's JSON
// must stay the service contract the goldens pin (#3565).

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
	legacy, err := requestValue(value)
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
		legacy, err := childValue(value)
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

// The routes decode a whole batch of children at once; a corrupt or invalid
// stored row fails the batch instead of yielding a partial one.
func TestChildBatchRejectsMalformedStoredJSON(t *testing.T) {
	t.Parallel()
	rows, err := childValues([]*capability.RequestChild{
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
		rows, err := childValues([]*capability.RequestChild{
			{ID: 1, DateOfBirth: "2018-04-15"}, row,
		})
		require.Error(t, err)
		require.Nil(t, rows, "invalid dates must not yield a partial child batch")
	}
}

func assertEnrollmentContractGolden(t *testing.T, name string, actual []byte) {
	t.Helper()
	expected, err := os.ReadFile("testdata/" + name + ".golden")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), string(actual))
}
