package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStaffQualificationReminderTemplate_Render pins the reminder mail of
// #4808: the Leitung reads whose proof expires, the person reads their own.
func TestStaffQualificationReminderTemplate_Render(t *testing.T) {
	t.Parallel()

	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err)

	tests := []struct {
		name        string
		data        map[string]any
		mustContain []string
		mustNot     []string
	}{
		{
			name: "leitung before expiry",
			data: map[string]any{
				"SchoolName": "OGS Am Park", "FirstName": "Lea", "StaffName": "Max Muster",
				"QualificationName": "Erste Hilfe am Kind", "ExpiresOn": "28.12.2026",
				"LinkURL": "https://park.example.com/staff/7",
			},
			mustContain: []string{"Nachweis läuft bald ab", "Hallo Lea", "Nachweis von Max Muster", "Erste Hilfe am Kind: gültig bis 28.12.2026", "Person öffnen"},
			mustNot:     []string{"Ihr Nachweis"},
		},
		{
			name: "own on expiry day",
			data: map[string]any{
				"SchoolName": "OGS Am Park", "FirstName": "Max", "StaffName": "Max Muster",
				"QualificationName": "Erweitertes Führungszeugnis", "ExpiresOn": "02.11.2026", "ExpiryDay": true, "Own": true,
				"LinkURL": "https://park.example.com/profile",
			},
			mustContain: []string{"Nachweis läuft heute ab", "Ihr Nachweis „Erweitertes Führungszeugnis“ gilt nur noch heute", "Profil öffnen"},
			mustNot:     []string{"Max Muster"},
		},
		{
			name: "leitung after a caught-up expiry",
			data: map[string]any{
				"SchoolName": "OGS Am Park", "FirstName": "Lea", "StaffName": "Max Muster",
				"QualificationName": "Erste Hilfe am Kind", "ExpiresOn": "28.10.2026", "Lapsed": true,
			},
			mustContain: []string{"Nachweis ist abgelaufen", "Ein Nachweis von Max Muster ist abgelaufen"},
			mustNot:     []string{"läuft heute ab"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			require.NoError(t, tpl.ExecuteTemplate(&out, "staff-qualification-reminder.html", tt.data))
			for _, want := range tt.mustContain {
				assert.Contains(t, out.String(), want)
			}
			for _, unwanted := range tt.mustNot {
				assert.NotContains(t, out.String(), unwanted)
			}
		})
	}
}
