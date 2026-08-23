package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAppointmentNotificationTemplate_Renders verifies the shared calendar
// appointment (Termine) notification template renders with the payload the
// renderer supplies for published / updated / cancelled / reminder mails.
func TestAppointmentNotificationTemplate_Renders(t *testing.T) {
	t.Parallel()

	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)

	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err, "parse all email templates")
	require.NotNil(t, tpl.Lookup("appointment-notification.html"), "template must register under its filename")

	var buf bytes.Buffer
	err = tpl.ExecuteTemplate(&buf, "appointment-notification.html", map[string]any{
		"Title":             "Elternabend",
		"BrandKicker":       "Neuer Termin",
		"IntroText":         "es gibt einen neuen Termin für Sie.",
		"WhenText":          "02.04.2026, 18:00–19:30 Uhr",
		"Location":          "Aula",
		"SchoolName":        "OGS Musterschule",
		"GuardianFirstName": "Erika",
		"GuardianLastName":  "Mustermann",
		"PortalURL":         "https://parents.example.com",
		"LogoURL":           "https://example.com/school-logo.png",
		"MotoLogoURL":       "https://example.com/moto.png",
	})
	require.NoError(t, err, "render appointment-notification.html")

	out := buf.String()
	for _, want := range []string{
		"Elternabend",
		"02.04.2026, 18:00–19:30 Uhr",
		"Aula",
		"Erika",
		"https://parents.example.com",
		"OGS Musterschule",
	} {
		assert.Contains(t, out, want)
	}
}
