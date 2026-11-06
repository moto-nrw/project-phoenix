package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCareContractTemplate_Renders pins the mail that carries the care
// contract of a Träger (#4678): the parent, the child, the offering, what to
// check and where the contract also lies.
func TestCareContractTemplate_Renders(t *testing.T) {
	t.Parallel()
	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err, "parse all email templates")

	var out bytes.Buffer
	require.NoError(t, tpl.ExecuteTemplate(&out, "care-contract.html", map[string]any{
		"GuardianName": "Erika Muster", "SchoolName": "Ganztag im Quartier e.V.",
		"ChildName": "Lina Muster", "Modules": "Kurzbetreuung und Ganztag",
	}))
	body := out.String()
	for _, want := range []string{"Guten Tag Erika Muster,", "Lina Muster", "Kurzbetreuung und Ganztag", "Ganztag im Quartier e.V.",
		"Prüfen Sie Betreuungstage, Beginn und Beitrag.", "unter „Dokumente“"} {
		assert.Contains(t, body, template.HTMLEscapeString(want))
	}
}
