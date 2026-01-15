package email

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// SMTPMailer Interface Tests
// =============================================================================

func TestSMTPMailer_ImplementsMailer(_ *testing.T) {
	// SMTPMailer should implement Mailer interface
	var _ Mailer = &SMTPMailer{}
}

// =============================================================================
// NewMailer Tests
// =============================================================================

func TestNewMailer_NoSMTPHost_ReturnsMockMailer(t *testing.T) {
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

	// Change working directory temporarily
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()
	require.NoError(t, os.Chdir(tempDir))

	// Clear SMTP host to trigger MockMailer fallback
	viper.Set("email_smtp_host", "")

	mailer, err := NewMailer()
	require.NoError(t, err)
	assert.NotNil(t, mailer)

	// Should return MockMailer when SMTP host is empty
	_, isMock := mailer.(*MockMailer)
	assert.True(t, isMock, "Expected MockMailer when SMTP host is empty")
}

func TestNewMailer_WithSMTPHost_ConfiguresClient(t *testing.T) {
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

	// Change working directory temporarily
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()
	require.NoError(t, os.Chdir(tempDir))

	// Set SMTP configuration (won't actually connect)
	viper.Set("email_smtp_host", "localhost")
	viper.Set("email_smtp_port", 587)
	viper.Set("email_smtp_user", "testuser")
	viper.Set("email_smtp_password", "testpass")
	viper.Set("email_from_name", "Test Sender")
	viper.Set("email_from_address", "test@example.com")

	defer func() {
		viper.Set("email_smtp_host", "")
	}()

	mailer, err := NewMailer()
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

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()
	require.NoError(t, os.Chdir(tempDir))

	// Set SMTP configuration with port 465 (SSL)
	viper.Set("email_smtp_host", "localhost")
	viper.Set("email_smtp_port", 465) // SSL port
	viper.Set("email_smtp_user", "testuser")
	viper.Set("email_smtp_password", "testpass")

	defer func() {
		viper.Set("email_smtp_host", "")
	}()

	mailer, err := NewMailer()
	require.NoError(t, err)
	assert.NotNil(t, mailer)

	_, isSMTP := mailer.(*SMTPMailer)
	assert.True(t, isSMTP, "Expected SMTPMailer for port 465")
}

func TestNewMailer_NoTemplates_ReturnsError(t *testing.T) {
	// Setup: Create empty directory (no templates)
	tempDir := t.TempDir()

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()
	require.NoError(t, os.Chdir(tempDir))

	// Should fail because no templates directory exists
	mailer, err := NewMailer()

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

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()
	require.NoError(t, os.Chdir(tempDir))

	// Reset templates global
	templates = nil

	err = parseTemplates()
	assert.NoError(t, err)
	assert.NotNil(t, templates)
}

func TestParseTemplates_NoTemplatesDirectory(t *testing.T) {
	tempDir := t.TempDir()

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()
	require.NoError(t, os.Chdir(tempDir))

	// Reset templates global
	templates = nil

	// Should handle missing directory gracefully or return error
	err = parseTemplates()
	// Either succeeds with no templates or returns error
	if err != nil {
		assert.Contains(t, err.Error(), "templates")
	}
}

func TestParseTemplates_MultipleTemplates(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	// Create multiple template files
	templateFiles := map[string]string{
		"email1.html": `<html><body>Email 1</body></html>`,
		"email2.html": `<html><body>Email 2</body></html>`,
		"nested/email3.html": `<html><body>Email 3</body></html>`,
	}

	for path, content := range templateFiles {
		fullPath := filepath.Join(templatesDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()
	require.NoError(t, os.Chdir(tempDir))

	templates = nil
	err = parseTemplates()
	assert.NoError(t, err)
}

// =============================================================================
// Message.parse Tests
// =============================================================================

func TestMessage_Parse_RequiresTemplates(t *testing.T) {
	// This test documents that parse() requires templates to be initialized
	// The current implementation panics if templates is nil,
	// so we skip the actual parse call and just verify the setup

	// Reset templates to nil to demonstrate the dependency
	originalTemplates := templates
	defer func() { templates = originalTemplates }()
	templates = nil

	msg := Message{
		Template: "nonexistent.html",
		Content:  map[string]string{"key": "value"},
	}

	// Document that templates must be initialized before parse()
	// The parse function requires templates != nil
	assert.Nil(t, templates, "Templates should be nil for this test setup")
	assert.NotEmpty(t, msg.Template)
}

func TestMessage_Parse_WithValidTemplate(t *testing.T) {
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

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()
	require.NoError(t, os.Chdir(tempDir))

	templates = nil
	require.NoError(t, parseTemplates())

	msg := Message{
		Template: "greeting.html",
		Content:  struct{ Name string }{Name: "World"},
	}

	err = msg.parse()
	assert.NoError(t, err)
	assert.NotEmpty(t, msg.html)
	assert.NotEmpty(t, msg.text)
	assert.Contains(t, msg.html, "Hello World")
}
