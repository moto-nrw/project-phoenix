package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FormField.Validate's target-handling branch (introduced with the
// linked-Stammdaten feature) keeps the editor and decision service
// from drifting: every reserved target has a single legal type, and
// the structured types (phone_list etc.) can only exist when a target
// requires them. Pure tests — no DB.

func validBaseField() *FormField {
	return &FormField{
		Key:       "allergies",
		Label:     "Allergien",
		Type:      FormFieldText,
		SortOrder: 0,
	}
}

func TestFormField_Validate_NoTargetIsFree(t *testing.T) {
	f := validBaseField()
	assert.NoError(t, f.Validate())
	assert.Equal(t, "", f.Target, "absent target stays empty")
}

func TestFormField_Validate_KnownTargetWithCorrectTypeOK(t *testing.T) {
	f := &FormField{
		Key:       "health_info",
		Label:     "Allergien",
		Type:      FormFieldTextarea, // matches ReservedTargets[TargetStudentHealthInfo].Type
		SortOrder: 0,
		Target:    TargetStudentHealthInfo,
	}
	assert.NoError(t, f.Validate())
}

func TestFormField_Validate_KnownTargetWithWrongTypeRejected(t *testing.T) {
	f := &FormField{
		Key:       "bus",
		Label:     "Bus",
		Type:      FormFieldText, // ReservedTargets requires boolean
		SortOrder: 0,
		Target:    TargetStudentBus,
	}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target")
	assert.Contains(t, err.Error(), "type")
}

func TestFormField_Validate_UnknownTargetRejected(t *testing.T) {
	f := &FormField{
		Key:       "foo",
		Label:     "Foo",
		Type:      FormFieldText,
		SortOrder: 0,
		Target:    "student.does_not_exist",
	}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown form field target")
}

func TestFormField_Validate_StructuredTypeWithoutTargetRejected(t *testing.T) {
	f := &FormField{
		Key:       "phones",
		Label:     "Telefonnummern",
		Type:      FormFieldPhoneList,
		SortOrder: 0,
	}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must declare a target")
}

func TestFormField_Validate_StructuredTypeWithMatchingTargetOK(t *testing.T) {
	f := &FormField{
		Key:       "contacts",
		Label:     "Weitere Kontakte",
		Type:      FormFieldContactList,
		SortOrder: 0,
		Target:    TargetStudentContacts,
	}
	assert.NoError(t, f.Validate())
}

func TestFormField_Validate_SelectTargetSeedsAreCheckedToo(t *testing.T) {
	// pickup_status is target=select; the editor seeds canonical options
	// but the model must still enforce that select fields ship options.
	f := &FormField{
		Key:       "pickup_status",
		Label:     "Abholregelung",
		Type:      FormFieldSelect,
		SortOrder: 0,
		Target:    TargetStudentPickupStatus,
		Options: []FormFieldOption{
			{Label: "Geht alleine nach Hause", Value: "alone"},
			{Label: "Wird abgeholt", Value: "picked_up"},
		},
	}
	assert.NoError(t, f.Validate())
}

func TestFormField_Validate_SelectWithoutOptionsRejected(t *testing.T) {
	f := &FormField{
		Key:       "pickup_status",
		Label:     "Abholregelung",
		Type:      FormFieldSelect,
		SortOrder: 0,
		Target:    TargetStudentPickupStatus,
	}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "select")
}

func TestIsStructuredFieldType_RecognisesAllThree(t *testing.T) {
	for _, ft := range []FormFieldType{
		FormFieldPhoneList,
		FormFieldWeekdaySchedule,
		FormFieldContactList,
	} {
		assert.True(t, isStructuredFieldType(ft), "type %q must be structured", ft)
	}
}

func TestIsStructuredFieldType_RejectsScalars(t *testing.T) {
	for _, ft := range []FormFieldType{
		FormFieldBoolean,
		FormFieldNumber,
		FormFieldText,
		FormFieldTextarea,
		FormFieldDate,
		FormFieldSelect,
	} {
		assert.False(t, isStructuredFieldType(ft), "type %q must not be structured", ft)
	}
}

func TestReservedTargets_AllEntriesHaveValidType(t *testing.T) {
	// Guard: every entry in ReservedTargets must reference a
	// valid FormFieldType. Prevents silent typos that would only
	// surface as "unknown form field type" at submit time.
	for target, spec := range ReservedTargets {
		assert.True(t, validFormFieldTypes[spec.Type],
			"target %q references unknown type %q", target, spec.Type)
		assert.NotEmpty(t, spec.Label, "target %q must carry a non-empty label", target)
	}
}
