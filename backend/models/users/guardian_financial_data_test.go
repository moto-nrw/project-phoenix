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

	data := &GuardianFinancialData{
		GuardianProfileID: 7,
		IBAN:              strPtr("DE89370400440532013000"),
		AccountHolder:     strPtr("Sabine Schneider"),
	}

	body, err := json.Marshal(data)
	require.NoError(t, err)

	assert.NotContains(t, string(body), "DE89370400440532013000")
	assert.NotContains(t, string(body), "iban")
	assert.NotContains(t, string(body), "account_holder")
}

func TestGuardianFinancialDataValidate(t *testing.T) {
	t.Parallel()

	require.NoError(t, (&GuardianFinancialData{GuardianProfileID: 7}).Validate())
	require.Error(t, (&GuardianFinancialData{}).Validate())
}

// TestGuardianFinancialDataHasData pins that a row cleared of both fields reads
// as "no bank details", not as an empty-but-present record.
func TestGuardianFinancialDataHasData(t *testing.T) {
	t.Parallel()

	assert.False(t, (&GuardianFinancialData{GuardianProfileID: 7}).HasData())
	assert.False(t, (&GuardianFinancialData{GuardianProfileID: 7, IBAN: strPtr("")}).HasData())
	assert.True(t, (&GuardianFinancialData{GuardianProfileID: 7, IBAN: strPtr("DE89")}).HasData())
	assert.True(t, (&GuardianFinancialData{GuardianProfileID: 7, AccountHolder: strPtr("A")}).HasData())
}
