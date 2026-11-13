package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHolidayRegistrationReceivedTemplate_Renders pins the receipt of a
// public holiday registration (#4661): the Träger, the period, each child with
// its booked weeks, the holiday guest mark and the next step.
func TestHolidayRegistrationReceivedTemplate_Renders(t *testing.T) {
	t.Parallel()
	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err, "parse all email templates")

	var out bytes.Buffer
	require.NoError(t, tpl.ExecuteTemplate(&out, "holiday-registration-received.html", map[string]any{
		"GuardianName": "Eva Muster",
		"SchoolName":   "Träger Bissendorf",
		"PeriodName":   "Winterferien 2027",
		"Binding":      true,
		"Children": []map[string]any{
			{"Name": "Lia Muster", "Guest": false, "Weeks": []string{"04.01.2027 bis 08.01.2027: OGS Nord, bis 16 Uhr (08:00 bis 16:00 Uhr)"}},
			{"Name": "Ben Muster", "Guest": true, "Weeks": []string{"11.01.2027 bis 15.01.2027: OGS Süd, bis 13 Uhr (08:00 bis 13:00 Uhr)"}},
		},
	}))
	body := out.String()
	for _, want := range []string{"Eva Muster", "Winterferien 2027", "Träger Bissendorf", "Lia Muster", "Ben Muster", "(Feriengast)",
		"OGS Süd, bis 13 Uhr", "Das ist noch keine Zusage.", "Die Anmeldung ist verbindlich."} {
		assert.Contains(t, body, template.HTMLEscapeString(want))
	}
	assert.NotContains(t, body, "Stand ansehen", "without a status link the mail shows no button")
}

// TestHolidayRegistrationReceivedTemplate_LinksTheStatusPage pins the status
// link of the receipt (#6676): a button and the plain link to copy.
func TestHolidayRegistrationReceivedTemplate_LinksTheStatusPage(t *testing.T) {
	t.Parallel()
	tpl, err := template.ParseGlob(filepath.Join(".", "*.html"))
	require.NoError(t, err, "parse all email templates")
	var out bytes.Buffer
	require.NoError(t, tpl.ExecuteTemplate(&out, "holiday-registration-received.html", map[string]any{
		"GuardianName": "Eva Muster", "SchoolName": "Träger Bissendorf", "PeriodName": "Winterferien 2027",
		"Children":  []map[string]any{{"Name": "Lia Muster", "Weeks": []string{"04.01.2027 bis 08.01.2027"}}},
		"StatusURL": "https://eltern.example.org/ferienanmeldung/status/abc",
	}))
	body := out.String()
	assert.Contains(t, body, "Stand ansehen")
	assert.Equal(t, 3, strings.Count(body, "https://eltern.example.org/ferienanmeldung/status/abc"), "the button and the link to copy")
}

// TestHolidayRegistrationReceivedTemplate_RendersTheDeclaration pins the
// parent's copy of the declaration (#5626): the typed name, when it was
// given, the version of the texts and every declaration in its words. A mail
// enqueued before #5626 has no declaration and renders without the section.
func TestHolidayRegistrationReceivedTemplate_RendersTheDeclaration(t *testing.T) {
	t.Parallel()
	tpl, err := template.ParseGlob(filepath.Join(".", "*.html"))
	require.NoError(t, err, "parse all email templates")
	content := map[string]any{
		"GuardianName": "Eva Muster",
		"SchoolName":   "Träger Bissendorf",
		"PeriodName":   "Winterferien 2027",
		"Children":     []map[string]any{{"Name": "Lia Muster", "Weeks": []string{"04.01.2027 bis 08.01.2027"}}},
		"Declaration": map[string]any{
			"Name": "Eva Muster", "GivenAt": "16.11.2026 um 09:30 Uhr", "Version": 3,
			"Statements": []map[string]any{
				{"Title": "Sorgerecht", "Text": "Ich darf die Anmeldung allein abgeben: Ich bin allein sorgeberechtigt oder die andere sorgeberechtigte Person ist einverstanden."},
				{"Title": "Fotos", "Text": "Ich bin mit Fotos einverstanden.", "Detail": "Die OGS macht Fotos.\nSie hängen im Flur."},
			},
		},
	}
	var out bytes.Buffer
	require.NoError(t, tpl.ExecuteTemplate(&out, "holiday-registration-received.html", content))
	body := out.String()
	for _, want := range []string{"Ihre Erklärung", "Name: Eva Muster", "Abgegeben am 16.11.2026 um 09:30 Uhr", "Fassung der Texte: 3",
		"Sorgerecht", "Ich bin allein sorgeberechtigt oder die andere sorgeberechtigte Person ist einverstanden.", "Fotos",
		"Ich bin mit Fotos einverstanden.", "Sie hängen im Flur."} {
		assert.Contains(t, body, template.HTMLEscapeString(want))
	}
	assert.NotContains(t, body, "Unterschrift", "the typed name is no signature")

	delete(content, "Declaration")
	out.Reset()
	require.NoError(t, tpl.ExecuteTemplate(&out, "holiday-registration-received.html", content))
	assert.NotContains(t, out.String(), "Ihre Erklärung", "a mail without a declaration has no section")
}
