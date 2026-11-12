package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHolidayParentAccountOfferTemplate_Renders pins the offer of a parent
// account a holiday registration mails (ADR 0216 point 4): one button to set
// up the account, how long the link holds, and the sign-in for an address
// that already has an account. The mail names no child.
func TestHolidayParentAccountOfferTemplate_Renders(t *testing.T) {
	t.Parallel()
	tpl, err := template.ParseGlob(filepath.Join(".", "*.html"))
	require.NoError(t, err, "parse all email templates")

	var out bytes.Buffer
	require.NoError(t, tpl.ExecuteTemplate(&out, "holiday-parent-account-offer.html", map[string]any{
		"GuardianName": "Eva Muster",
		"SchoolName":   "Träger Bissendorf",
		"PeriodName":   "Winterferien 2027",
		"SetupURL":     "https://eltern.example.org/ferienkonto/setup-token",
		"LoginURL":     "https://eltern.example.org/login",
	}))
	body := out.String()
	for _, want := range []string{"Ihr Elternkonto für die Ferienbetreuung", "Eva Muster", "Winterferien 2027", "Elternkonto einrichten",
		"Der Link gilt 30 Tage.", "Sie haben schon ein Elternkonto?", "melden Sie sich einfach an", "Träger Bissendorf"} {
		assert.Contains(t, body, template.HTMLEscapeString(want))
	}
	assert.Contains(t, body, `href="https://eltern.example.org/ferienkonto/setup-token"`)
	assert.Contains(t, body, `href="https://eltern.example.org/login"`)
	assert.Equal(t, 1, strings.Count(body, `class="button"`), "one primary button")
	assert.NotContains(t, body, "—", "no em dash")
}

// TestHolidayParentAccountOfferTemplate_SignInForAnExistingAccount pins the
// variant for an address that already has a parent account (ADR 0216 point
// 4): only the sign-in button and the sentence that the registration shows
// in the Eltern-App after sign-in, no setup.
func TestHolidayParentAccountOfferTemplate_SignInForAnExistingAccount(t *testing.T) {
	t.Parallel()
	tpl, err := template.ParseGlob(filepath.Join(".", "*.html"))
	require.NoError(t, err, "parse all email templates")

	var out bytes.Buffer
	require.NoError(t, tpl.ExecuteTemplate(&out, "holiday-parent-account-offer.html", map[string]any{
		"GuardianName": "Eva Muster", "SchoolName": "Träger Bissendorf", "PeriodName": "Winterferien 2027",
		"LoginURL": "https://eltern.example.org/login",
	}))
	body := out.String()
	for _, want := range []string{"Sie haben schon ein Elternkonto.", "Nach der Anmeldung sehen Sie Ihre Ferienanmeldung", "In der Eltern-App anmelden"} {
		assert.Contains(t, body, template.HTMLEscapeString(want))
	}
	assert.Contains(t, body, `href="https://eltern.example.org/login"`)
	assert.Equal(t, 1, strings.Count(body, `class="button"`), "one button")
	assert.NotContains(t, body, "Elternkonto einrichten")
	assert.NotContains(t, body, "Der Link gilt 30 Tage.")
}
