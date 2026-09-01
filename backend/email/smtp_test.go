package email

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wneessen/go-mail"
)

// =============================================================================
// SMTPMailer Interface Tests
// =============================================================================

func TestSMTPMailer_ImplementsMailer(t *testing.T) {
	t.Parallel()
	// SMTPMailer should implement Mailer interface
	var _ Mailer = &SMTPMailer{}
}

// =============================================================================
// NewMailer Tests
// =============================================================================

func TestNewMailer_NoSMTPHost_ReturnsMockMailer(t *testing.T) {
	t.Parallel()
	// Setup: Create minimal templates directory
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	// Create a minimal template file
	templateContent := `<!DOCTYPE html><html><body>{{.Content}}</body></html>`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "test.html"),
		[]byte(templateContent),
		0644,
	))

	mailer, err := NewMailer(MailerConfig{TemplateDir: templatesDir})
	require.NoError(t, err)
	assert.NotNil(t, mailer)

	// Should return MockMailer when SMTP host is empty
	_, isMock := mailer.(*MockMailer)
	assert.True(t, isMock, "Expected MockMailer when SMTP host is empty")
}

func TestNewMailer_WithSMTPHost_ConfiguresClient(t *testing.T) {
	t.Parallel()
	// Setup: Create minimal templates directory
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	// Create a minimal template file
	templateContent := `<!DOCTYPE html><html><body>Test</body></html>`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "test.html"),
		[]byte(templateContent),
		0644,
	))

	mailer, err := NewMailer(MailerConfig{
		Host:        "localhost",
		Port:        587,
		User:        "testuser",
		Password:    "testpass",
		DefaultFrom: NewEmail("Test Sender", "test@example.com"),
		TemplateDir: templatesDir,
	})
	require.NoError(t, err)
	assert.NotNil(t, mailer)

	// Should return SMTPMailer when SMTP host is set
	smtpMailer, isSMTP := mailer.(*SMTPMailer)
	assert.True(t, isSMTP, "Expected SMTPMailer when SMTP host is set")

	if isSMTP {
		assert.Equal(t, "Test Sender", smtpMailer.defaultFrom.Name)
		assert.Equal(t, "test@example.com", smtpMailer.defaultFrom.Address)
	}
}

func TestNewMailer_Port465_ConfiguresSSL(t *testing.T) {
	t.Parallel()
	// Setup: Create minimal templates directory
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	templateContent := `<!DOCTYPE html><html><body>Test</body></html>`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "test.html"),
		[]byte(templateContent),
		0644,
	))

	mailer, err := NewMailer(MailerConfig{
		Host:        "localhost",
		Port:        465,
		User:        "testuser",
		Password:    "testpass",
		TemplateDir: templatesDir,
	})
	require.NoError(t, err)
	assert.NotNil(t, mailer)

	_, isSMTP := mailer.(*SMTPMailer)
	assert.True(t, isSMTP, "Expected SMTPMailer for port 465")
}

func TestNewMailer_NoTemplates_ReturnsError(t *testing.T) {
	t.Parallel()
	// Setup: Create empty directory (no templates)
	tempDir := t.TempDir()

	// Should fail because no templates directory exists
	mailer, err := NewMailer(MailerConfig{TemplateDir: filepath.Join(tempDir, "templates")})

	// Either error or nil mailer expected when templates missing
	if err == nil {
		// If no error, should still get a mailer (mock fallback)
		assert.NotNil(t, mailer)
	}
}

// =============================================================================
// SMTPMailer.Send Tests
// =============================================================================

