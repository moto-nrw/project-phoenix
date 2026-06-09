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
		Type:      FormFieldText, // ReservedTargets requires weekday_boolean
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
	// pickup_status is no longer a select target (it is now weekday_boolean);
	// a free-form admin select must still ship options and validate.
	f := &FormField{
		Key:       "lunch_choice",
		Label:     "Mittagessen",
		Type:      FormFieldSelect,
		SortOrder: 0,
		Options: []FormFieldOption{
			{Label: "Vegetarisch", Value: "veggie"},
			{Label: "Mit Fleisch", Value: "meat"},
		},
	}
	assert.NoError(t, f.Validate())
}

func TestFormField_Validate_SelectWithoutOptionsRejected(t *testing.T) {
	f := &FormField{
		Key:       "lunch_choice",
		Label:     "Mittagessen",
		Type:      FormFieldSelect,
		SortOrder: 0,
	}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "select")
}

func TestFormField_Validate_PickupStatusMustBeWeekdayBoolean(t *testing.T) {
	// Locks in the post-migration contract: pickup_status is a
	// weekday_boolean reserved target. A weekday_boolean field wired to it
	// must validate...
	ok := &FormField{
		Key:       "pickup_status",
		Label:     "Abholregelung",
		Type:      FormFieldWeekdayBoolean,
		SortOrder: 0,
		Target:    TargetStudentPickupStatus,
	}
	assert.NoError(t, ok.Validate())

	// ...and the legacy select shape (the exact thing migration 1.15.117
	// rewrites) must now be rejected, so a stale schema can't slip through.
	legacy := &FormField{
		Key:       "pickup_status",
		Label:     "Abholregelung",
		Type:      FormFieldSelect,
		SortOrder: 0,
		Target:    TargetStudentPickupStatus,
		Options: []FormFieldOption{
			{Label: "Wird abgeholt", Value: "picked_up"},
		},
	}
	err := legacy.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target")
	assert.Contains(t, err.Error(), "type")
}

func TestIsStructuredFieldType_RecognisesAllStructuredTypes(t *testing.T) {
	for _, ft := range []FormFieldType{
		FormFieldPhoneList,
		FormFieldWeekdaySchedule,
		FormFieldWeekdayBoolean,
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
