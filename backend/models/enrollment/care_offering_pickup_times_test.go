package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pure tests for the pickup_times (Angebots-Gehzeit) validation on
// CareOffering.Validate (#2290). A Gehzeit is optional per weekday; keys
// must be canonical day codes within available_days and values HH:MM.

func TestCareOffering_Validate_PickupTimes_HappyPath(t *testing.T) {
	c := validCareOffering()
	c.PickupTimes = map[string]string{"mon": "14:30", "fri": "16:00"}
	require.NoError(t, c.Validate())
	assert.Equal(t, map[string]string{"mon": "14:30", "fri": "16:00"}, c.PickupTimes)
}

func TestCareOffering_Validate_PickupTimes_NilStaysNil(t *testing.T) {
	c := validCareOffering()
	c.PickupTimes = nil
	require.NoError(t, c.Validate())
	assert.Nil(t, c.PickupTimes)
}

func TestCareOffering_Validate_PickupTimes_DropsEmptyValues(t *testing.T) {
	c := validCareOffering()
	c.PickupTimes = map[string]string{"mon": "14:30", "tue": "  ", "wed": ""}
	require.NoError(t, c.Validate())
	assert.Equal(t, map[string]string{"mon": "14:30"}, c.PickupTimes)
}

func TestCareOffering_Validate_PickupTimes_EmptyMapBecomesNil(t *testing.T) {
	c := validCareOffering()
	c.PickupTimes = map[string]string{"mon": ""}
	require.NoError(t, c.Validate())
	assert.Nil(t, c.PickupTimes)
}

func TestCareOffering_Validate_PickupTimes_NormalizesKeysAndValues(t *testing.T) {
	c := validCareOffering()
	c.PickupTimes = map[string]string{"MON": " 14:30 "}
	require.NoError(t, c.Validate())
	assert.Equal(t, map[string]string{"mon": "14:30"}, c.PickupTimes)
}

func TestCareOffering_Validate_PickupTimes_RejectsUnknownDay(t *testing.T) {
	c := validCareOffering()
	c.PickupTimes = map[string]string{"montag": "14:30"}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pickup_times")
}

func TestCareOffering_Validate_PickupTimes_RejectsDayOutsideAvailableDays(t *testing.T) {
	c := validCareOffering()
	c.AvailableDays = []string{"mon", "tue"}
	c.PickupTimes = map[string]string{"fri": "14:30"}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "available_days")
}

func TestCareOffering_Validate_PickupTimes_RejectsInvalidTime(t *testing.T) {
	c := validCareOffering()
	c.PickupTimes = map[string]string{"mon": "25:99"}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pickup_times")
}

func TestCareOffering_Validate_PickupTimes_ZeroPadsHours(t *testing.T) {
	// time.Parse accepts "9:30"; the stored value must still be canonical
	// zero-padded HH:MM, because the reconciler's latest-wins rule compares
	// the strings lexicographically ("15:00" must beat "9:00").
	c := validCareOffering()
	c.PickupTimes = map[string]string{"mon": "9:30"}
	require.NoError(t, c.Validate())
	assert.Equal(t, map[string]string{"mon": "09:30"}, c.PickupTimes)
}
