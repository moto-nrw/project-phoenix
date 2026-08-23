package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMFAEmailCodeTemplate_Renders verifies the mfa-email-code.html template
// renders with the data shape the MFA service ships, and that the rendered
// output carries the user-facing essentials (code, validity, anti-phishing
// disclaimer). Phase 4 of issue #1308.
func TestMFAEmailCodeTemplate_Renders(t *testing.T) {
	t.Parallel()

	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)

	pattern := filepath.Join(templatesDir, "*.html")
	tpl, err := template.ParseGlob(pattern)
	require.NoError(t, err, "parse all email templates")

	require.NotNil(t, tpl.Lookup("mfa-email-code.html"), "template must register under its filename")

	tests := []struct {
		name        string
		data        map[string]any
		mustContain []string
		mustNotHave []string
	}{
		{
			name: "code + validity + ip + trusted-device hint",
			data: map[string]any{
				"LogoURL":              "https://example.com/images/moto-logo-mit-schriftzug.png",
				"Code":                 "482157",
				"ExpiryMinutes":        10,
				"RequestIP":            "203.0.113.42",
				"TrustedDeviceEnabled": true,
				"TrustedDeviceDays":    30,
			},
			mustContain: []string{
				"482157",
				"10 Minuten",
				"203.0.113.42",
				"vertrauenswürdiges Gerät",
				"30 Tage",
				// Anti-phishing disclaimer
				"ignorieren Sie diese E-Mail",
				"fragt Sie niemals",
				// Branding
				`alt="moto"`,
				// Footer chrome
				"Unterstützt von",
			},
			// Critical: the template must NEVER include a clickable
			// "enter code" button — phishing-resistance per the design plan.
			mustNotHave: []string{
				`href="javascript:`,
				"Code eingeben</a>",
				"verify-mfa",
			},
		},
		{
			name: "trusted-device hint hidden when feature disabled",
			data: map[string]any{
				"LogoURL":              "https://example.com/logo.png",
				"Code":                 "000123",
				"ExpiryMinutes":        10,
				"RequestIP":            "",
				"TrustedDeviceEnabled": false,
				"TrustedDeviceDays":    0,
			},
			mustContain: []string{"000123"},
			mustNotHave: []string{
				"vertrauenswürdiges Gerät",
				// IP block also hidden when empty
				"Anfrage von IP-Adresse",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, tpl.ExecuteTemplate(&buf, "mfa-email-code.html", tt.data))
			rendered := buf.String()

			for _, snippet := range tt.mustContain {
				assert.Contains(t, rendered, snippet, "rendered output must contain %q", snippet)
			}
			for _, snippet := range tt.mustNotHave {
				assert.NotContains(t, rendered, snippet, "rendered output must NOT contain %q", snippet)
			}
		})
	}
}
