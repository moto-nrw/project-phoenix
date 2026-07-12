package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTrustedDeviceAddedTemplate_Renders verifies that the security
// notification mail dispatched after a new trusted-device cookie is
// issued contains the user-actionable bits: device label, IP, timestamp,
// validity window, plus the explicit "wasn't you?" guidance and a link
// back to the security section so the user can revoke the device.
func TestTrustedDeviceAddedTemplate_Renders(t *testing.T) {
	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)

	pattern := filepath.Join(templatesDir, "*.html")
	tpl, err := template.ParseGlob(pattern)
	require.NoError(t, err, "parse all email templates")

	require.NotNil(t, tpl.Lookup("trusted-device-added.html"), "template must register under its filename")

	tests := []struct {
		name        string
		data        map[string]any
		mustContain []string
	}{
		{
			name: "happy path with IP",
			data: map[string]any{
				"LogoURL":     "https://example.com/images/moto-logo-mit-schriftzug.png",
				"SecurityURL": "https://demo.moto-app.de/settings?tab=settings-security",
				"DeviceLabel": "Chrome auf macOS",
				"IPAddress":   "203.0.113.42",
				"AddedAt":     "14.05.2026 09:42",
				"TrustedDays": 90,
			},
			mustContain: []string{
				"Chrome auf macOS",
				"203.0.113.42",
				"14.05.2026 09:42",
				"90",
				// "Was that you?" guidance
				"Waren Sie das nicht",
				// Primary self-service path — every user has access via /profile
				"Mein Profil",
				"Meine vertrauten Geräte",
				// Step 1 of the rescue checklist
				"Passwort",
				// Account-locked fallback: contact admin
				"Administration",
				// Branding chrome
				`alt="moto"`,
				"Unterstützt von",
			},
		},
		{
			name: "IP optional (template still renders without)",
			data: map[string]any{
				"LogoURL":     "https://example.com/logo.png",
				"SecurityURL": "https://demo.moto-app.de/settings",
				"DeviceLabel": "Safari auf iPhone",
				"IPAddress":   "",
				"AddedAt":     "01.06.2026 12:00",
				"TrustedDays": 30,
			},
			mustContain: []string{
				"Safari auf iPhone",
				"01.06.2026 12:00",
				"30",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, tpl.ExecuteTemplate(&buf, "trusted-device-added.html", tt.data))
			rendered := buf.String()

			for _, snippet := range tt.mustContain {
				assert.Contains(t, rendered, snippet, "rendered output must contain %q", snippet)
			}
		})
	}
}
