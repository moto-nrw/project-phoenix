package exporttransfer_test

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The wire format of the capability's responses (#3050).
//
// These payloads travel through the frontend's route wrapper, and that wrapper
// treats ANY object carrying a "success" field as an already-formed API
// envelope: it forwards such an object as-is instead of nesting it under
// "data". A domain field called "success" therefore changes the response shape
// silently, and the browser reads `data.success` off an undefined `data`.
//
// That is exactly what happened once. These tests keep the field name out.

// decodeFields serializes a value and returns its top-level JSON keys.
func decodeFields(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(encoded, &fields))
	return fields
}

func TestOutcomeDoesNotCarryAnEnvelopeMarker(t *testing.T) {
	t.Parallel()

	fields := decodeFields(t, exporttransfer.Outcome{
		Transferred: true,
		Filename:    "zeitkonten-2026-08.csv",
		ByteSize:    42,
	})

	assert.NotContains(t, fields, "success",
		`a payload field named "success" makes the frontend route wrapper skip the data envelope`)
	assert.Contains(t, fields, "transferred", "the outcome must still say whether the file arrived")
	assert.Equal(t, true, fields["transferred"])
}

func TestStatusDoesNotCarryAnEnvelopeMarkerOrCredentials(t *testing.T) {
	t.Parallel()

	fields := decodeFields(t, exporttransfer.Status{
		Enabled:         true,
		Ready:           true,
		Host:            "dateien.beispiel.de",
		Port:            22,
		RemoteDirectory: "/upload/lohn",
	})

	assert.NotContains(t, fields, "success")
	// The status names the destination and nothing that could reach it.
	for _, forbidden := range []string{"password", "username", "host_key_fingerprint"} {
		assert.NotContains(t, fields, forbidden, "the status must stay credential-free")
	}
	assert.Contains(t, fields, "ready")
	assert.Contains(t, fields, "host")
}

// A failed transfer must be readable as a failure from the payload alone: the
// endpoint answers 200 so the journal row survives the tenant transaction.
func TestFailedOutcomeIsDistinguishableFromASuccessfulOne(t *testing.T) {
	t.Parallel()

	failed := decodeFields(t, exporttransfer.Outcome{
		Transferred: false,
		Filename:    "zeitkonten-2026-08.csv",
		Reason:      exporttransfer.ReasonHostKey,
	})

	assert.Equal(t, false, failed["transferred"])
	assert.Equal(t, exporttransfer.ReasonHostKey, failed["reason"])
}
