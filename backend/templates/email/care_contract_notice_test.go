package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCareContractNoticeTemplate_Renders pins the confirmation of a
// termination or withdrawal (#4682): the heading of its kind, the parent and
// every paragraph of the Träger's confirmation text as stored.
func TestCareContractNoticeTemplate_Renders(t *testing.T) {
	t.Parallel()
	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err, "parse all email templates")

	for withdrawal, heading := range map[bool]string{false: "Bestätigung Ihrer Kündigung", true: "Bestätigung Ihres Widerrufs"} {
		var out bytes.Buffer
		require.NoError(t, tpl.ExecuteTemplate(&out, "care-contract-notice.html", map[string]any{
			"GuardianName": "Erika Muster", "SchoolName": "Ganztag im Quartier e.V.", "Withdrawal": withdrawal,
			"Paragraphs": []string{"wir bestätigen Ihre Erklärung für Lina Muster.", "Der Vertrag endet mit Ablauf des 31.12.2026."},
		}))
		body := out.String()
		for _, want := range []string{heading, "Guten Tag Erika Muster,", "wir bestätigen Ihre Erklärung für Lina Muster.",
			"Der Vertrag endet mit Ablauf des 31.12.2026.", "Ganztag im Quartier e.V."} {
			assert.Contains(t, body, template.HTMLEscapeString(want))
		}
	}
}
