package application

import (
	"context"
	"testing"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTemplateLegalBlocks_FiltersDisabledAndKeepsCustomKeys(t *testing.T) {
	t.Parallel()

	blocks := buildTemplateLegalBlocks([]enrollmentNative.FormLegalBlock{
		{
			Key:       enrollmentNative.ConsentKeyDataProcessing,
			Kind:      enrollmentNative.LegalBlockKindPrivacyNotice,
			Title:     "Datenschutz",
			Label:     "Zur Kenntnis genommen.",
			Required:  true,
			Enabled:   true,
			SortOrder: 10,
			Source:    enrollmentNative.LegalBlockSourceStandard,
		},
		{
			Key:       "custom_pool",
			Kind:      enrollmentNative.LegalBlockKindConsent,
			Title:     "Schwimmbad",
			Label:     "Mein Kind darf teilnehmen.",
			Required:  false,
			Enabled:   true,
			SortOrder: 20,
			Source:    enrollmentNative.LegalBlockSourceCustom,
		},
		{
			Key:     enrollmentNative.ConsentKeyPhoto,
			Kind:    enrollmentNative.LegalBlockKindConsent,
			Title:   "Foto",
			Label:   "Foto",
			Enabled: false,
		},
	})

	require.Len(t, blocks, 2)
	assert.Equal(t, enrollmentNative.ConsentKeyDataProcessing, blocks[0].Key)
	assert.True(t, blocks[0].Required)
	assert.Equal(t, "custom_pool", blocks[1].Key)
	assert.False(t, blocks[1].Required)
}

func TestResolveRequiredConsents_UsesTemplateLegalBlocks(t *testing.T) {
	t.Parallel()

	svc := NewIntake(IntakeDependencies{})
	schema := &enrollmentNative.FormSchema{
		LegalBlocks: []enrollmentNative.FormLegalBlock{
			{
				Key:      enrollmentNative.ConsentKeyDataProcessing,
				Kind:     enrollmentNative.LegalBlockKindPrivacyNotice,
				Title:    "Datenschutz",
				Label:    "Zur Kenntnis genommen.",
				Required: true,
				Enabled:  true,
				Source:   enrollmentNative.LegalBlockSourceStandard,
			},
			{
				Key:      "custom_pool",
				Kind:     enrollmentNative.LegalBlockKindConsent,
				Title:    "Schwimmbad",
				Label:    "Mein Kind darf teilnehmen.",
				Required: true,
				Enabled:  true,
				Source:   enrollmentNative.LegalBlockSourceCustom,
			},
			{
				Key:      enrollmentNative.ConsentKeyAGB,
				Kind:     enrollmentNative.LegalBlockKindTerms,
				Title:    "AGB",
				Label:    "AGB",
				Required: true,
				Enabled:  false,
				Source:   enrollmentNative.LegalBlockSourceStandard,
			},
		},
	}

	blocks, err := svc.resolveSubmissionLegalBlocks(context.Background(), schema)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{enrollmentNative.ConsentKeyDataProcessing, "custom_pool"}, requiredConsentKeys(blocks))
}

func TestBuildLegalBlocks_RequiresEnabledTogglesForEveryStandardBlock(t *testing.T) {
	t.Parallel()

	texts := enrollmentNative.LegalTexts{
		AGB:                 "AGB Text",
		DSGVO:               "DSGVO Text",
		EmailContact:        "E-Mail Text",
		Photo:               "Foto Text",
		TermsEnabled:        true,
		DSGVOEnabled:        false,
		EmailContactEnabled: false,
		PhotoEnabled:        false,
	}

	blocks := buildLegalBlocks(texts)

	require.Len(t, blocks, 1)
	assert.Equal(t, enrollmentNative.ConsentKeyAGB, blocks[0].Key)
}

func TestBuildLegalBlocks_DefaultAGBSourceUsesTextWhenDocumentExists(t *testing.T) {
	t.Parallel()

	texts := enrollmentNative.LegalTexts{
		AGB:            "AGB Text",
		AGBDocumentURL: "/uploads/enrollment-legal-documents/tenant-1.pdf",
		TermsEnabled:   true,
	}

	blocks := buildLegalBlocks(texts)

	require.Len(t, blocks, 1)
	assert.Equal(t, "AGB Text", blocks[0].Text)
}

func TestBuildLegalBlocks_PDFAGBSourceUsesDocumentLinkOnly(t *testing.T) {
	t.Parallel()

	texts := enrollmentNative.LegalTexts{
		AGB:            "AGB Text",
		AGBDocumentURL: "/uploads/enrollment-legal-documents/tenant-1.pdf",
		AGBDisplayMode: legalAGBDisplayModePDF,
		TermsEnabled:   true,
	}

	blocks := buildLegalBlocks(texts)

	require.Len(t, blocks, 1)
	assert.NotContains(t, blocks[0].Text, "AGB Text")
	assert.Contains(t, blocks[0].Text, "AGB-Dokument öffnen")
	assert.Contains(t, blocks[0].Text, "/api/public/enrollment-legal-documents/tenant-1.pdf")
}

