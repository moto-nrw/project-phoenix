package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBillingNoticeAndInvoiceLetterTemplates_Render pins the mails that
// carry a fee notice, its Aufhebungs- or Änderungsbescheid and a holiday
// invoice that is no confirmation letter (#4688).
func TestBillingNoticeAndInvoiceLetterTemplates_Render(t *testing.T) {
	t.Parallel()
	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err, "parse all email templates")

	for name, wants := range map[string][]string{
		"billing-notice-letter.html": {"Ihr Gebührenbescheid", "Am Ende steht, wie Sie widersprechen können.",
			"Die Kontaktdaten stehen im Bescheid."},
		"billing-notice-correction-letter.html": {"Ihr Gebührenbescheid hat sich geändert", "Sie müssen jetzt weniger bezahlen.",
			"Dann melden Sie sich bitte beim Träger."},
		// The change notice of a Jahresbescheid (#6347) may raise or lower
		// the amount, so it promises neither.
		"billing-care-change-notice-letter.html": {"Ihr Gebührenbescheid wurde geändert", "Er ersetzt Ihren bisherigen Bescheid",
			"Am Ende steht, wie Sie widersprechen können."},
		// The mail names no reference of its own: the letter says what to
		// give, the debtor number or the invoice number (#4688, #6175).
		"billing-invoice-letter.html": {"Ihre Rechnung", "Dort steht, ob und bis wann Sie etwas bezahlen müssen und welchen Verwendungszweck Sie angeben.",
			"Die Kontaktdaten stehen in der Rechnung."},
	} {
		var out bytes.Buffer
		require.NoError(t, tpl.ExecuteTemplate(&out, name, map[string]any{
			"GuardianName": "Eva Muster", "SchoolName": "Gemeinde Hardt", "PeriodName": "Herbstferien 2026", "Number": "GB-2026-0001",
		}), name)
		body := out.String()
		for _, want := range append(wants, "Guten Tag Eva Muster,", "Herbstferien 2026", "GB-2026-0001", "Gemeinde Hardt") {
			assert.Contains(t, body, template.HTMLEscapeString(want), name)
		}
		assert.NotContains(t, body, "Rechnungsnummer an", "%s names no reference the letter may not use", name)
	}
}
