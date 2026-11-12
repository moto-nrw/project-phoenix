package users

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(v string) *string { return &v }

// TestGuardianFinancialDataNeverSerializes is the isolation guarantee: even if
// a row reaches a generic JSON response by accident, no bank value rides along.
func TestGuardianFinancialDataNeverSerializes(t *testing.T) {
	t.Parallel()

	// Since #6332 the row holds sealed values and the mask only.
	data := &GuardianFinancialData{
		GuardianProfileID:   7,
		IBANSealed:          []byte("sealed-iban"),
		IBANMasked:          strPtr("•••• 3000"),
		AccountHolderSealed: []byte("sealed-holder"),
	}

	body, err := json.Marshal(data)
	require.NoError(t, err)

	assert.NotContains(t, string(body), "3000")
	assert.NotContains(t, string(body), "iban")
	assert.NotContains(t, string(body), "account_holder")
}

func TestGuardianFinancialDataValidate(t *testing.T) {
	t.Parallel()

	require.NoError(t, (&GuardianFinancialData{GuardianProfileID: 7}).Validate())
	require.Error(t, (&GuardianFinancialData{}).Validate())
}
