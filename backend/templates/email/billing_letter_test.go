package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBillingLetterTemplate_Renders pins the mail that carries the
// confirmation letter of a Träger (#4670): the parent, the period, the
// invoice number, what to do next and where questions go.
func TestBillingLetterTemplate_Renders(t *testing.T) {
	t.Parallel()
	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err, "parse all email templates")

	var out bytes.Buffer
	require.NoError(t, tpl.ExecuteTemplate(&out, "billing-letter.html", map[string]any{
		"GuardianName": "Eva Muster",
		"SchoolName":   "Träger Bissendorf",
		"PeriodName":   "Herbstferien 2026",
		"Number":       "FE-2026-0001",
	}))
	body := out.String()
	for _, want := range []string{"Guten Tag Eva Muster,", "Herbstferien 2026", "FE-2026-0001", "Träger Bissendorf",
		"Öffnen Sie das Schreiben im Anhang.", "Die Kontaktdaten stehen im Schreiben."} {
		assert.Contains(t, body, template.HTMLEscapeString(want))
	}
}
