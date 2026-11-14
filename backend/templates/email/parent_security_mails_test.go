package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The parents portal's code and device notice (#7051) render the text they
// are handed and, like every code mail, carry no link.
func TestParentSecurityMailTemplatesRender(t *testing.T) {
	t.Parallel()
	tpl, err := template.ParseGlob(filepath.Join(".", "*.html"))
	require.NoError(t, err)

	var code bytes.Buffer
	require.NoError(t, tpl.ExecuteTemplate(&code, "parent-mfa-email-code.html", map[string]any{
		"Lang": "en", "Code": "482157", "Title": "Your sign-in code", "Validity": "Valid for 10 minutes.",
		"RequestFrom": "Request from IP address", "RequestIP": "203.0.113.42", "Tip": "", "Security": "moto never asks",
		"PoweredByLabel": "Powered by", "FooterText": "This email was sent automatically.",
	}))
	out := code.String()
	for _, want := range []string{`lang="en"`, "482157", "Your sign-in code", "203.0.113.42", "Powered by", "This email was sent automatically."} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "<a ", "a code mail carries no link")
	assert.NotContains(t, out, "Unterstützt von")

	var device bytes.Buffer
	require.NoError(t, tpl.ExecuteTemplate(&device, "parent-trusted-device-added.html", map[string]any{
		"Lang": "de", "Title": "Neues gemerktes Gerät", "DeviceName": "Gerät", "DeviceLabel": "Firefox auf Linux",
		"TimeName": "Zeitpunkt", "AddedAt": "14.11.2026 18:00", "NotYou": "Einstellungen → Anmeldung",
	}))
	assert.Contains(t, device.String(), "Firefox auf Linux")
	assert.Contains(t, device.String(), "Einstellungen → Anmeldung")
}