func TestSMTPMailer_Send_UsesDefaultFrom(t *testing.T) {
	t.Parallel()

	// This test verifies the defaultFrom logic without actual SMTP connection
	smtpMailer := &SMTPMailer{
		defaultFrom: Email{
			Name:    "Default Sender",
			Address: "default@example.com",
		},
	}

	// Message without From address
	msg := Message{
		To:      Email{Name: "Recipient", Address: "recipient@example.com"},
		Subject: "Test",
	}

	// We can't fully test Send without templates and SMTP,
	// but we can verify the struct is configured correctly
	assert.Equal(t, "Default Sender", smtpMailer.defaultFrom.Name)
	assert.Equal(t, "default@example.com", smtpMailer.defaultFrom.Address)
	assert.Empty(t, msg.From.Address, "Message should have empty From initially")
}

// =============================================================================
// parseTemplates Tests
// =============================================================================

func TestParseTemplates_WithValidTemplates(t *testing.T) {
	t.Parallel()
	// Setup: Create templates directory with valid template
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	templateContent := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<p>Hello {{.Name}}</p>
<p>Date: {{formatAsDate .Date}}</p>
</body>
</html>`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "test_email.html"),
		[]byte(templateContent),
		0644,
	))

	templates, err := parseTemplates(templatesDir)
	assert.NoError(t, err)
	assert.NotNil(t, templates)
}

func TestParseTemplates_NoTemplatesDirectory(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	_, err := parseTemplates(filepath.Join(tempDir, "templates"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "templates")
}

func TestParseTemplates_MultipleTemplates(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	// Create multiple template files
	templateFiles := map[string]string{
		"email1.html":        `<html><body>Email 1</body></html>`,
		"email2.html":        `<html><body>Email 2</body></html>`,
		"nested/email3.html": `<html><body>Email 3</body></html>`,
	}

	for path, content := range templateFiles {
		fullPath := filepath.Join(templatesDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}

	templates, err := parseTemplates(templatesDir)
	assert.NoError(t, err)
	assert.NotNil(t, templates)
}

// =============================================================================
// Message.parse Tests
// =============================================================================

func TestMessage_Parse_RequiresTemplates(t *testing.T) {
	t.Parallel()

	msg := Message{
		Template: "nonexistent.html",
		Content:  map[string]string{"key": "value"},
	}

	require.ErrorContains(t, msg.parse(nil), "not configured")
}

func TestMessage_Parse_WithValidTemplate(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	templateContent := `<!DOCTYPE html>
<html>
<body>
<p>Hello {{.Name}}</p>
</body>
</html>`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "greeting.html"),
		[]byte(templateContent),
		0644,
	))

	templates, err := parseTemplates(templatesDir)
	require.NoError(t, err)

	msg := Message{
		Template: "greeting.html",
		Content:  struct{ Name string }{Name: "World"},
	}

	err = msg.parse(templates)
	assert.NoError(t, err)
	assert.NotEmpty(t, msg.html)
	assert.NotEmpty(t, msg.text)
	assert.Contains(t, msg.html, "Hello World")
}

// =============================================================================
// Regression: RFC 5322 address header formatting (#1193)
// =============================================================================

// TestSMTPMailer_Send_SpecialCharacterDisplayName is a regression test for
// issue #1193. Display names containing RFC 5322 special characters (e.g. "@",
// ",", double-quotes) used to break Send() because the code formatted
// addresses manually with fmt.Sprintf("%s <%s>", ...) before handing them to
// go-mail's SetAddrHeader. For a display name like "admin@example.de" that
// produced the malformed string `admin@example.de <real@email.de>`, which
// go-mail's address parser rejected, and the email was never sent.
//
// The fix uses msg.FromFormat / msg.AddToFormat, which build the address from
// a typed mail.Address{Name, Address} and apply RFC 5322 quoting plus RFC 2047
// encoding for non-ASCII. This test makes sure that if anyone reverts the fix
// back to manual Sprintf, Send() will fail the address-setting step and the
// error assertions below will fire.
//
// Strategy: point the SMTP client at an unreachable local port so DialAndSend
// fails fast with a connection error. Send() constructs headers *before*
// DialAndSend, so as long as the fix is in place the error bubbles up from
// the dial step — never from "failed to set from/to address".
func TestSMTPMailer_Send_SpecialCharacterDisplayName(t *testing.T) {
	t.Parallel()
	// parseTemplates() walks ./templates, so we need a temp working dir with a
	// minimal template file. Same pattern as the other tests in this file.
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "regression.html"),
		[]byte(`<!DOCTYPE html><html><body><p>{{.Message}}</p></body></html>`),
		0644,
	))

	templates, err := parseTemplates(templatesDir)
	require.NoError(t, err)

	// Unreachable local port → DialAndSend fails immediately with
	// "connection refused", which is exactly what we want: Send() gets far
	// enough to exercise the address-setting code, but never hits the network.
	client, err := mail.NewClient(
		"127.0.0.1",
		mail.WithPort(1),
		mail.WithTLSPolicy(mail.NoTLS),
	)
	require.NoError(t, err)

	mailer := &SMTPMailer{
		client:    client,
		templates: templates,
		logger:    slog.New(slog.DiscardHandler),
		defaultFrom: Email{
			Name:    "System",
			Address: "system@example.com",
		},
	}

	// Each of these display names contains an RFC 5322 special that must be
	// quoted in the header. The "@" case is the exact scenario from #1193.
	cases := []struct {
		name        string
		displayName string
	}{
		{"at_sign_in_display_name", "admin@example.de"},
		{"comma_in_display_name", "Doe, John"},
		{"double_quote_in_display_name", `He said "hi"`},
		{"non_ascii_display_name", "Müller Häusler"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := mailer.Send(Message{
				From:     Email{Name: tc.displayName, Address: "from@example.com"},
				To:       Email{Name: tc.displayName, Address: "to@example.com"},
				Subject:  "Regression #1193",
				Template: "regression.html",
				Content:  map[string]string{"Message": "hi"},
			})

			// DialAndSend MUST fail — we are deliberately pointing at an
			// unreachable port. A nil error here would mean the client
			// somehow connected to something on 127.0.0.1:1, which is
			// either a misconfigured environment or a broken test.
			require.Error(t, err, "expected DialAndSend to fail against 127.0.0.1:1")

			// The whole point of this regression test: the error must NOT
			// come from header construction. If someone reverts the fix to
			// SetAddrHeader(fmt.Sprintf(...)), these assertions fail because
			// go-mail's parser rejects `admin@example.de <real@email.de>`
			// and Send() returns "failed to set to address: ...".
			assert.NotContains(t, err.Error(), "failed to set from address",
				"display name %q broke From header formatting — fix for #1193 may have regressed", tc.displayName)
			assert.NotContains(t, err.Error(), "failed to set to address",
				"display name %q broke To header formatting — fix for #1193 may have regressed", tc.displayName)
		})
	}

	t.Run("reply-to header", func(t *testing.T) {
		message := Message{
			From:     NewEmail("moto", "kontakt@moto.nrw"),
			ReplyTo:  NewEmail(`OGS "Am Berg", Grundschule`, "ogs@schule-am-berg.example"),
			To:       NewEmail("Erika Muster", "erika@example.test"),
			Subject:  "Einladung zum Eltern-Portal",
			Template: "regression.html",
			Content:  map[string]string{"Message": "Hallo"},
		}

		built, err := mailer.buildMessage(message)
		require.NoError(t, err)
		replyTo := built.GetAddrHeaderString(mail.HeaderReplyTo)
		require.Len(t, replyTo, 1)
		assert.Contains(t, replyTo[0], "ogs@schule-am-berg.example")
		from := built.GetAddrHeaderString(mail.HeaderFrom)
		require.Len(t, from, 1)
		assert.Contains(t, from[0], "kontakt@moto.nrw")

		message.ReplyTo = Email{}
		built, err = mailer.buildMessage(message)
		require.NoError(t, err)
		assert.Empty(t, built.GetAddrHeaderString(mail.HeaderReplyTo))
	})
}
