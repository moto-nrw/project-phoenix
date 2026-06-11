package enrollment

import (
	"context"
	"testing"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTemplateLegalBlocks_FiltersDisabledAndKeepsCustomKeys(t *testing.T) {
	blocks := buildTemplateLegalBlocks([]enrollmentModels.FormLegalBlock{
		{
			Key:       enrollmentModels.ConsentKeyDataProcessing,
			Kind:      enrollmentModels.LegalBlockKindPrivacyNotice,
			Title:     "Datenschutz",
			Label:     "Zur Kenntnis genommen.",
			Required:  true,
			Enabled:   true,
			SortOrder: 10,
			Source:    enrollmentModels.LegalBlockSourceStandard,
		},
		{
			Key:       "custom_pool",
			Kind:      enrollmentModels.LegalBlockKindConsent,
			Title:     "Schwimmbad",
			Label:     "Mein Kind darf teilnehmen.",
			Required:  false,
			Enabled:   true,
			SortOrder: 20,
			Source:    enrollmentModels.LegalBlockSourceCustom,
		},
		{
			Key:     enrollmentModels.ConsentKeyPhoto,
			Kind:    enrollmentModels.LegalBlockKindConsent,
			Title:   "Foto",
			Label:   "Foto",
			Enabled: false,
		},
	})

	require.Len(t, blocks, 2)
	assert.Equal(t, enrollmentModels.ConsentKeyDataProcessing, blocks[0].Key)
	assert.True(t, blocks[0].Required)
	assert.Equal(t, "custom_pool", blocks[1].Key)
	assert.False(t, blocks[1].Required)
}

func TestResolveRequiredConsents_UsesTemplateLegalBlocks(t *testing.T) {
	svc := &requestService{}
	schema := &enrollmentModels.FormSchema{
		LegalBlocks: []enrollmentModels.FormLegalBlock{
			{
				Key:      enrollmentModels.ConsentKeyDataProcessing,
				Kind:     enrollmentModels.LegalBlockKindPrivacyNotice,
				Title:    "Datenschutz",
				Label:    "Zur Kenntnis genommen.",
				Required: true,
				Enabled:  true,
				Source:   enrollmentModels.LegalBlockSourceStandard,
			},
			{
				Key:      "custom_pool",
				Kind:     enrollmentModels.LegalBlockKindConsent,
				Title:    "Schwimmbad",
				Label:    "Mein Kind darf teilnehmen.",
				Required: true,
				Enabled:  true,
				Source:   enrollmentModels.LegalBlockSourceCustom,
			},
			{
				Key:      enrollmentModels.ConsentKeyAGB,
				Kind:     enrollmentModels.LegalBlockKindTerms,
				Title:    "AGB",
				Label:    "AGB",
				Required: true,
				Enabled:  false,
				Source:   enrollmentModels.LegalBlockSourceStandard,
			},
		},
	}

	required, err := svc.resolveRequiredConsents(context.Background(), schema)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{enrollmentModels.ConsentKeyDataProcessing, "custom_pool"}, required)
}
