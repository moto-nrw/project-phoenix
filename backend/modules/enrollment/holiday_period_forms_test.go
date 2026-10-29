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
	}, health)
	require.NoError(t, ValidateHolidayFormFields(HolidayFormTemplate()), "the template passes its own checks")

	shared := map[string]any{}
	for _, question := range questions {
		value, ok := question["share_with_host"]
		require.True(t, ok, "question %v names whether the hosting school sees it", question["key"])
		shared[question["key"].(string)] = value
	}
	assert.Equal(t, map[string]any{
		"lebensmittelunvertraeglichkeiten": false,
		"tetanus_impfschutz":               false,
		"foerderbedarf":                    false,
		"koerperliche_einschraenkungen":    false,
		"darf_allein_nach_hause":           true,
		"darf_an_ausfluegen_teilnehmen":    true,
	}, shared, "the template shares the way home and the trips with the hosting school (#5202)")
}

// TestHolidayFormFieldRefusesSharingAHealthQuestion pins #5202: a health
// question stays with the Träger, so the save refuses it with the hosting
// school's share at the field, as it refuses a share on an information
// block.
func TestHolidayFormFieldRefusesSharingAHealthQuestion(t *testing.T) {
	t.Parallel()
	question := func(typ FormFieldType, health, share bool) HolidayFormField {
		field := HolidayFormField{FormField: FormField{Key: "frage", Label: "Frage", Type: typ, AppliesToCh: true}, Health: health, ShareWithHost: share}
		if typ == FormFieldInfo {
			field.Content = "Hinweis"
		}
		return field
	}
	for name, field := range map[string]HolidayFormField{
		"health question":   question(FormFieldTextarea, true, true),
		"information block": question(FormFieldInfo, false, true),
	} {
		err := ValidateHolidayFormFields([]HolidayFormField{field})
		var rejection *InvalidInputError
		require.True(t, errors.As(err, &rejection), "%s: %v", name, err)
		assert.Equal(t, CodeHolidayFormFieldNotAllowed, rejection.Code, name)
		assert.Equal(t, "fields.0.share_with_host", rejection.Field, name)
		assert.ErrorIs(t, err, ErrHolidayFormFieldNotAllowed, name)
	}
	require.NoError(t, ValidateHolidayFormFields([]HolidayFormField{question(FormFieldBoolean, false, true)}),
		"a question without health information may go to the hosting school")
}
