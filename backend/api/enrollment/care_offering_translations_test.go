package enrollment

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

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

	public, err := careOfferingPublicTranslations(offering)
	require.NoError(t, err)
	// The name was renamed from "OGS" after it was translated.
	assert.Equal(t, capability.Translations{"ru": {
		capability.TranslationAttrDescription:    {Text: "Присмотр до 16:00"},
		capability.TranslationAttrSelectionGroup: {Text: "Объём присмотра"},
	}}, public)

	stored, err := careOfferingTranslations(offering)
	require.NoError(t, err)
	assert.Equal(t, "OGS", stored["ru"][capability.TranslationAttrName].Source)
}
