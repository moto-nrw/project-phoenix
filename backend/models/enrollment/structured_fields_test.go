package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pure validator tests for the structured form-field value shapes:
// PhoneEntry, WeekdaySchedule, ContactEntry. The decision service +
// the submit service both call these before persisting, so the
// invariants they encode (subset/non-empty/required-fields) are the
// boundary between trusted and untrusted data. No DB needed.

// ---- PhoneEntry ---------------------------------------------------------

func TestPhoneEntry_Validate_RequiresPhoneNumber(t *testing.T) {
	p := &PhoneEntry{PhoneNumber: "  ", PhoneType: "mobile"}
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "phone_number")
}

func TestPhoneEntry_Validate_DefaultsTypeToOther(t *testing.T) {
	p := &PhoneEntry{PhoneNumber: "0177 12345"}
	require.NoError(t, p.Validate())
	assert.Equal(t, "other", p.PhoneType, "missing phone_type must default to other")
}

func TestPhoneEntry_Validate_RejectsUnknownType(t *testing.T) {
	p := &PhoneEntry{PhoneNumber: "0177 12345", PhoneType: "carrier-pigeon"}
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "phone_type")
}

func TestPhoneEntry_Validate_AcceptsKnownTypes(t *testing.T) {
	for _, kind := range []string{"mobile", "home", "work", "other"} {
		p := &PhoneEntry{PhoneNumber: "0177 12345", PhoneType: kind}
		assert.NoError(t, p.Validate(), "type %q must validate", kind)
	}
}

func TestPhoneEntry_Validate_TrimsPhoneNumber(t *testing.T) {
	p := &PhoneEntry{PhoneNumber: "  0177 12345  ", PhoneType: "mobile"}
	require.NoError(t, p.Validate())
	assert.Equal(t, "0177 12345", p.PhoneNumber)
}

// ---- WeekdaySchedule ----------------------------------------------------

func TestWeekdaySchedule_Validate_EmptyIsValid(t *testing.T) {
	assert.NoError(t, WeekdaySchedule{}.Validate())
}

func TestWeekdaySchedule_Validate_AcceptsAllKnownWeekdays(t *testing.T) {
	s := WeekdaySchedule{
		"mon": "08:00",
		"tue": "08:30",
		"wed": "09:00",
		"thu": "09:30",
		"fri": "10:00",
	}
	assert.NoError(t, s.Validate())
}

func TestWeekdaySchedule_Validate_RejectsUnknownWeekday(t *testing.T) {
	err := WeekdaySchedule{"sat": "10:00"}.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sat")
}

func TestWeekdaySchedule_Validate_RejectsBadTime(t *testing.T) {
	err := WeekdaySchedule{"mon": "25:99"}.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HH:MM")
}

func TestWeekdaySchedule_Validate_AcceptsEmptyDayValueAsSkip(t *testing.T) {
	// Empty string means "no time set for that day" — caller skips
	// that weekday at insert time rather than failing the whole row.
	assert.NoError(t, WeekdaySchedule{"mon": "", "tue": " "}.Validate())
}

// ---- ContactEntry -------------------------------------------------------

func TestContactEntry_Validate_RequiresName(t *testing.T) {
	c := &ContactEntry{FirstName: " ", LastName: "Müller", Email: "a@b.de"}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "first_name and last_name")
}

func TestContactEntry_Validate_RequiresEmailOrPhone(t *testing.T) {
	c := &ContactEntry{FirstName: "Erika", LastName: "Müller"}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of email or phone_numbers")
}

func TestContactEntry_Validate_OKWithEmailOnly(t *testing.T) {
	c := &ContactEntry{FirstName: "Erika", LastName: "Müller", Email: "erika@example.com"}
	assert.NoError(t, c.Validate())
}

func TestContactEntry_Validate_OKWithPhonesOnly(t *testing.T) {
	c := &ContactEntry{
		FirstName: "Erika",
		LastName:  "Müller",
		PhoneNumbers: []PhoneEntry{
			{PhoneNumber: "0177 12345", PhoneType: "mobile"},
		},
	}
	assert.NoError(t, c.Validate())
}

func TestContactEntry_Validate_PropagatesPhoneError(t *testing.T) {
	c := &ContactEntry{
		FirstName: "Erika",
		LastName:  "Müller",
		Email:     "erika@example.com",
		PhoneNumbers: []PhoneEntry{
			{PhoneNumber: "", PhoneType: "mobile"}, // invalid
		},
	}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Erika Müller")
	assert.Contains(t, err.Error(), "phone_number")
}

func TestContactEntry_Validate_RejectsNegativeEmergencyPriority(t *testing.T) {
	c := &ContactEntry{
		FirstName:         "Erika",
		LastName:          "Müller",
		Email:             "erika@example.com",
		EmergencyPriority: -1,
	}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "emergency_priority")
}
