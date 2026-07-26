package parent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

func TestTrackBFieldState_PersonAndDepartureBranches(t *testing.T) {
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

func TestTrackBFieldState_InvalidInputs(t *testing.T) {
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
	primary := &usersModels.GuardianPhoneNumber{Model: modelBase.Model{ID: 2}, PhoneNumber: "2", IsPrimary: true}
	secondary := &usersModels.GuardianPhoneNumber{Model: modelBase.Model{ID: 1}, PhoneNumber: "1"}
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
	svc := &service{}
	assert.JSONEq(t, `"parent@example.test"`, string(svc.guardianProfileFieldJSON(profile, "email")))
	assert.JSONEq(t, `"Musterweg 1"`, string(svc.guardianProfileFieldJSON(profile, "address_street")))
	assert.JSONEq(t, `"email"`, string(svc.guardianProfileFieldJSON(profile, "preferred_contact_method")))
	assert.JSONEq(t, `"de"`, string(svc.guardianProfileFieldJSON(profile, "language_preference")))
	assert.JSONEq(t, `null`, string(svc.guardianProfileFieldJSON(profile, "unknown")))
}

func TestParentWritePermissionHelpers(t *testing.T) {
	permissions := map[string]interface{}{
		authorize.GuardianPermissionMasterDataEdit: true,
	}

	child := &parentModels.ChildSummary{GuardianPermissions: permissions}
	assert.True(t, childHasPermission(child, authorize.GuardianPermissionMasterDataEdit))
	assert.False(t, childHasPermission(child, authorize.GuardianPermissionNotesWrite))
	assert.False(t, childHasPermission(nil, authorize.GuardianPermissionMasterDataEdit))

	resolved := &parentChild{guardianPermissions: permissions}
	assert.True(t, resolved.hasPermission(authorize.GuardianPermissionMasterDataEdit))
	assert.False(t, resolved.hasPermission(authorize.GuardianPermissionNotesWrite))
	assert.False(t, (*parentChild)(nil).hasPermission(authorize.GuardianPermissionMasterDataEdit))
}
