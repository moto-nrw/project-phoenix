package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBillingCorrectionLetterTemplate_Renders pins the mail that carries
// the letter of a Storno or a Gutschrift (#4690): the parent, the period,
// the number of the document, what to do next and where questions go.
func TestBillingCorrectionLetterTemplate_Renders(t *testing.T) {
	t.Parallel()
	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err, "parse all email templates")

	var out bytes.Buffer
	require.NoError(t, tpl.ExecuteTemplate(&out, "billing-correction-letter.html", map[string]any{
		"GuardianName": "Eva Muster",
		"SchoolName":   "Träger Bissendorf",
		"PeriodName":   "Herbstferien 2026",
		"Number":       "FE-2026-0009",
	}))
	body := out.String()
	for _, want := range []string{"Guten Tag Eva Muster,", "Herbstferien 2026", "FE-2026-0009", "Träger Bissendorf",
		"Ihre Rechnung hat sich geändert", "Öffnen Sie das Schreiben im Anhang.", "Die Kontaktdaten stehen im Schreiben."} {
		assert.Contains(t, body, template.HTMLEscapeString(want))
	}
}
