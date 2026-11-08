package enrollmenthttp

import (
	"testing"

	"github.com/stretchr/testify/require"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func TestFormLabelsResponseKeepsOnlyWhatNamesAnAnswer(t *testing.T) {
	t.Parallel()
	labels := &capability.RequestFormLabels{
		Fields: []capability.FormField{
			{Key: "hinweis", Label: "Bitte beachten", Type: capability.FormFieldInfo, Content: "Text"},
			{
				Key: "lunch", Label: "Mittagessen", Type: capability.FormFieldSelect, AppliesToCh: true,
				HelpText: "Wählen Sie aus.",
				Translations: capability.Translations{
					"en": {"label": {Text: "Lunch", Source: "Mittagessen"}},
					"tr": {"label": {Text: "Eski", Source: "Essen"}},
				},
				Options: []capability.FormFieldOption{{Label: "Vegetarisch", Value: "veg"}},
			},
		},
		LegalBlocks: []capability.LegalBlock{{
			Key: "photo", Title: "Fotos",
			Translations: capability.Translations{"en": {"title": {Text: "Photos"}, "text": {Text: "Long text"}}},
		}},
	}

	out := toFormLabelsResponse(labels)

	require.Equal(t, []FormLabelField{{
		Key: "lunch", Label: "Mittagessen", Type: string(capability.FormFieldSelect), AppliesToChild: true,
		Translations: capability.Translations{"en": {"label": {Text: "Lunch"}}},
		Options:      []FormLabelOption{{Label: "Vegetarisch", Value: "veg"}},
	}}, out.Fields)
	require.Equal(t, []FormLabelLegalBlock{{
		Key: "photo", Title: "Fotos",
		Translations: capability.Translations{"en": {"title": {Text: "Photos"}}},
	}}, out.LegalBlocks)
}

func TestFormLabelsResponseWithoutLabelsIsEmptyNotNull(t *testing.T) {
	t.Parallel()
	out := toFormLabelsResponse(nil)
	require.NotNil(t, out.Fields)
	require.NotNil(t, out.LegalBlocks)
	require.Empty(t, out.Fields)
	require.Empty(t, out.LegalBlocks)
}
