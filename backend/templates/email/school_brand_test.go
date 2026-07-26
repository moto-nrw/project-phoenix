package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchoolBrandTemplate_Renders verifies the school-brand branding variant.
// Enrollment emails render the shared "school-brand" partial (school logo +
// "Online-Anmeldung" kicker + school name) instead of the centered moto logo
// used by every other transactional email. Together with
// TestMFAEmailCodeTemplate_Renders (moto-brand variant) this covers both
// branding partials defined in header.html. Issue #1687.
func TestSchoolBrandTemplate_Renders(t *testing.T) {
	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)

	pattern := filepath.Join(templatesDir, "*.html")
	tpl, err := template.ParseGlob(pattern)
	require.NoError(t, err, "parse all email templates")

	require.NotNil(t, tpl.Lookup("enrollment-submitted.html"), "template must register under its filename")

	tests := []struct {
		name        string
		data        map[string]any
		mustContain []string
		mustNotHave []string
	}{
		{
			name: "school logo + name when LogoURL is set",
			data: map[string]any{
				"GuardianFirstName": "Erika",
				"GuardianLastName":  "Mustermann",
				"SchoolName":        "OGS Musterschule",
				"StatusURL":         "https://parents.example.com/status/abc",
				"LogoURL":           "https://example.com/school-logo.png",
				"MotoLogoURL":       "https://example.com/images/moto-logo-mit-schriftzug.png",
				"ChildNames":        []string{"Max Mustermann"},
			},
			mustContain: []string{
				// school-brand partial markup
				`class="school-brand"`,
				"Online-Anmeldung",
				"OGS Musterschule",
				// school logo uses the provided URL
				`class="school-logo"`,
				"https://example.com/school-logo.png",
				// footer chrome still present
				"Unterstützt von",
			},
			mustNotHave: []string{
				// must NOT fall back to the centered moto-brand logo block
				`class="brand"`,
				// no fallback placeholder element when a logo exists
				// (the .school-logo-fallback CSS rule always ships in <style>,
				// so assert on the rendered markup element, not the bare class
				// name)
				`class="school-logo-fallback"`,
			},
		},
		{
			name: "OGS fallback when LogoURL is empty",
			data: map[string]any{
				"GuardianFirstName": "Erika",
				"GuardianLastName":  "Mustermann",
				"SchoolName":        "OGS Musterschule",
				"StatusURL":         "https://parents.example.com/status/abc",
				"LogoURL":           "",
				"MotoLogoURL":       "",
				"ChildNames":        []string{"Max Mustermann"},
			},
			mustContain: []string{
				`class="school-logo-fallback"`,
				"OGS",
				"Online-Anmeldung",
				"OGS Musterschule",
			},
			mustNotHave: []string{
				// no school-logo img element when there is no URL
				`class="school-logo"`,
				`class="brand"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, tpl.ExecuteTemplate(&buf, "enrollment-submitted.html", tt.data))
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
