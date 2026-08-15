package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParentMessageNotificationTemplate_Renders verifies the "Neue Nachricht von
// der OGS" mail (#2307) renders with the payload the messaging service supplies.
func TestParentMessageNotificationTemplate_Renders(t *testing.T) {
	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)

	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err, "parse all email templates")
	require.NotNil(t, tpl.Lookup("parent-message-notification.html"), "template must register under its filename")

	var buf bytes.Buffer
	err = tpl.ExecuteTemplate(&buf, "parent-message-notification.html", map[string]any{
		"GuardianFirstName": "Erika",
		"GuardianLastName":  "Mustermann",
		"SchoolName":        "OGS Musterschule",
		"BrandKicker":       "Neue Nachricht",
		"ChildName":         "Felix Mustermann",
		"Preview":           "Guten Tag, bitte melden Sie sich kurz bei uns.",
		"MessagesURL":       "https://eltern.example.com/messages",
		"LogoURL":           "https://example.com/school-logo.png",
		"MotoLogoURL":       "https://example.com/moto.png",
	})
	require.NoError(t, err, "render parent-message-notification.html")

	out := buf.String()
	for _, want := range []string{
		"Neue Nachricht von der OGS",
		"Erika",
		"OGS Musterschule",
		"Felix Mustermann",
		"Guten Tag, bitte melden Sie sich kurz bei uns.",
		"https://eltern.example.com/messages",
		"Antworten",
		"Sorgeberechtigte",
	} {
		assert.Contains(t, out, want)
	}
}

// TestParentMessageNotificationTemplate_RendersWithoutSchool keeps the mail
// readable when the school name or the child could not be resolved: the sentence
// must still be a sentence, never "die OGS  hat Ihnen eine Nachricht zu ".
func TestParentMessageNotificationTemplate_RendersWithoutSchool(t *testing.T) {
	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err)

	var buf bytes.Buffer
	err = tpl.ExecuteTemplate(&buf, "parent-message-notification.html", map[string]any{
		"GuardianFirstName": "",
		"GuardianLastName":  "",
		"SchoolName":        "",
		"ChildName":         "",
		"Preview":           "Kurze Nachricht",
		"MessagesURL":       "https://eltern.example.com/messages",
	})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "die OGS hat Ihnen eine Nachricht geschrieben:")
	assert.Contains(t, out, "Guten Tag,")
}
