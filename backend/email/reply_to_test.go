package email

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wneessen/go-mail"
)

// withTestTemplate parses a throwaway template set so buildMessage can render.
// The package parses templates from ./templates relative to the working
// directory, so the test moves there and back.
func withTestTemplate(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "reply-to-test.html"),
		[]byte(`<!DOCTYPE html><html><body>Hallo</body></html>`),
		0644,
	))

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	require.NoError(t, os.Chdir(tempDir))
	require.NoError(t, parseTemplates())
}

func replyToTestMessage() Message {
	return Message{
		From:     NewEmail("moto", "kontakt@moto.nrw"),
		To:       NewEmail("Erika Muster", "erika@example.test"),
		Subject:  "Einladung zum Eltern-Portal",
		Template: "reply-to-test.html",
		Content:  map[string]any{},
	}
}

// A tenant-bound mail keeps the central authenticated From and moves ONLY the
// return path. Both halves matter: a changed From would break SPF/DKIM
// alignment, an unchanged Reply-To sends the parent's answer to moto (#1936).
func TestBuildMessage_ReplyTo_SetWithoutTouchingFrom(t *testing.T) {
	withTestTemplate(t)

	m := &SMTPMailer{}
	msg := replyToTestMessage()
	msg.ReplyTo = NewEmail("OGS Schule am Berg", "ogs@schule-am-berg.example")

	built, err := m.buildMessage(msg)
	require.NoError(t, err)

	replyTo := built.GetAddrHeaderString(mail.HeaderReplyTo)
	require.Len(t, replyTo, 1)
	assert.Contains(t, replyTo[0], "ogs@schule-am-berg.example")

	from := built.GetAddrHeaderString(mail.HeaderFrom)
	require.Len(t, from, 1)
	assert.Contains(t, from[0], "kontakt@moto.nrw",
		"From must stay the central authenticated sender")
}

// No configured reply address must produce NO header at all, so global system
// mail (Passwort-Reset, MFA) and unconfigured schools behave exactly as before.
func TestBuildMessage_NoReplyTo_OmitsHeader(t *testing.T) {
	withTestTemplate(t)

	m := &SMTPMailer{}
	built, err := m.buildMessage(replyToTestMessage())
	require.NoError(t, err)

	assert.Empty(t, built.GetAddrHeaderString(mail.HeaderReplyTo))
}

// A display name carrying RFC 5322 specials must be encoded, not concatenated
// into a malformed header — the same reason From/To use the typed setters.
func TestBuildMessage_ReplyTo_EncodesSpecialCharactersInName(t *testing.T) {
	withTestTemplate(t)

	m := &SMTPMailer{}
	msg := replyToTestMessage()
	msg.ReplyTo = NewEmail(`OGS "Am Berg", Grundschule`, "ogs@schule-am-berg.example")

	built, err := m.buildMessage(msg)
	require.NoError(t, err)

	replyTo := built.GetAddrHeaderString(mail.HeaderReplyTo)
	require.Len(t, replyTo, 1)
	assert.Contains(t, replyTo[0], "ogs@schule-am-berg.example")
}
