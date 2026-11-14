package identityaccess

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Every language of the parents portal carries both texts, fills in its
// numbers and falls back to German for a locale the portal does not speak
// (#7051).
func TestParentSecurityMailsSpeakEveryPortalLanguage(t *testing.T) {
	t.Parallel()
	for lang := range parentCopies {
		code := MFACodeMailContent(MFACodeMail{Parent: true, Locale: lang, Code: "482157", ExpiryMinutes: 10, TrustedDeviceEnabled: true, TrustedDeviceDays: 90}, "https://eltern.example")
		device := TrustedDeviceAddedMailContent(TrustedDeviceMail{Parent: true, Locale: strings.ToUpper(lang), TrustedDays: 90}, "https://eltern.example")
		assert.Equal(t, "parent-mfa-email-code.html", code.Template, lang)
		assert.Equal(t, "parent-trusted-device-added.html", device.Template, lang)
		for name, mail := range map[string]Mail{"code": code, "device": device} {
			content := mail.Content.(map[string]any)
			assert.Equal(t, lang, content["Lang"], "%s %s", lang, name)
			rendered := fmt.Sprint(content)
			assert.NotContains(t, rendered, "%!", "%s %s fills in every number", lang, name)
			assert.Contains(t, rendered, "90", "%s %s names the days", lang, name)
		}
		assert.Contains(t, code.Content.(map[string]any)["Validity"], "10", lang)
		reset := MFAResetByAdminMailContent(MFAResetByAdminMail{Locale: lang, ResetAt: "14.11.2026 13:40", ByOperator: true}, "")
		assert.Equal(t, "parent-mfa-reset.html", reset.Template, lang)
		assert.Contains(t, reset.Content.(map[string]any)["Body"], "14.11.2026 13:40", lang)
	}
	german := MFACodeMailContent(MFACodeMail{Parent: true, Locale: "fr", ExpiryMinutes: 10}, "")
	assert.Equal(t, "de", german.Content.(map[string]any)["Lang"])
	assert.Empty(t, german.Content.(map[string]any)["Tip"], "no remembered device, no tip")
	staff := MFACodeMailContent(MFACodeMail{Locale: "en"}, "")
	assert.Equal(t, "mfa-email-code.html", staff.Template, "codes of the other portals stay German")
	assert.Equal(t, "account-mfa-reset.html", MFAResetByAdminMailContent(MFAResetByAdminMail{}, "").Template,
		"an account without a parents-portal language keeps the staff notice")
}
