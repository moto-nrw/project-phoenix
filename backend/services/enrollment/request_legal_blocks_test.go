package enrollment

import (
	"context"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTemplateLegalBlocks_FiltersDisabledAndKeepsCustomKeys(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

	blocks, err := svc.resolveSubmissionLegalBlocks(context.Background(), schema)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{enrollmentModels.ConsentKeyDataProcessing, "custom_pool"}, requiredConsentKeys(blocks))
}

func TestBuildLegalBlocks_RequiresEnabledTogglesForEveryStandardBlock(t *testing.T) {
	t.Parallel()

	texts := LegalTexts{
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
	assert.Equal(t, enrollmentModels.ConsentKeyAGB, blocks[0].Key)
}

func TestBuildLegalBlocks_DefaultAGBSourceUsesTextWhenDocumentExists(t *testing.T) {
	t.Parallel()

	texts := LegalTexts{
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

	texts := LegalTexts{
		AGB:            "AGB Text",
		AGBDocumentURL: "/uploads/enrollment-legal-documents/tenant-1.pdf",
		AGBDisplayMode: configModel.EnrollmentLegalAGBDisplayModePDF,
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

	blocks := buildTemplateLegalBlocks([]enrollmentModels.FormLegalBlock{
		{
			Key:         enrollmentModels.ConsentKeyAGB,
			Kind:        enrollmentModels.LegalBlockKindTerms,
			Title:       "AGB",
			Label:       "AGB akzeptieren",
			Text:        "Textquelle bleibt gespeichert",
			Required:    true,
			Enabled:     true,
			SortOrder:   10,
			Source:      enrollmentModels.LegalBlockSourceStandard,
			DisplayMode: enrollmentModels.LegalBlockDisplayModePDF,
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

	texts := LegalTexts{
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
	assert.Equal(t, enrollmentModels.ConsentKeyAGB, blocks[0].Key)
	assert.Equal(t, enrollmentModels.ConsentKeyDataProcessing, blocks[1].Key)
	assert.Equal(t, enrollmentModels.ConsentKeyPhoto, blocks[2].Key)
	assert.Equal(t, enrollmentModels.ConsentKeyEmailContact, blocks[3].Key)
}

// legalSettingsStub is a minimal RequestSettingsResolver for the internal
// legal-block tests: every key present in values counts as overridden.
type legalSettingsStub struct {
	values map[string]string
	bools  map[string]bool
}

func (s *legalSettingsStub) HasTenantOverride(_ context.Context, key string) (bool, error) {
	if _, ok := s.values[key]; ok {
		return true, nil
	}
	if _, ok := s.bools[key]; ok {
		return true, nil
	}
	return false, nil
}

func (s *legalSettingsStub) ResolveString(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *legalSettingsStub) ResolveBool(_ context.Context, key string) (bool, error) {
	return s.bools[key], nil
}

func (s *legalSettingsStub) ResolveInt(_ context.Context, _ string) (int, error) { return 0, nil }

func TestResolveRequiredConsents_AllDisabledTemplateFallsBackToSettings(t *testing.T) {
	t.Parallel()

	// A template whose blocks are ALL disabled must behave like a template
	// without blocks: the tenant-wide settings contract stays in force, so
	// the DSGVO acknowledgment cannot be erased by an empty snapshot.
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{Settings: &legalSettingsStub{
		values: map[string]string{
			configModel.KeyEnrollmentLegalDSGVOText: "DSGVO Text",
		},
		bools: map[string]bool{
			configModel.KeyEnrollmentLegalDSGVOEnabled: true,
		},
	}}}
	schema := &enrollmentModels.FormSchema{
		LegalBlocks: []enrollmentModels.FormLegalBlock{
			{
				Key:     enrollmentModels.ConsentKeyDataProcessing,
				Kind:    enrollmentModels.LegalBlockKindPrivacyNotice,
				Title:   "Datenschutz",
				Label:   "Zur Kenntnis genommen.",
				Enabled: false,
			},
		},
	}

	blocks, err := svc.resolveSubmissionLegalBlocks(context.Background(), schema)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{enrollmentModels.ConsentKeyDataProcessing}, requiredConsentKeys(blocks))
}

func TestBuildTemplateLegalBlocks_SortsBySortOrder(t *testing.T) {
	t.Parallel()

	blocks := buildTemplateLegalBlocks([]enrollmentModels.FormLegalBlock{
		{Key: "custom_b", Kind: enrollmentModels.LegalBlockKindConsent, Title: "B", Label: "B", Enabled: true, SortOrder: 30},
		{Key: "custom_a", Kind: enrollmentModels.LegalBlockKindConsent, Title: "A", Label: "A", Enabled: true, SortOrder: 10},
		{Key: "custom_c", Kind: enrollmentModels.LegalBlockKindConsent, Title: "C", Label: "C", Enabled: true, SortOrder: 20},
	})

	require.Len(t, blocks, 3)
	assert.Equal(t, "custom_a", blocks[0].Key)
	assert.Equal(t, "custom_c", blocks[1].Key)
	assert.Equal(t, "custom_b", blocks[2].Key)
}

func TestFilterConsentFlags_DropsKeysOutsideContract(t *testing.T) {
	t.Parallel()

	blocks := []LegalBlock{
		{Key: enrollmentModels.ConsentKeyDataProcessing},
		{Key: "custom_pool"},
	}
	flags := map[string]any{
		enrollmentModels.ConsentKeyDataProcessing: true,
		"custom_pool":  false,
		"smuggled_key": true,
	}

	filtered := filterConsentFlags(flags, blocks)

	assert.Equal(t, map[string]any{
		enrollmentModels.ConsentKeyDataProcessing: true,
		"custom_pool": false,
	}, filtered)
}
