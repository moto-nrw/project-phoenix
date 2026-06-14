package localization

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed locales.json
var localesJSON []byte

type Locale struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	Fallback bool   `json:"fallback,omitempty"`
}

var (
	supportedLocales = loadLocales()
	localeByCode     = indexLocales(supportedLocales)
	fallbackLocale   = findFallbackLocale(supportedLocales)
)

func DefaultLocale() string {
	return fallbackLocale.Code
}

// IsSupported reports whether raw resolves to a registered locale. Region
// subtags are stripped first ("en-US" → "en"), mirroring NormalizeLocale, but
// unlike NormalizeLocale it does NOT fall back to the default — an unknown
// locale returns false so callers can reject it instead of silently coercing
// it to German.
func IsSupported(raw string) bool {
	code := strings.ToLower(strings.TrimSpace(raw))
	if code == "" {
		return false
	}
	if idx := strings.IndexAny(code, "-_"); idx >= 0 {
		code = code[:idx]
	}
	_, ok := localeByCode[code]
	return ok
}

func NormalizeLocale(raw string) string {
	code := strings.ToLower(strings.TrimSpace(raw))
	if code == "" {
		return DefaultLocale()
	}
	if idx := strings.IndexAny(code, "-_"); idx >= 0 {
		code = code[:idx]
	}
	if _, ok := localeByCode[code]; ok {
		return code
	}
	return DefaultLocale()
}

func loadLocales() []Locale {
	var locales []Locale
	if err := json.Unmarshal(localesJSON, &locales); err != nil {
		panic(fmt.Sprintf("localization: parse locales.json: %v", err))
	}
	if len(locales) == 0 {
		panic("localization: locales.json must contain at least one locale")
	}
	return locales
}

func indexLocales(locales []Locale) map[string]Locale {
	out := make(map[string]Locale, len(locales))
	for _, locale := range locales {
		code := strings.ToLower(strings.TrimSpace(locale.Code))
		if code == "" {
			panic("localization: locale code must not be empty")
		}
		if _, exists := out[code]; exists {
			panic(fmt.Sprintf("localization: duplicate locale code %q", code))
		}
		locale.Code = code
		out[code] = locale
	}
	return out
}

func findFallbackLocale(locales []Locale) Locale {
	for _, locale := range locales {
		if locale.Fallback {
			return locale
		}
	}
	return locales[0]
}
