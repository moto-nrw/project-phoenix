package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBillingCareLetterTemplate_Renders pins the mail that carries the
// letter of a monthly invoice (#4684): the parent, the month, the number,
// what to do next and where questions go.
func TestBillingCareLetterTemplate_Renders(t *testing.T) {
	t.Parallel()
	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err, "parse all email templates")

	var out bytes.Buffer
	require.NoError(t, tpl.ExecuteTemplate(&out, "billing-care-letter.html", map[string]any{
		"GuardianName": "Eva Muster",
		"SchoolName":   "SJR Pforzheim",
		"PeriodName":   "Monatsbeitrag Januar 2027",
		"Number":       "MB-2027-0001",
	}))
	body := out.String()
	for _, want := range []string{"Guten Tag Eva Muster,", "Monatsbeitrag Januar 2027", "MB-2027-0001", "SJR Pforzheim",
		"Ihre Rechnung für die Betreuung", "Öffnen Sie die Rechnung im Anhang.", "Die Kontaktdaten stehen in der Rechnung."} {
		assert.Contains(t, body, template.HTMLEscapeString(want))
	}
}
