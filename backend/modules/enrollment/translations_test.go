package enrollment

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslationsNormalize(t *testing.T) {
	t.Parallel()

	t.Run("trims entries and drops empty ones", func(t *testing.T) {
		t.Parallel()
		in := Translations{
			" RU ": {
				TranslationAttrLabel:    {Text: "  Имя ребёнка ", Source: " Name des Kindes "},
				TranslationAttrHelpText: {Text: "   ", Source: "Hinweis"},
			},
			"uk": {TranslationAttrLabel: {Text: ""}},
		}
		out, err := in.Normalize(TranslationAttrLabel, TranslationAttrHelpText)
		require.NoError(t, err)
		assert.Equal(t, Translations{
			"ru": {TranslationAttrLabel: {Text: "Имя ребёнка", Source: "Name des Kindes"}},
		}, out)
	})

	t.Run("returns nil when nothing remains", func(t *testing.T) {
		t.Parallel()
		out, err := Translations{"en": {TranslationAttrLabel: {Text: " "}}}.Normalize(TranslationAttrLabel)
		require.NoError(t, err)
		assert.Nil(t, out)
	})

	t.Run("rejects the German source language", func(t *testing.T) {
		t.Parallel()
		_, err := Translations{"de": {TranslationAttrLabel: {Text: "Name"}}}.Normalize(TranslationAttrLabel)
		require.Error(t, err)
	})

	t.Run("rejects malformed language codes", func(t *testing.T) {
		t.Parallel()
		for _, locale := range []string{"", "e", "en-US", "engl", "e1"} {
			_, err := Translations{locale: {TranslationAttrLabel: {Text: "Name"}}}.Normalize(TranslationAttrLabel)
			require.Error(t, err, "locale %q", locale)
		}
	})

	t.Run("rejects attributes the entity does not translate", func(t *testing.T) {
		t.Parallel()
		_, err := Translations{"en": {TranslationAttrText: {Text: "Body"}}}.Normalize(TranslationAttrLabel)
		require.Error(t, err)
	})

	t.Run("rejects overlong texts", func(t *testing.T) {
		t.Parallel()
		long := strings.Repeat("ä", translationMaxTextLength+1)
		_, err := Translations{"en": {TranslationAttrLabel: {Text: long}}}.Normalize(TranslationAttrLabel)
		require.Error(t, err)
	})

	t.Run("rejects a translation without its German source", func(t *testing.T) {
		t.Parallel()
		_, err := Translations{"en": {TranslationAttrLabel: {Text: "Name"}}}.Normalize(TranslationAttrLabel)
		require.Error(t, err)
	})
}

func TestTranslationsFresh(t *testing.T) {
	t.Parallel()
	stored := Translations{
		"ru": {
			TranslationAttrLabel:    {Text: "Имя", Source: "Name"},
			TranslationAttrHelpText: {Text: "Старая подсказка", Source: "Alter Hinweis"},
		},
		"en": {TranslationAttrLabel: {Text: "Name", Source: ""}},
	}

	fresh := stored.Fresh(map[string]string{
		TranslationAttrLabel:    " Name ",
		TranslationAttrHelpText: "Neuer Hinweis",
	})

	// The help text changed after it was translated and the English label was
	// never confirmed against a source, so parents read German for both.
	assert.Equal(t, Translations{"ru": {TranslationAttrLabel: {Text: "Имя"}}}, fresh)

	raw, err := json.Marshal(fresh)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "source", "public translations never expose the German source")

	assert.Nil(t, stored.Fresh(map[string]string{TranslationAttrLabel: "Vorname"}))
	assert.Nil(t, Translations(nil).Fresh(map[string]string{TranslationAttrLabel: "Name"}))
}

func TestPhasePublicNameTranslations(t *testing.T) {
	t.Parallel()
	phase := &Phase{
		Name: "Schuljahr 2026/27",
		Translations: Translations{
			"ru": {TranslationAttrName: {Text: "Учебный год 2026/27", Source: "Schuljahr 2026/27"}},
			// Translated before the phase was renamed.
			"en": {TranslationAttrName: {Text: "School year 2025/26", Source: "Schuljahr 2025/26"}},
		},
	}

	assert.Equal(t, map[string]string{"ru": "Учебный год 2026/27"}, phase.PublicNameTranslations())
	assert.Nil(t, (&Phase{Name: "Ferien"}).PublicNameTranslations())
}

func TestFormFieldValidateNormalizesTranslations(t *testing.T) {
	t.Parallel()
	field := FormField{
		Key:   "meal",
		Label: "Essen",
		Type:  FormFieldSelect,
		Options: []FormFieldOption{{
			Label: "Vegetarisch", Value: "veg",
			Translations: Translations{"ru": {TranslationAttrLabel: {Text: " Вегетарианское ", Source: "Vegetarisch"}}},
		}},
		Translations: Translations{"ru": {TranslationAttrLabel: {Text: "Питание", Source: "Essen"}}},
	}
	require.NoError(t, field.Validate())
	assert.Equal(t, "Вегетарианское", field.Options[0].Translations["ru"][TranslationAttrLabel].Text)

	field.Translations = Translations{"ru": {TranslationAttrTitle: {Text: "Питание"}}}
	require.Error(t, field.Validate(), "a field has no title to translate")
}

func TestFormFieldPublicViewKeepsOnlyFreshTranslations(t *testing.T) {
	t.Parallel()
	field := FormField{
		Key:   "meal",
		Label: "Mittagessen",
		Type:  FormFieldSelect,
		Options: []FormFieldOption{{
			Label: "Vegetarisch", Value: "veg",
			Translations: Translations{"ru": {TranslationAttrLabel: {Text: "Вегетарианское", Source: "Vegetarisch"}}},
		}},
		Translations: Translations{"ru": {TranslationAttrLabel: {Text: "Питание", Source: "Essen"}}},
	}

	public := PublicFormFields([]FormField{field})[0]

	assert.Nil(t, public.Translations, "the label changed from Essen to Mittagessen")
	assert.Equal(t, Translations{"ru": {TranslationAttrLabel: {Text: "Вегетарианское"}}}, public.Options[0].Translations)
	assert.Equal(t, "Vegetarisch", field.Options[0].Translations["ru"][TranslationAttrLabel].Source,
		"the stored field keeps its sources")
}

func TestFormLegalBlockPublicTranslations(t *testing.T) {
	t.Parallel()
	block := FormLegalBlock{
		Key: ConsentKeyAGB, Kind: "terms", Title: "AGB", Label: "Ich stimme zu.", Text: "Bedingungen",
		Source: LegalBlockSourceStandard, DisplayMode: LegalBlockDisplayModeText,
		Translations: Translations{"en": {
			TranslationAttrTitle: {Text: "Terms", Source: "AGB"},
			TranslationAttrText:  {Text: "Conditions", Source: "Bedingungen"},
		}},
	}
	assert.Equal(t, Translations{"en": {
		TranslationAttrTitle: {Text: "Terms"},
		TranslationAttrText:  {Text: "Conditions"},
	}}, block.PublicTranslations())

	block.DisplayMode = LegalBlockDisplayModePDF
	assert.Equal(t, Translations{"en": {TranslationAttrTitle: {Text: "Terms"}}}, block.PublicTranslations(),
		"a PDF block renders a generated link text, not the stored text")
}
