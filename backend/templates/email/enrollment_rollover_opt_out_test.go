package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRolloverOptOutTemplate_NamesTheTermsConfirmation pins the opt-out
// renewal mail (#6456): the care runs on either way, and when the new phase's
// form has required blocks the mail asks for the confirmation instead of
// saying nothing needs to be done.
func TestRolloverOptOutTemplate_NamesTheTermsConfirmation(t *testing.T) {
	t.Parallel()
	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err, "parse all email templates")

	render := func(termsRequired bool) string {
		var out bytes.Buffer
		require.NoError(t, tpl.ExecuteTemplate(&out, "enrollment-rollover-opt-out.html", map[string]any{
			"GuardianFirstName": "Erika", "GuardianLastName": "Muster", "SchoolName": "OGS Sonnenschule",
			"PhaseName": "Schuljahr 2027/28", "StatusURL": "https://parents.example.test/status/abc",
			"RenewalTermsRequired": termsRequired,
		}))
		return out.String()
	}

	withTerms := render(true)
	for _, want := range []string{"Die Betreuung Ihres Kindes läuft im neuen Zeitraum weiter.",
		"Bitte bestätigen Sie die Vertragsbedingungen.", "Für den Vertrag im neuen Zeitraum brauchen wir Ihre Zustimmung.",
		"Vertragsbedingungen bestätigen"} {
		assert.Contains(t, withTerms, template.HTMLEscapeString(want))
	}
	assert.NotContains(t, withTerms, template.HTMLEscapeString("Sie müssen dafür nichts tun."))

	withoutTerms := render(false)
	assert.Contains(t, withoutTerms, template.HTMLEscapeString("Sie müssen dafür nichts tun."))
	assert.Contains(t, withoutTerms, template.HTMLEscapeString("Status ansehen oder abmelden"))
	assert.NotContains(t, withoutTerms, template.HTMLEscapeString("Vertragsbedingungen bestätigen"))
}
