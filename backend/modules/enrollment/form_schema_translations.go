package enrollment

import "fmt"

// Translations of the form schema's own texts (#3377): field label, help text
// and content, option labels, and a legal block's title, label and text.

func (f *FormField) normalizeTranslations() error {
	var err error
	f.Translations, err = f.Translations.Normalize(TranslationAttrLabel, TranslationAttrHelpText, TranslationAttrContent)
	if err != nil {
		return fmt.Errorf("form field %q: %w", f.Key, err)
	}
	for i := range f.Options {
		f.Options[i].Translations, err = f.Options[i].Translations.Normalize(TranslationAttrLabel)
		if err != nil {
			return fmt.Errorf("form field %q option %q: %w", f.Key, f.Options[i].Value, err)
		}
	}
	return nil
}

// PublicView returns the field as parents may read it: translations are
// reduced to the ones still matching the current German text.
func (f FormField) PublicView() FormField {
	f.Translations = f.Translations.Fresh(map[string]string{
		TranslationAttrLabel:    f.Label,
		TranslationAttrHelpText: f.HelpText,
		TranslationAttrContent:  f.Content,
	})
	if len(f.Options) > 0 {
		options := make([]FormFieldOption, len(f.Options))
		for i, option := range f.Options {
			option.Translations = option.Translations.Fresh(map[string]string{TranslationAttrLabel: option.Label})
			options[i] = option
		}
		f.Options = options
	}
	return f
}

// PublicFormFields maps fields to their PublicView, keeping a non-nil slice.
func PublicFormFields(fields []FormField) []FormField {
	out := make([]FormField, len(fields))
	for i, field := range fields {
		out[i] = field.PublicView()
	}
	return out
}

// PublicNameTranslations maps locale → the school's translation of the phase
// name, limited to translations that still match the German name. It is the
// flat shape for callers that must not depend on the Translations type.
func (p *Phase) PublicNameTranslations() map[string]string {
	fresh := p.Translations.Fresh(map[string]string{TranslationAttrName: p.Name})
	if len(fresh) == 0 {
		return nil
	}
	names := make(map[string]string, len(fresh))
	for locale, attrs := range fresh {
		names[locale] = attrs[TranslationAttrName].Text
	}
	return names
}

// validateDisplayMode is the display-mode step of FormLegalBlock.Validate. It
// lives here because the mode decides whether Text is translatable at all.
func (b *FormLegalBlock) validateDisplayMode() error {
	if b.DisplayMode == "" {
		b.DisplayMode = LegalBlockDisplayModeText
	}
	if b.DisplayMode != LegalBlockDisplayModeText && b.DisplayMode != LegalBlockDisplayModePDF {
		return fmt.Errorf("legal block %q has unknown display mode %q", b.Key, b.DisplayMode)
	}
	if b.DisplayMode == LegalBlockDisplayModePDF {
		if b.Key != ConsentKeyAGB || b.Source != LegalBlockSourceStandard {
			return fmt.Errorf("legal block %q cannot use PDF display mode", b.Key)
		}
		if b.Enabled && b.DocumentURL == "" {
			return fmt.Errorf("enabled legal block %q requires a PDF document", b.Key)
		}
	}
	return nil
}

// PublicTranslations returns the translations parents may read. A block shown
// as a PDF link renders generated text, so its Text translation does not apply.
func (b FormLegalBlock) PublicTranslations() Translations {
	sources := map[string]string{TranslationAttrTitle: b.Title, TranslationAttrLabel: b.Label}
	if b.DisplayMode != LegalBlockDisplayModePDF {
		sources[TranslationAttrText] = b.Text
	}
	return b.Translations.Fresh(sources)
}
