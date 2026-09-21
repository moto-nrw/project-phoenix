package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/k3a/html2text"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderDemoMail(t *testing.T, name string, data map[string]any) (html, text string) {
	t.Helper()
	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err, "parse all email templates")
	var buf bytes.Buffer
	require.NoError(t, tpl.ExecuteTemplate(&buf, name, data))
	// The mailer derives the text part from the HTML the same way.
	return buf.String(), html2text.HTML2Text(buf.String())
}

// The mail that brings a prospect back into the demo (#3465), in both parts.
func TestDemoAccessMailRenders(t *testing.T) {
	t.Parallel()
	entryURL := "https://messe-demo.demo.example/demo#token=abc123"
	html, text := renderDemoMail(t, "demo-access.html", map[string]any{
		"PersonName": "Kim Beispiel", "EntryURL": entryURL, "LogoURL": "https://demo.example/logo.png",
	})
	assert.Contains(t, html, `href="`+entryURL+`">Demo öffnen</a>`)
	// The text part has no button: the address stands alone on its line.
	assert.Contains(t, text, entryURL+"\r\n")
	assert.NotContains(t, text, entryURL+" ")
	for _, part := range []string{html, text} {
		assert.Contains(t, part, "Guten Tag,")
		assert.NotContains(t, part, "Kim Beispiel", "nothing the public form carried reaches the mail")
		assert.Contains(t, part, "14 Tage gültig")
		assert.Contains(t, part, "Fragen? Antworten Sie einfach auf diese Mail.")
		assert.Contains(t, part, entryURL, "the link must survive without the button")
	}
}

func TestDemoLeadMailRenders(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"PersonName": "Kim Beispiel", "OGSName": "OGS Beispiel", "Email": "kim@ogs-beispiel.de",
		"Source": "messe", "ContactOptIn": true, "AccessID": int64(4711), "LogoURL": "https://demo.example/logo.png",
	}
	html, text := renderDemoMail(t, "demo-lead.html", data)
	// The shared footer signs for a school when the content names one; the
	// prospect's OGS sent nothing.
	assert.NotContains(t, html, "im Auftrag von")
	for _, part := range []string{html, text} {
		for _, snippet := range []string{
			"Name: Kim Beispiel", "OGS: OGS Beispiel", "E-Mail: kim@ogs-beispiel.de",
			"Quelle: messe", "Kontakt erlaubt: ja", "Kennung des Demo-Zugangs: 4711",
		} {
			assert.Contains(t, part, snippet)
		}
	}

	data["Source"], data["ContactOptIn"] = "", false
	_, text = renderDemoMail(t, "demo-lead.html", data)
	assert.Contains(t, text, "Quelle: keine Angabe")
	assert.Contains(t, text, "Kontakt erlaubt: nein")
}