func TestBuildTemplateLegalBlocks_PDFAGBSourceUsesDocumentURL(t *testing.T) {
	t.Parallel()

	blocks := buildTemplateLegalBlocks([]enrollmentNative.FormLegalBlock{
		{
			Key:         enrollmentNative.ConsentKeyAGB,
			Kind:        enrollmentNative.LegalBlockKindTerms,
			Title:       "AGB",
			Label:       "AGB akzeptieren",
			Text:        "Textquelle bleibt gespeichert",
			Required:    true,
			Enabled:     true,
			SortOrder:   10,
			Source:      enrollmentNative.LegalBlockSourceStandard,
			DisplayMode: enrollmentNative.LegalBlockDisplayModePDF,
			DocumentURL: "/uploads/enrollment-form-legal-documents/1_terms.pdf",
		},
	})

	require.Len(t, blocks, 1)
	assert.NotContains(t, blocks[0].Text, "Textquelle bleibt gespeichert")
	assert.Contains(t, blocks[0].Text, "AGB-Dokument öffnen")
	assert.Contains(t, blocks[0].Text, "/api/public/enrollment-form-legal-documents/1_terms.pdf")
}

func TestBuildLegalBlocks_ShowsAllEnabledContentfulStandardBlocks(t *testing.T) {
	t.Parallel()

	texts := enrollmentNative.LegalTexts{
		AGB:                 "AGB Text",
		DSGVO:               "DSGVO Text",
		EmailContact:        "E-Mail Text",
		Photo:               "Foto Text",
		TermsEnabled:        true,
		DSGVOEnabled:        true,
		EmailContactEnabled: true,
		PhotoEnabled:        true,
	}

	blocks := buildLegalBlocks(texts)

	require.Len(t, blocks, 4)
	assert.Equal(t, enrollmentNative.ConsentKeyAGB, blocks[0].Key)
	assert.Equal(t, enrollmentNative.ConsentKeyDataProcessing, blocks[1].Key)
	assert.Equal(t, enrollmentNative.ConsentKeyPhoto, blocks[2].Key)
	assert.Equal(t, enrollmentNative.ConsentKeyEmailContact, blocks[3].Key)
}

func TestResolveRequiredConsents_AllDisabledTemplateFallsBackToSettings(t *testing.T) {
	t.Parallel()

	// A template whose blocks are ALL disabled must behave like a template
	// without blocks: the tenant-wide settings contract stays in force, so
	// the DSGVO acknowledgment cannot be erased by an empty snapshot.
	svc := NewIntake(IntakeDependencies{Settings: intakeSettingsFake{legal: LegalSettings{
		DSGVO:        "DSGVO Text",
		DSGVOEnabled: true,
	}}})
	schema := &enrollmentNative.FormSchema{
		LegalBlocks: []enrollmentNative.FormLegalBlock{
			{
				Key:     enrollmentNative.ConsentKeyDataProcessing,
				Kind:    enrollmentNative.LegalBlockKindPrivacyNotice,
				Title:   "Datenschutz",
				Label:   "Zur Kenntnis genommen.",
				Enabled: false,
			},
		},
	}

	blocks, err := svc.resolveSubmissionLegalBlocks(context.Background(), schema)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{enrollmentNative.ConsentKeyDataProcessing}, requiredConsentKeys(blocks))
}

func TestBuildTemplateLegalBlocks_SortsBySortOrder(t *testing.T) {
	t.Parallel()

	blocks := buildTemplateLegalBlocks([]enrollmentNative.FormLegalBlock{
		{Key: "custom_b", Kind: enrollmentNative.LegalBlockKindConsent, Title: "B", Label: "B", Enabled: true, SortOrder: 30},
		{Key: "custom_a", Kind: enrollmentNative.LegalBlockKindConsent, Title: "A", Label: "A", Enabled: true, SortOrder: 10},
		{Key: "custom_c", Kind: enrollmentNative.LegalBlockKindConsent, Title: "C", Label: "C", Enabled: true, SortOrder: 20},
	})

	require.Len(t, blocks, 3)
	assert.Equal(t, "custom_a", blocks[0].Key)
	assert.Equal(t, "custom_c", blocks[1].Key)
	assert.Equal(t, "custom_b", blocks[2].Key)
}

func TestFilterConsentFlags_DropsKeysOutsideContract(t *testing.T) {
	t.Parallel()

	blocks := []enrollmentNative.LegalBlock{
		{Key: enrollmentNative.ConsentKeyDataProcessing},
		{Key: "custom_pool"},
	}
	flags := map[string]any{
		enrollmentNative.ConsentKeyDataProcessing: true,
		"custom_pool":  false,
		"smuggled_key": true,
	}

	filtered := filterConsentFlags(flags, blocks)

	assert.Equal(t, map[string]any{
		enrollmentNative.ConsentKeyDataProcessing: true,
		"custom_pool": false,
	}, filtered)
}
