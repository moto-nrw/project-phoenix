package enrollment

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func TestNormalizeCareOfferingTranslations(t *testing.T) {
	t.Parallel()

	t.Run("stores the canonical document", func(t *testing.T) {
		t.Parallel()
		offering := &enrollmentModels.CareOffering{
			Translations: json.RawMessage(`{"RU":{"name":{"text":" Продлёнка ","source":"OGS"},"description":{"text":" "}}}`),
		}
		require.NoError(t, normalizeCareOfferingTranslations(offering))
		assert.JSONEq(t, `{"ru":{"name":{"text":"Продлёнка","source":"OGS"}}}`, string(offering.Translations))
	})

	t.Run("stores nothing when no translation remains", func(t *testing.T) {
		t.Parallel()
		offering := &enrollmentModels.CareOffering{Translations: json.RawMessage(`{"ru":{"name":{"text":""}}}`)}
		require.NoError(t, normalizeCareOfferingTranslations(offering))
		assert.Nil(t, offering.Translations)
	})

	t.Run("rejects malformed and foreign documents as invalid input", func(t *testing.T) {
		t.Parallel()
		for _, raw := range []string{`[]`, `{"ru":{"label":{"text":"x"}}}`, `{"de":{"name":{"text":"x"}}}`} {
			offering := &enrollmentModels.CareOffering{Translations: json.RawMessage(raw)}
			err := normalizeCareOfferingTranslations(offering)
			require.Error(t, err, raw)
			assert.True(t, errors.Is(err, ErrCareOfferingInvalid), raw)
		}
	})
}

func TestCareOfferingPublicTranslations(t *testing.T) {
	t.Parallel()
	description := "Betreuung bis 16 Uhr"
	offering := &enrollmentModels.CareOffering{
		Name:           "OGS lang",
		Description:    &description,
		SelectionGroup: "Betreuungsumfang",
		Translations: json.RawMessage(`{"ru":{
			"name":{"text":"Продлёнка","source":"OGS"},
			"description":{"text":"Присмотр до 16:00","source":"Betreuung bis 16 Uhr"},
			"selection_group":{"text":"Объём присмотра","source":"Betreuungsumfang"}}}`),
	}

	public, err := CareOfferingPublicTranslations(offering)
	require.NoError(t, err)
	// The name was renamed from "OGS" after it was translated.
	assert.Equal(t, enrollmentOwner.Translations{"ru": {
		enrollmentOwner.TranslationAttrDescription:    {Text: "Присмотр до 16:00"},
		enrollmentOwner.TranslationAttrSelectionGroup: {Text: "Объём присмотра"},
	}}, public)

	stored, err := CareOfferingTranslations(offering)
	require.NoError(t, err)
	assert.Equal(t, "OGS", stored["ru"][enrollmentOwner.TranslationAttrName].Source)
}
