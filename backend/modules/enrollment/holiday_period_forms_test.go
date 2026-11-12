package enrollment

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHolidayFormTemplateMarksEveryQuestionForHealthExplicitly pins the
// marker later readers rely on (#5202): every question of the template
// carries "health" on the wire, false included, so the way home and the
// excursions read as questions without health information and the four
// health questions as health information.
func TestHolidayFormTemplateMarksEveryQuestionForHealthExplicitly(t *testing.T) {
	t.Parallel()
	encoded, err := json.Marshal(HolidayFormTemplate())
	require.NoError(t, err)
	var questions []map[string]any
	require.NoError(t, json.Unmarshal(encoded, &questions))

	health := map[string]any{}
	for _, question := range questions {
		value, ok := question["health"]
		require.True(t, ok, "question %v names whether it is health information", question["key"])
		health[question["key"].(string)] = value
	}
	assert.Equal(t, map[string]any{
		"lebensmittelunvertraeglichkeiten": true,
		"tetanus_impfschutz":               true,
		"foerderbedarf":                    true,
		"koerperliche_einschraenkungen":    true,
		"darf_allein_nach_hause":           false,
		"darf_an_ausfluegen_teilnehmen":    false,
		"klasse":                           false,
	}, health)
	require.NoError(t, ValidateHolidayFormFields(HolidayFormTemplate()), "the template passes its own checks")

	shared := map[string]any{}
	for _, question := range questions {
		value, ok := question["share_with_host"]
		require.True(t, ok, "question %v names whether the hosting school sees it", question["key"])
		shared[question["key"].(string)] = value
	}
	assert.Equal(t, map[string]any{
		"lebensmittelunvertraeglichkeiten": true,
		"tetanus_impfschutz":               true,
		"foerderbedarf":                    true,
		"koerperliche_einschraenkungen":    true,
		"darf_allein_nach_hause":           true,
		"darf_an_ausfluegen_teilnehmen":    true,
		"klasse":                           true,
	}, shared, "the template shares every question with the hosting school; health answers only with the consent (#4722)")
}

// TestHolidayFormFieldSharesHealthQuestionsButNoInformationBlock pins
// #4722: a health question may go to the hosting school (it sees the answer
// only with the consent of the registration), an information block has no
// answer and the save refuses its share at the field.
func TestHolidayFormFieldSharesHealthQuestionsButNoInformationBlock(t *testing.T) {
	t.Parallel()
	question := func(typ FormFieldType, health, share bool) HolidayFormField {
		field := HolidayFormField{FormField: FormField{Key: "frage", Label: "Frage", Type: typ, AppliesToCh: true}, Health: health, ShareWithHost: share}
		if typ == FormFieldInfo {
			field.Content = "Hinweis"
		}
		return field
	}
	err := ValidateHolidayFormFields([]HolidayFormField{question(FormFieldInfo, false, true)})
	var rejection *InvalidInputError
	require.True(t, errors.As(err, &rejection), "information block: %v", err)
	assert.Equal(t, CodeHolidayFormFieldNotAllowed, rejection.Code)
	assert.Equal(t, "fields.0.share_with_host", rejection.Field)
	assert.ErrorIs(t, err, ErrHolidayFormFieldNotAllowed)
	require.NoError(t, ValidateHolidayFormFields([]HolidayFormField{question(FormFieldTextarea, true, true)}),
		"a health question may go to the hosting school")
	require.NoError(t, ValidateHolidayFormFields([]HolidayFormField{question(FormFieldBoolean, false, true)}),
		"a question without health information may go to the hosting school")
}

// TestHolidayFormTemplateAsksTheClassPerChild pins #5625: the template asks
// the class of each child as an optional text without health information.
func TestHolidayFormTemplateAsksTheClassPerChild(t *testing.T) {
	t.Parallel()
	for _, field := range HolidayFormTemplate() {
		if field.Key != "klasse" {
			continue
		}
		assert.Equal(t, "Klasse", field.Label)
		assert.Equal(t, FormFieldText, field.Type)
		assert.True(t, field.AppliesToCh, "per child")
		assert.False(t, field.Required, "optional")
		assert.False(t, field.Health)
		assert.True(t, field.ShareWithHost)
		return
	}
	t.Fatal("the template asks no class")
}

// TestHolidayAddressRequirementTakesThreeSettings pins the building block
// "Anschrift" (#5625): an empty setting asks nothing, the three settings
// stay, anything else is refused at its path.
func TestHolidayAddressRequirementTakesThreeSettings(t *testing.T) {
	t.Parallel()
	for in, want := range map[HolidayAddressRequirement]HolidayAddressRequirement{
		"": HolidayAddressOff, HolidayAddressOff: HolidayAddressOff,
		HolidayAddressOptional: HolidayAddressOptional, HolidayAddressRequired: HolidayAddressRequired,
	} {
		got, err := ValidateHolidayAddressRequirement(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got)
	}
	_, err := ValidateHolidayAddressRequirement("always")
	var rejection *InvalidInputError
	require.True(t, errors.As(err, &rejection), "%v", err)
	assert.Equal(t, "address_requirement", rejection.Field)
	assert.Equal(t, CodeHolidayFormFieldNotAllowed, rejection.Code)
}

// TestHolidayHealthRequirementTakesTwoSettings pins the building block
// "Gesundheit" (#6686): an empty setting keeps the fixed health fields
// optional, both settings stay, anything else is refused at its path.
func TestHolidayHealthRequirementTakesTwoSettings(t *testing.T) {
	t.Parallel()
	for in, want := range map[HolidayHealthRequirement]HolidayHealthRequirement{
		"": HolidayHealthOptional, HolidayHealthOptional: HolidayHealthOptional, HolidayHealthRequired: HolidayHealthRequired,
	} {
		got, err := ValidateHolidayHealthRequirement(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got)
	}
	_, err := ValidateHolidayHealthRequirement("off")
	var rejection *InvalidInputError
	require.True(t, errors.As(err, &rejection), "%v", err)
	assert.Equal(t, "health_requirement", rejection.Field)
	assert.Equal(t, CodeHolidayFormFieldNotAllowed, rejection.Code)
}
