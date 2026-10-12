package application

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// A route that saves an offering sets counts_as_care itself, so an explicit
// false must reach Care Plan as false instead of the column default (#2734).
func TestCareOfferingRowOfKeepsExplicitCountsAsCareFalse(t *testing.T) {
	t.Parallel()

	row, err := CareOfferingRowOf(&enrollment.CareOffering{PhaseID: 5678, Name: "Randstunde", CountsAsCare: false})
	require.NoError(t, err)
	assert.False(t, row.CountsAsCare)
	assert.True(t, row.CountsAsCareSet)

	offering, err := careOfferingFromRow(row)
	require.NoError(t, err)
	assert.False(t, offering.CountsAsCare)
}

// The availability rule travels as the owner stores it: decoded into the row
// and encoded back unchanged, and absent when the value carries none.
func TestCareOfferingRowOfRoundTripsTheAvailabilityRule(t *testing.T) {
	t.Parallel()

	rule := json.RawMessage(`{"match":"all","conditions":[{"source":"grade_level","operator":"in","value":[1,2]}]}`)
	row, err := CareOfferingRowOf(&enrollment.CareOffering{PhaseID: 5678, Name: "OGS", AvailabilityRule: rule})
	require.NoError(t, err)
	require.NotNil(t, row.AvailabilityRule)
	assert.JSONEq(t, string(rule), string(PublicCareOffering(row).AvailabilityRule))

	row, err = CareOfferingRowOf(&enrollment.CareOffering{PhaseID: 5678, Name: "OGS"})
	require.NoError(t, err)
	assert.Nil(t, row.AvailabilityRule)

	_, err = CareOfferingRowOf(&enrollment.CareOffering{PhaseID: 5678, Name: "OGS", AvailabilityRule: json.RawMessage(`{`)})
	require.Error(t, err)
}
