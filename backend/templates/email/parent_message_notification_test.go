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
	t.Parallel()

	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)

	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err, "parse all email templates")
	require.NotNil(t, tpl.Lookup("parent-message-notification.html"), "template must register under its filename")

	var buf bytes.Buffer
	err = tpl.ExecuteTemplate(&buf, "parent-message-notification.html", map[string]any{
		"Subject":            "Neue Nachricht von der OGS",
		"GuardianFirstName":  "Erika",
		"GuardianLastName":   "Mustermann",
		"SchoolName":         "Musterschule",
		"BrandKicker":        "Neue Nachricht",
		"Greeting":           "Guten Tag Erika Mustermann,",
		"IntroText":          "Die OGS Musterschule hat Ihnen eine Nachricht geschrieben:",
		"ReplyLabel":         "Antworten",
		"FallbackHint":       "Falls der Button nicht funktioniert, kopieren Sie bitte diesen Link:",
		"PreferenceHint":     "Sie erhalten diese E-Mail, weil Sie in moto als sorgeberechtigte Person hinterlegt sind.",
		"FooterText":         "Diese E-Mail wurde im Auftrag von Musterschule versendet.",
		"PoweredByLabel":     "Unterstützt von",
		"SchoolLogoAlt":      "Logo der Schule",
		"DefaultBrandKicker": "Elternportal",
		"DefaultSchoolName":  "Ihre OGS",
		"MessagesURL":        "https://eltern.example.com/messages",
		"LogoURL":            "https://example.com/school-logo.png",
		"MotoLogoURL":        "https://example.com/moto.png",
	})
	require.NoError(t, err, "render parent-message-notification.html")

	out := buf.String()
	for _, want := range []string{
		"Neue Nachricht von der OGS",
		"Erika",
		"Musterschule",
		"https://eltern.example.com/messages",
		"Antworten",
		"sorgeberechtigte",
	} {
		assert.Contains(t, out, want)
	}
}

// TestParentMessageNotificationTemplate_OmitsMessageContent pins the data
// minimisation the mail was cut down to: it announces that a message arrived and
// links to the portal, it never carries the message text or the child's name.
func TestParentMessageNotificationTemplate_OmitsMessageContent(t *testing.T) {
	t.Parallel()

	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err)

	var buf bytes.Buffer
	err = tpl.ExecuteTemplate(&buf, "parent-message-notification.html", map[string]any{
		"Subject":        "Neue Nachricht von der OGS",
		"SchoolName":     "Musterschule",
		"Greeting":       "Guten Tag Erika Mustermann,",
		"IntroText":      "Die OGS Musterschule hat Ihnen eine Nachricht geschrieben:",
		"ReplyLabel":     "Antworten",
		"FallbackHint":   "Falls der Button nicht funktioniert, kopieren Sie bitte diesen Link:",
		"PreferenceHint": "Sie erhalten diese E-Mail, weil Sie in moto als sorgeberechtigte Person hinterlegt sind.",
		"MessagesURL":    "https://eltern.example.com/messages",
		// Deliberately supplied and deliberately unused by the template.
		"ChildName": "Felix Mustermann",
		"Preview":   "Guten Tag, bitte melden Sie sich kurz bei uns.",
	})
	require.NoError(t, err)

	out := buf.String()
	assert.NotContains(t, out, "Felix Mustermann", "child name must stay out of the mail")
	assert.NotContains(t, out, "bitte melden Sie sich kurz bei uns", "message text must stay out of the mail")
}

// TestParentMessageNotificationTemplate_RendersWithoutSchool keeps the mail
// readable when the school name or the child could not be resolved: the sentence
// must still be a sentence, never "die OGS  hat Ihnen eine Nachricht zu ".
func TestParentMessageNotificationTemplate_RendersWithoutSchool(t *testing.T) {
	t.Parallel()

	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err)

	var buf bytes.Buffer
	err = tpl.ExecuteTemplate(&buf, "parent-message-notification.html", map[string]any{
		"Subject":           "Neue Nachricht von der OGS",
		"GuardianFirstName": "",
		"GuardianLastName":  "",
		"SchoolName":        "",
		"MessagesURL":       "https://eltern.example.com/messages",
		"Greeting":          "Guten Tag,",
		"IntroText":         "Die OGS hat Ihnen eine Nachricht geschrieben:",
		"ReplyLabel":        "Antworten",
		"FallbackHint":      "Link kopieren",
		"PreferenceHint":    "Benachrichtigungen verwalten",
	})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Die OGS hat Ihnen eine Nachricht geschrieben:")
	assert.Contains(t, out, "Guten Tag,")
}

func TestParentMessageNotificationTemplate_RendersLocalizedChrome(t *testing.T) {
	t.Parallel()

	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err)

	var buf bytes.Buffer
	err = tpl.ExecuteTemplate(&buf, "parent-message-notification.html", map[string]any{
		"Subject":            "New message from the OGS",
		"SchoolName":         "Example School",
		"BrandKicker":        "New message",
		"Greeting":           "Hello Erika,",
		"IntroText":          "The OGS sent you a message:",
		"ReplyLabel":         "Reply",
		"FallbackHint":       "Copy this link:",
		"PreferenceHint":     "Manage notifications in the app.",
		"MessagesURL":        "https://parents.example.com/messages",
		"FooterText":         "This email was sent on behalf of Example School.",
		"PoweredByLabel":     "Powered by",
		"SchoolLogoAlt":      "School logo",
		"DefaultBrandKicker": "Parent portal",
		"DefaultSchoolName":  "Your OGS",
	})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "This email was sent on behalf of Example School.")
	assert.Contains(t, out, "Powered by")
	assert.NotContains(t, out, "Diese E-Mail wurde")
	assert.NotContains(t, out, "Unterstützt von")
}
