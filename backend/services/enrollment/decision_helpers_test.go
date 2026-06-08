package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// Pure-helper tests for the small functions the decision service
// uses to dispatch targeted form-field values onto downstream
// records. Covered here because each helper is independently
// testable without spinning up the test DB.

// ---- stringValue --------------------------------------------------------

func TestStringValue_TrimsString(t *testing.T) {
	assert.Equal(t, "hello", stringValue("  hello  "))
}

func TestStringValue_NonStringReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", stringValue(42))
	assert.Equal(t, "", stringValue(nil))
	assert.Equal(t, "", stringValue(true))
	assert.Equal(t, "", stringValue([]string{"a"}))
}

func TestStringValue_EmptyStringReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", stringValue(""))
	assert.Equal(t, "", stringValue("   "))
}

// ---- decodeStructured ---------------------------------------------------

func TestDecodeStructured_DecodesPhoneList(t *testing.T) {
	raw := []any{
		map[string]any{
			"phone_number": "0177 12345",
			"phone_type":   "mobile",
			"is_primary":   true,
		},
	}
	var out []enrollmentModels.PhoneEntry
	require.NoError(t, decodeStructured(raw, &out))
	require.Len(t, out, 1)
	assert.Equal(t, "0177 12345", out[0].PhoneNumber)
	assert.Equal(t, "mobile", out[0].PhoneType)
	assert.True(t, out[0].IsPrimary)
}

func TestDecodeStructured_DecodesWeekdaySchedule(t *testing.T) {
	raw := map[string]any{"mon": "08:00", "fri": "10:30"}
	var out enrollmentModels.WeekdaySchedule
	require.NoError(t, decodeStructured(raw, &out))
	assert.Equal(t, "08:00", out["mon"])
	assert.Equal(t, "10:30", out["fri"])
}

func TestDecodeStructured_DecodesWeekdayBoolean(t *testing.T) {
	raw := map[string]any{"mon": true, "fri": false}
	var out enrollmentModels.WeekdayBoolean
	require.NoError(t, decodeStructured(raw, &out))
	assert.True(t, out["mon"])
	assert.False(t, out["fri"])
}

func TestDecodeBusDays_NormalizesSelectedWeekdays(t *testing.T) {
	raw := map[string]any{"mon": true, "tue": false, "fri": true}
	out, err := decodeBusDays(raw)
	require.NoError(t, err)
	assert.True(t, out["mon"])
	assert.True(t, out["fri"])
	assert.False(t, out["tue"])
	assert.True(t, out.HasAny())
}

func TestDecodeBusDays_AcceptsLegacyBoolean(t *testing.T) {
	enabled, err := decodeBusDays(true)
	require.NoError(t, err)
	assert.True(t, enabled["mon"])
	assert.True(t, enabled["tue"])
	assert.True(t, enabled["wed"])
	assert.True(t, enabled["thu"])
	assert.True(t, enabled["fri"])

	disabled, err := decodeBusDays(false)
	require.NoError(t, err)
	assert.False(t, disabled.HasAny())
}

func TestDecodeBusDays_RejectsUnknownWeekday(t *testing.T) {
	_, err := decodeBusDays(map[string]any{"sat": true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sat")
}

func TestDecodeStructured_DecodesContactList(t *testing.T) {
	raw := []any{
		map[string]any{
			"first_name":           "Erika",
			"last_name":            "Müller",
			"email":                "erika@example.com",
			"is_emergency_contact": true,
			"can_pickup":           true,
		},
	}
	var out []enrollmentModels.ContactEntry
	require.NoError(t, decodeStructured(raw, &out))
	require.Len(t, out, 1)
	assert.Equal(t, "Erika", out[0].FirstName)
	assert.True(t, out[0].IsEmergencyContact)
	assert.True(t, out[0].CanPickup)
}

func TestDecodeStructured_RejectsMismatch(t *testing.T) {
	// A scalar can't decode into []PhoneEntry — json.Unmarshal returns
	// an error and decodeStructured surfaces it.
	var out []enrollmentModels.PhoneEntry
	err := decodeStructured("not a list", &out)
	require.Error(t, err)
}
