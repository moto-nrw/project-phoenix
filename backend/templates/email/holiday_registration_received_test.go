package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
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
}
