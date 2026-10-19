package parentportal_test

import (
	"encoding/json"
	"testing"

	api "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func trackBFieldState(target, field string, person *parentService.MasterDataPerson, student *parentService.MasterDataStudent, value json.RawMessage) (json.RawMessage, json.RawMessage, bool, error) {
	return parentService.TrackBFieldStateFixture(target, field, person, student, value, api.ParentDeparturePolicy())
}
func TestTrackBFieldState_PersonAndDepartureBranches(t *testing.T) {
	t.Parallel()

	birthday := calendar.NewDate(2018, 3, 4)
	person := &parentService.MasterDataPerson{FirstName: "Felix", LastName: "Schneider", Birthday: &birthday}
	student := &parentService.MasterDataStudent{
		AllowedDepartureModes: parentService.AllowedDepartureModes{
			"mon": []parentService.DepartureMode{parentService.DeparturePickup},
		},
	}

	oldRaw, newRaw, changed, err := trackBFieldState(
		parentService.DataChangeTargetPerson,
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
		parentService.DataChangeTargetPerson,
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
		"departure",
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

	person := &parentService.MasterDataPerson{}
	student := &parentService.MasterDataStudent{SchoolClass: "1a"}

	oldRaw, newRaw, changed, err := trackBFieldState(
		parentService.DataChangeTargetStudent,
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
		parentService.DataChangeTargetStudent,
		"school_class",
		person,
		student,
		json.RawMessage(`""`),
	)
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)
}

func TestTrackBFieldState_InvalidInputs(t *testing.T) {
	t.Parallel()

	person := &parentService.MasterDataPerson{FirstName: "Felix", LastName: "Schneider"}
	student := &parentService.MasterDataStudent{}

	_, _, _, err := trackBFieldState(parentService.DataChangeTargetPerson, "first_name", person, student, json.RawMessage(`""`))
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)

	_, _, _, err = trackBFieldState(parentService.DataChangeTargetPerson, "birthday", person, student, json.RawMessage(`"bad-date"`))
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)

	_, _, _, err = trackBFieldState(parentService.DataChangeTargetPerson, "unknown", person, student, json.RawMessage(`"x"`))
	assert.ErrorIs(t, err, parentService.ErrMasterDataFieldNotEditable)

	_, _, _, err = trackBFieldState("departure", "pickup_status", person, student, json.RawMessage(`{}`))
	assert.ErrorIs(t, err, parentService.ErrMasterDataFieldNotEditable)

	_, _, _, err = trackBFieldState("departure", "allowed_departure_modes", person, student, json.RawMessage(`{`))
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)

	_, _, _, err = trackBFieldState("guardian_profile", "email", person, student, json.RawMessage(`"x"`))
	assert.ErrorIs(t, err, parentService.ErrMasterDataFieldNotEditable)
}
