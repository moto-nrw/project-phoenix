package enrollmenthttp

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

// These routes render the owner's requests and children directly; their
// answers travel decoded as FormValues (ADR 0170). The owner's JSON must stay
// the service contract the goldens pin (#3565).

func TestOwnerRequestJSONPreservesServiceContract(t *testing.T) {
	t.Parallel()
	mode := "immediate"
	value := &capability.Request{
		ID: 9007199254740993, TenantID: 42, PhaseID: 17,
		GuardianFirstName: "Anna", GuardianLastName: "Beispiel",
		GuardianEmail: "anna@example.test", StatusToken: "fixture-token",
		SubmittedAt:              time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC),
		ConsentFlags:             decodedFormValues(t, `{"photo":false}`),
		CustomData:               decodedFormValues(t, `{"answer":"yes","count":3}`),
		SourceMetadata:           decodedFormValues(t, `{}`),
		LegalBlocksSnapshot:      []capability.LegalBlocksSnapshotEntry{},
		DecisionNotificationMode: &mode,
	}
	ownerJSON, err := json.Marshal(value)
	require.NoError(t, err)
	require.Contains(t, string(ownerJSON), `"id":9007199254740993`)
	assertEnrollmentContractGolden(t, "request", ownerJSON)
}

func TestOwnerChildJSONPreservesServiceContract(t *testing.T) {
	t.Parallel()
	on := calendar.NewDate(2027, 9, 1)
	for _, activateOn := range []*calendar.Date{nil, &on} {
		value := &capability.RequestChild{
			ID: 9007199254740993, TenantID: 42, RequestID: 17,
			FirstName: "Lina", LastName: "Beispiel",
			DateOfBirth:    calendar.NewDate(2018, 4, 15),
			CustomData:     decodedFormValues(t, `{"answer":"yes","count":3}`),
			Status:         capability.ChildStatusSubmitted,
			ActivationMode: capability.ChildActivationScheduled, ActivateOn: activateOn,
		}
		ownerJSON, err := json.Marshal(value)
		require.NoError(t, err)
		require.Contains(t, string(ownerJSON), `"id":9007199254740993`)
		require.Contains(t, string(ownerJSON), `"date_of_birth":"2018-04-15"`)
		if activateOn == nil {
			assertEnrollmentContractGolden(t, "request_child", ownerJSON)
		} else {
			require.Contains(t, string(ownerJSON), `"activate_on":"2027-09-01"`)
			assertEnrollmentContractGolden(t, "request_child_scheduled", ownerJSON)
		}
	}
}

// Empty answers, consent flags and source metadata render as {} (ADR 0170).
func TestOwnerRequestJSONRendersEmptyValuesAsObjects(t *testing.T) {
	t.Parallel()
	ownerJSON, err := json.Marshal(&capability.Request{})
	require.NoError(t, err)
	for _, field := range []string{`"consent_flags":{}`, `"custom_data":{}`, `"source_metadata":{}`} {
		require.Contains(t, string(ownerJSON), field)
	}
	childJSON, err := json.Marshal(&capability.RequestChild{})
	require.NoError(t, err)
	require.Contains(t, string(childJSON), `"custom_data":{}`)
}

func decodedFormValues(t *testing.T, raw string) capability.FormValues {
	t.Helper()
	var values capability.FormValues
	require.NoError(t, json.Unmarshal([]byte(raw), &values))
	return values
}

func assertEnrollmentContractGolden(t *testing.T, name string, actual []byte) {
	t.Helper()
	expected, err := os.ReadFile("testdata/" + name + ".golden")
	require.NoError(t, err)
	require.JSONEq(t, string(expected), string(actual))
}
