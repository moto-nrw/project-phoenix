package enrollment

import (
	"encoding/json"
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
}
