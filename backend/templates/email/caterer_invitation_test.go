package email_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The caterer's invitation (ADR 0075) carries its link in both parts of
// the mail and says how long it is valid.
func TestCatererInvitationMailRenders(t *testing.T) {
	t.Parallel()
	link := "https://caterer.example/einladung/42/abc123"
	html, text := renderDemoMail(t, "caterer-invitation.html", map[string]any{
		"InvitationURL": link, "CatererName": "Küche am Markt", "SchoolName": "OGS Beispiel",
		"ExpiryDays": 14, "LogoURL": "https://caterer.example/logo.png",
	})
	assert.Contains(t, html, `href="`+link+`">Einladung annehmen</a>`)
	for _, part := range []string{html, text} {
		assert.Contains(t, part, "Guten Tag, Küche am Markt,")
		assert.Contains(t, part, "OGS Beispiel")
		assert.Contains(t, part, "Die Einladung gilt 14 Tage.")
		assert.Contains(t, part, link, "the link must survive without the button")
		assert.NotContains(t, part, "—")
	}
}
