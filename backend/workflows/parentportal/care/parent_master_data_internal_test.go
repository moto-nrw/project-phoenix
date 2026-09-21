package care

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

func TestTrackBFieldState_PersonAndDepartureBranches(t *testing.T) {
	t.Parallel()

	birthday := timezone.NewDate(2018, 3, 4)
	person := &usersModels.Person{FirstName: "Felix", LastName: "Schneider", Birthday: &birthday}
	student := &usersModels.Student{
		AllowedDepartureModes: usersModels.AllowedDepartureModes{
			usersModels.PickupDayMonday: []usersModels.DepartureMode{usersModels.DeparturePickup},
		},
	}

	oldRaw, newRaw, changed, err := trackBFieldState(
		usersModels.DataChangeTargetPerson,
		"last_name",
		person,
		student,
		json.RawMessage(`"Müller"`),
	)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.JSONEq(t, `"Schneider"`, string(oldRaw))
	assert.JSONEq(t, `"Müller"`, string(newRaw))

	oldRaw, newRaw, changed, err = trackBFieldState(
		usersModels.DataChangeTargetPerson,
		"birthday",
		person,
		student,
		json.RawMessage(`"2018-03-04"`),
	)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.JSONEq(t, `"2018-03-04"`, string(oldRaw))
	assert.JSONEq(t, `"2018-03-04"`, string(newRaw))

	oldRaw, newRaw, changed, err = trackBFieldState(
		usersModels.DataChangeTargetDeparture,
		"allowed_departure_modes",
		person,
		student,
		json.RawMessage(`{"mon":["pickup"],"tue":["bus"]}`),
	)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.JSONEq(t, `{"mon":["pickup"]}`, string(oldRaw))
	assert.JSONEq(t, `{"mon":["pickup"],"tue":["bus"]}`, string(newRaw))
}

func TestTrackBFieldState_SchoolClass(t *testing.T) {
	t.Parallel()

	person := &usersModels.Person{}
	student := &usersModels.Student{SchoolClass: "1a"}

	oldRaw, newRaw, changed, err := trackBFieldState(
		usersModels.DataChangeTargetStudent,
		"school_class",
		person,
		student,
		json.RawMessage(`"2b"`),
	)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.JSONEq(t, `"1a"`, string(oldRaw))
	assert.JSONEq(t, `"2b"`, string(newRaw))

	_, _, _, err = trackBFieldState(
		usersModels.DataChangeTargetStudent,
		"school_class",
		person,
		student,
		json.RawMessage(`""`),
	)
	assert.ErrorIs(t, err, ErrMasterDataInvalidValue)
}

func TestTrackBFieldState_InvalidInputs(t *testing.T) {
	t.Parallel()

	person := &usersModels.Person{FirstName: "Felix", LastName: "Schneider"}
	student := &usersModels.Student{}

	_, _, _, err := trackBFieldState(usersModels.DataChangeTargetPerson, "first_name", person, student, json.RawMessage(`""`))
	assert.ErrorIs(t, err, ErrMasterDataInvalidValue)

	_, _, _, err = trackBFieldState(usersModels.DataChangeTargetPerson, "birthday", person, student, json.RawMessage(`"bad-date"`))
	assert.ErrorIs(t, err, ErrMasterDataInvalidValue)

	_, _, _, err = trackBFieldState(usersModels.DataChangeTargetPerson, "unknown", person, student, json.RawMessage(`"x"`))
	assert.ErrorIs(t, err, ErrMasterDataFieldNotEditable)

	_, _, _, err = trackBFieldState(usersModels.DataChangeTargetDeparture, "pickup_status", person, student, json.RawMessage(`{}`))
	assert.ErrorIs(t, err, ErrMasterDataFieldNotEditable)

	_, _, _, err = trackBFieldState(usersModels.DataChangeTargetDeparture, "allowed_departure_modes", person, student, json.RawMessage(`{`))
	assert.ErrorIs(t, err, ErrMasterDataInvalidValue)

	_, _, _, err = trackBFieldState("guardian_profile", "email", person, student, json.RawMessage(`"x"`))
	assert.ErrorIs(t, err, ErrMasterDataFieldNotEditable)
}

func TestMasterDataSmallHelpers(t *testing.T) {
	t.Parallel()

	primary := &usersModels.GuardianPhoneNumber{PhoneNumber: "2", IsPrimary: true}
	primary.ID = 2
	secondary := &usersModels.GuardianPhoneNumber{PhoneNumber: "1"}
	secondary.ID = 1
	assert.Equal(t, primary, pickPrimaryPhone([]*usersModels.GuardianPhoneNumber{secondary, primary}))
	assert.Equal(t, secondary, pickPrimaryPhone([]*usersModels.GuardianPhoneNumber{secondary}))
	assert.Nil(t, pickPrimaryPhone(nil))

	got, err := decodeStringValue(json.RawMessage(`"  abc  "`))
	require.NoError(t, err)
	assert.Equal(t, "abc", got)
	_, err = decodeStringValue(nil)
	assert.ErrorIs(t, err, ErrMasterDataInvalidValue)
	_, err = decodeStringValue(json.RawMessage(`123`))
	assert.ErrorIs(t, err, ErrMasterDataInvalidValue)

	assert.Nil(t, strPtrOrNil("  "))
	assert.Equal(t, "abc", *strPtrOrNil("abc"))
	assert.JSONEq(t, `null`, string(jsonStringPtr(nil)))

	email := "parent@example.test"
	street := "Musterweg 1"
	profile := &usersModels.GuardianProfile{
		Email:                  &email,
		AddressStreet:          &street,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	svc := &Service{}
	assert.JSONEq(t, `"parent@example.test"`, string(svc.guardianProfileFieldJSON(profile, "email")))
	assert.JSONEq(t, `"Musterweg 1"`, string(svc.guardianProfileFieldJSON(profile, "address_street")))
	assert.JSONEq(t, `"email"`, string(svc.guardianProfileFieldJSON(profile, "preferred_contact_method")))
	assert.JSONEq(t, `"de"`, string(svc.guardianProfileFieldJSON(profile, "language_preference")))
	assert.JSONEq(t, `null`, string(svc.guardianProfileFieldJSON(profile, "unknown")))
}
