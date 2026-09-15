// The OGS-message e-mail is the one mail that speaks the guardian's portal
// language (#1540, #3202). Every registered locale must answer with its own
// wording; a locale branch that silently returns another language's copy is
// the slip a spot-check misses.
package messaging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var parentEmailLocales = []string{"de", "en", "ru", "sq", "pl", "tr", "uk"}

func assertDistinctEmailField(t *testing.T, field string, got map[string]string) {
	t.Helper()
	seen := make(map[string]string, len(got))
	for locale, text := range got {
		assert.NotEmpty(t, text, "locale %s has no %s", locale, field)
		if other, clash := seen[text]; clash {
			t.Errorf("locale %s reuses the %s of %s: %q", locale, field, other, text)
		}
		seen[text] = locale
	}
}

func TestMessageEmailCopyCoversEveryLocale(t *testing.T) {
	t.Parallel()

	fields := map[string]func(parentMessageEmailCopy) string{
		"Subject":            func(c parentMessageEmailCopy) string { return c.Subject },
		"Kicker":             func(c parentMessageEmailCopy) string { return c.Kicker },
		"Greeting":           func(c parentMessageEmailCopy) string { return c.Greeting },
		"Intro":              func(c parentMessageEmailCopy) string { return c.Intro },
		"Reply":              func(c parentMessageEmailCopy) string { return c.Reply },
		"FallbackHint":       func(c parentMessageEmailCopy) string { return c.FallbackHint },
		"PreferenceHint":     func(c parentMessageEmailCopy) string { return c.PreferenceHint },
		"FooterText":         func(c parentMessageEmailCopy) string { return c.FooterText },
		"PoweredByLabel":     func(c parentMessageEmailCopy) string { return c.PoweredByLabel },
		"SchoolLogoAlt":      func(c parentMessageEmailCopy) string { return c.SchoolLogoAlt },
		"DefaultBrandKicker": func(c parentMessageEmailCopy) string { return c.DefaultBrandKicker },
		"DefaultSchoolName":  func(c parentMessageEmailCopy) string { return c.DefaultSchoolName },
	}
	for field, pick := range fields {
		got := map[string]string{}
		for _, locale := range parentEmailLocales {
			got[locale] = pick(messageEmailCopy(locale, "Anna", "Nowak", "Grundschule Musterstadt", ""))
		}
		assertDistinctEmailField(t, field, got)
	}
}

func TestMessageEmailCopyWithoutSchoolUsesNeutralFooter(t *testing.T) {
	t.Parallel()

	for _, locale := range parentEmailLocales {
		copy := messageEmailCopy(locale, "", "", "", "")
		assert.NotEmpty(t, copy.FooterText, "locale %s", locale)
		assert.NotEmpty(t, copy.Greeting, "locale %s", locale)
		assert.NotContains(t, copy.Intro, "  ", "locale %s intro has a double space without a school", locale)
	}
}

func TestMessageEmailCopyFallsBackToGerman(t *testing.T) {
	t.Parallel()

	assert.Equal(t, messageEmailCopy("de", "", "", "", ""), messageEmailCopy("fr", "", "", "", ""))
}
