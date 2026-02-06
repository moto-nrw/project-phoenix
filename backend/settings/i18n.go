package settings

import (
	"sync"
)

// Locale represents a language code
type Locale string

const (
	LocaleDE Locale = "de"
	LocaleEN Locale = "en"
)

// DefaultLocale is the default locale for translations
var DefaultLocale Locale = LocaleDE

// translations holds all translations
var (
	translations    = make(map[Locale]map[string]string)
	translationsMu  sync.RWMutex
	currentLocale   = DefaultLocale
	currentLocaleMu sync.RWMutex
)

// SetLocale sets the current locale
func SetLocale(locale Locale) {
	currentLocaleMu.Lock()
	defer currentLocaleMu.Unlock()
	currentLocale = locale
}

// GetLocale returns the current locale
func GetLocale() Locale {
	currentLocaleMu.RLock()
	defer currentLocaleMu.RUnlock()
	return currentLocale
}

// RegisterTranslations registers translations for a locale
func RegisterTranslations(locale Locale, trans map[string]string) {
	translationsMu.Lock()
	defer translationsMu.Unlock()

	if translations[locale] == nil {
		translations[locale] = make(map[string]string)
	}

	for key, value := range trans {
		translations[locale][key] = value
	}
}

// T translates a key using the current locale
func T(key string) string {
	return TL(GetLocale(), key)
}

// TL translates a key using a specific locale
func TL(locale Locale, key string) string {
	translationsMu.RLock()
	defer translationsMu.RUnlock()

	if trans, ok := translations[locale]; ok {
		if value, found := trans[key]; found {
			return value
		}
	}

	// Fallback to default locale
	if locale != DefaultLocale {
		if trans, ok := translations[DefaultLocale]; ok {
			if value, found := trans[key]; found {
				return value
			}
		}
	}

	// Return the key itself if no translation found
	return key
}

// TranslatedDefinition wraps a Definition with i18n support
type TranslatedDefinition struct {
	Definition
	// LabelKey is the translation key for the label
	LabelKey string
	// DescriptionKey is the translation key for the description
	DescriptionKey string
}

// GetLabel returns the translated label, or falls back to the static Label
func (d *TranslatedDefinition) GetLabel() string {
	if d.LabelKey != "" {
		translated := T(d.LabelKey)
		if translated != d.LabelKey {
			return translated
		}
	}
	return d.Label
}

// GetDescription returns the translated description, or falls back to the static Description
func (d *TranslatedDefinition) GetDescription() string {
	if d.DescriptionKey != "" {
		translated := T(d.DescriptionKey)
		if translated != d.DescriptionKey {
			return translated
		}
	}
	return d.Description
}

// CategoryTranslations holds translations for category names
// Add translations here when you use categories in your setting definitions
var categoryTranslations = map[Locale]map[string]string{
	LocaleDE: {
		// Add German translations for your category keys here
		// Example: "general": "Allgemein",
	},
	LocaleEN: {
		// Add English translations for your category keys here
		// Example: "general": "General",
	},
}

// TranslateCategory translates a category key
func TranslateCategory(key string) string {
	return TranslateCategoryL(GetLocale(), key)
}

// TranslateCategoryL translates a category key for a specific locale
func TranslateCategoryL(locale Locale, key string) string {
	if trans, ok := categoryTranslations[locale]; ok {
		if value, found := trans[key]; found {
			return value
		}
	}

	// Fallback to default locale
	if locale != DefaultLocale {
		if trans, ok := categoryTranslations[DefaultLocale]; ok {
			if value, found := trans[key]; found {
				return value
			}
		}
	}

	return key
}

// TabTranslations holds translations for tab names
// Add translations here when you register tabs in settings/definitions/tabs.go
var tabTranslations = map[Locale]map[string]string{
	LocaleDE: {
		// Add German translations for your tab keys here
		// Example: "general": "Allgemein",
	},
	LocaleEN: {
		// Add English translations for your tab keys here
		// Example: "general": "General",
	},
}

// TranslateTab translates a tab key
func TranslateTab(key string) string {
	return TranslateTabL(GetLocale(), key)
}

// TranslateTabL translates a tab key for a specific locale
func TranslateTabL(locale Locale, key string) string {
	if trans, ok := tabTranslations[locale]; ok {
		if value, found := trans[key]; found {
			return value
		}
	}

	// Fallback to default locale
	if locale != DefaultLocale {
		if trans, ok := tabTranslations[DefaultLocale]; ok {
			if value, found := trans[key]; found {
				return value
			}
		}
	}

	return key
}

func init() {
	// Initialize with default German translations
	RegisterTranslations(LocaleDE, map[string]string{
		// Common labels
		"settings.save":    "Speichern",
		"settings.cancel":  "Abbrechen",
		"settings.reset":   "Zurücksetzen",
		"settings.default": "Standard",
	})

	RegisterTranslations(LocaleEN, map[string]string{
		// Common labels
		"settings.save":    "Save",
		"settings.cancel":  "Cancel",
		"settings.reset":   "Reset",
		"settings.default": "Default",
	})
}
