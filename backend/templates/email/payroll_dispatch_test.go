package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPayrollDispatchTemplates_Render pins the two mails of #4805: the link
// mail to the payroll office names the month, the link and its limits and
// carries no file; the notice to the managers names the reason and leads to
// /payroll.
func TestPayrollDispatchTemplates_Render(t *testing.T) {
	t.Parallel()

	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err)

	tests := []struct {
		name        string
		template    string
		data        map[string]any
		mustContain []string
	}{
		{
			name:     "first file",
			template: "payroll-export-link.html",
			data: map[string]any{
				"LogoURL": "https://example.com/logo.png", "SchoolName": "OGS Am Park", "MonthLabel": "September 2026",
				"LinkURL": "https://example.com/lohnbuero/7.token", "ExpiresAt": "19.10.2026",
			},
			mustContain: []string{"Monatsdatei September 2026", "OGS Am Park hat den Monat September 2026 abgeschlossen",
				"Datei herunterladen", "https://example.com/lohnbuero/7.token", "bis 19.10.2026", "nur einmal"},
		},
		{
			name:     "correction",
			template: "payroll-export-link.html",
			data: map[string]any{
				"LogoURL": "https://example.com/logo.png", "SchoolName": "OGS Am Park", "MonthLabel": "September 2026",
				"LinkURL": "https://example.com/lohnbuero/7.token", "ExpiresAt": "19.10.2026", "Correction": true,
			},
			mustContain: []string{"Korrektur der Monatsdatei September 2026", "ersetzt die vorherige"},
		},
		{
			name:     "notice",
			template: "payroll-dispatch-problem.html",
			data: map[string]any{
				"LogoURL": "https://example.com/logo.png", "FirstName": "Lea", "MonthLabel": "September 2026",
				"ReasonText": "Bei einigen Mitarbeitenden fehlt die Personalnummer.", "LinkURL": "https://park.example.com/payroll",
			},
			mustContain: []string{"Monatsdatei September 2026 nicht verschickt", "Hallo Lea",
				"fehlt die Personalnummer", "https://park.example.com/payroll", "Abrechnung öffnen"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			require.NoError(t, tpl.ExecuteTemplate(&out, tt.template, tt.data))
			for _, want := range tt.mustContain {
				assert.Contains(t, out.String(), want)
			}
		})
	}
}
