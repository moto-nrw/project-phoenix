package enrollment

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// TranslationSourceLocale is the language schools write their form texts in.
// Translations exist only for the other portal languages; the German text is
// the attribute itself and the fallback whenever a translation is missing.
const TranslationSourceLocale = "de"

const (
	translationMaxLocales    = 16
	translationMaxTextLength = 20000
)

// Translatable attribute names. They match the JSON name of the German
// attribute they translate, so the frontend can resolve them generically.
const (
	TranslationAttrLabel       = "label"
	TranslationAttrHelpText    = "help_text"
	TranslationAttrContent     = "content"
	TranslationAttrTitle       = "title"
	TranslationAttrText        = "text"
	TranslationAttrName        = "name"
	TranslationAttrDescription = "description"
	// TranslationAttrSelectionGroup is the heading parents read above care
	// offerings that share a selection rule.
	TranslationAttrSelectionGroup = "selection_group"
)

// TranslatedText is one school-written translation (#3377). Source is the
// German text the translator saw. A translation is only shown to parents
// while Source still equals the current German text: after the school edits
// the German text, parents read German until someone confirms the
// translation again. Public responses never carry Source.
type TranslatedText struct {
	Text   string `json:"text"`
	Source string `json:"source,omitempty"`
}

// Translations maps locale → attribute → translated text.
type Translations map[string]map[string]TranslatedText

// Normalize trims every entry, drops empty ones, and rejects locales or
// attributes that cannot apply. It returns nil when nothing remains, so an
// untranslated entity stores no translations key at all.
func (t Translations) Normalize(allowedAttrs ...string) (Translations, error) {
	if len(t) > translationMaxLocales {
		return nil, fmt.Errorf("translations must not cover more than %d languages", translationMaxLocales)
	}
	out := make(Translations, len(t))
	for rawLocale, attrs := range t {
		locale := strings.ToLower(strings.TrimSpace(rawLocale))
		if !validTranslationLocale(locale) {
			return nil, fmt.Errorf("translation language %q is not valid", rawLocale)
		}
		normalized, err := normalizeLocaleTranslations(locale, attrs, allowedAttrs)
		if err != nil {
			return nil, err
		}
		if len(normalized) > 0 {
			out[locale] = normalized
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func normalizeLocaleTranslations(locale string, attrs map[string]TranslatedText, allowedAttrs []string) (map[string]TranslatedText, error) {
	out := make(map[string]TranslatedText, len(attrs))
	for attr, entry := range attrs {
		if !slices.Contains(allowedAttrs, attr) {
			return nil, fmt.Errorf("translation attribute %q is not translatable here", attr)
		}
		entry.Text = strings.TrimSpace(entry.Text)
		entry.Source = strings.TrimSpace(entry.Source)
		if entry.Text == "" {
			continue
		}
		if entry.Source == "" {
			return nil, fmt.Errorf("translation %s/%s requires a source", locale, attr)
		}
		if utf8.RuneCountInString(entry.Text) > translationMaxTextLength {
			return nil, fmt.Errorf("translation %s/%s must be at most %d characters", locale, attr, translationMaxTextLength)
		}
		out[attr] = entry
	}
	return out, nil
}

// Fresh returns the translations parents may read: only entries whose Source
// still equals the current German text in sources (attribute → German text),
// with Source removed. It returns nil when no entry qualifies.
func (t Translations) Fresh(sources map[string]string) Translations {
	var out Translations
	for locale, attrs := range t {
		for attr, entry := range attrs {
			current := strings.TrimSpace(sources[attr])
			if current == "" || entry.Text == "" || strings.TrimSpace(entry.Source) != current {
				continue
			}
			if out == nil {
				out = make(Translations, len(t))
			}
			if out[locale] == nil {
				out[locale] = make(map[string]TranslatedText, len(attrs))
			}
			out[locale][attr] = TranslatedText{Text: entry.Text}
		}
	}
	return out
}

// Texts flattens the translations to locale → attribute → text, the shape
// consent snapshots persist.
func (t Translations) Texts() map[string]map[string]string {
	if len(t) == 0 {
		return nil
	}
	out := make(map[string]map[string]string, len(t))
	for locale, attrs := range t {
		out[locale] = make(map[string]string, len(attrs))
		for attr, entry := range attrs {
			out[locale][attr] = entry.Text
		}
	}
	return out
}

// validTranslationLocale accepts a lowercase two- or three-letter language
// code other than the source language. The registry of portal languages is
// owned by the delivery platform; an unregistered code is never requested by
// a parent, so it stays inert instead of coupling this module to the registry.
func validTranslationLocale(locale string) bool {
	if locale == TranslationSourceLocale || len(locale) < 2 || len(locale) > 3 {
		return false
	}
	for _, r := range locale {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}
