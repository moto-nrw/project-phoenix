package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Pure-helper tests for the small functions the decision service
// uses to dispatch targeted form-field values onto downstream
// records. Covered here because each helper is independently
// testable without spinning up the test DB.

// ---- stringValue --------------------------------------------------------

func TestStringValue_TrimsString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello", stringValue("  hello  "))
}

func TestStringValue_NonStringReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", stringValue(42))
	assert.Equal(t, "", stringValue(nil))
	assert.Equal(t, "", stringValue(true))
	assert.Equal(t, "", stringValue([]string{"a"}))
}

func TestStringValue_EmptyStringReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", stringValue(""))
	assert.Equal(t, "", stringValue("   "))
}

// ---- decodeStructured ---------------------------------------------------

func TestDecodeStructured_DecodesPhoneList(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	raw := map[string]any{"mon": "08:00", "fri": "10:30"}
	var out enrollmentModels.WeekdaySchedule
	require.NoError(t, decodeStructured(raw, &out))
	assert.Equal(t, "08:00", out["mon"])
	assert.Equal(t, "10:30", out["fri"])
}

func TestDecodeStructured_DecodesWeekdayBoolean(t *testing.T) {
	t.Parallel()

	raw := map[string]any{"mon": true, "fri": false}
	var out enrollmentModels.WeekdayBoolean
	require.NoError(t, decodeStructured(raw, &out))
	assert.True(t, out["mon"])
	assert.False(t, out["fri"])
}

func TestDecodeBusDays_NormalizesSelectedWeekdays(t *testing.T) {
	t.Parallel()

	raw := map[string]any{"mon": true, "tue": false, "fri": true}
	out, err := decodeBusDays(raw)
	require.NoError(t, err)
	assert.True(t, out["mon"])
	assert.True(t, out["fri"])
	assert.False(t, out["tue"])
	assert.True(t, out.HasAny())
}

func TestDecodeBusDays_AcceptsLegacyBoolean(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	_, err := decodeBusDays(map[string]any{"sat": true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sat")
}

func TestDecodeDepartureDays_MapsModes(t *testing.T) {
	t.Parallel()

	out, err := decodeDepartureDays(map[string]any{
		"mon": "bus",
		"wed": "pickup",
		"thu": "alone", // normalized away
	})
	require.NoError(t, err)
	assert.Equal(t, users.DepartureBus, out.ModeFor("mon"))
	assert.Equal(t, users.DeparturePickup, out.ModeFor("wed"))
	assert.Equal(t, users.DepartureAlone, out.ModeFor("thu"))
	assert.Equal(t, users.DepartureAlone, out.ModeFor("fri"))
}

func TestDecodeDepartureDays_RejectsUnknownMode(t *testing.T) {
	t.Parallel()

	_, err := decodeDepartureDays(map[string]any{"mon": "taxi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "taxi")
}

func TestDecodeDepartureDays_RejectsUnknownWeekday(t *testing.T) {
	t.Parallel()

	_, err := decodeDepartureDays(map[string]any{"sat": "bus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sat")
}

func TestDecodePickupDays_NormalizesWeekdayMap(t *testing.T) {
	t.Parallel()

	out, err := decodePickupDays(map[string]any{"mon": true, "wed": false, "fri": true})
	require.NoError(t, err)
	assert.True(t, out["mon"])
	assert.True(t, out["fri"])
	assert.False(t, out["wed"])
	assert.True(t, out.HasAny())
}

func TestDecodePickupDays_AcceptsLegacySelectValues(t *testing.T) {
	t.Parallel()

	// Pending pre-migration submissions stored the frontend select option
	// *value*, not the German label. "picked_up" must map to all weekdays.
	pickedUp, err := decodePickupDays("picked_up")
	require.NoError(t, err)
	for _, day := range users.PickupDayOrder {
		assert.True(t, pickedUp[day], "picked_up should imply %s", day)
	}

	alone, err := decodePickupDays("alone")
	require.NoError(t, err)
	assert.False(t, alone.HasAny(), "alone should imply no pickup days")
}

func TestDecodePickupDays_AcceptsGermanLabels(t *testing.T) {
	t.Parallel()

	pickedUp, err := decodePickupDays(users.PickupStatusPickedUp)
	require.NoError(t, err)
	assert.True(t, pickedUp.HasAny())

	alone, err := decodePickupDays(users.PickupStatusGoesAlone)
	require.NoError(t, err)
	assert.False(t, alone.HasAny())
}

func TestDecodePickupDays_RejectsUnknownWeekday(t *testing.T) {
	t.Parallel()

	_, err := decodePickupDays(map[string]any{"sun": true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sun")
}

func TestDecodeStructured_DecodesContactList(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	// A scalar can't decode into []PhoneEntry — json.Unmarshal returns
	// an error and decodeStructured surfaces it.
	var out []enrollmentModels.PhoneEntry
	err := decodeStructured("not a list", &out)
	require.Error(t, err)
}

func TestContactGuardianRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		isEmergencyContact bool
		canPickup          bool
		want               string
	}{
		{
			name:      "pickup wins",
			canPickup: true,
			want:      authorize.GuardianRolePickupOnly,
		},
		{
			name:               "emergency contact",
			isEmergencyContact: true,
			want:               authorize.GuardianRoleEmergency,
		},
		{
			name: "no operational role",
			want: authorize.GuardianRoleCustom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, contactGuardianRole(tt.isEmergencyContact, tt.canPickup))
		})
	}
}
