package email

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMailerRejectsMissingSMTPInDeployedEnvironments(t *testing.T) {
	t.Parallel()

	for _, appEnv := range []string{"staging", "production"} {
		appEnv := appEnv
		t.Run(appEnv, func(t *testing.T) {
			t.Parallel()
			mailer, err := NewMailer(MailerConfig{AppEnv: appEnv})
			require.ErrorContains(t, err, "EMAIL_SMTP_HOST is required")
			assert.Nil(t, mailer)
		})
	}
}

func TestNewMailerReturnsTransportInitializationErrors(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "test.html"), []byte("<p>test</p>"), 0o600))

	mailer, err := NewMailer(MailerConfig{
		Host:        "smtp.example.invalid",
		Port:        0,
		TemplateDir: templateDir,
	})
	require.Error(t, err)
	assert.Nil(t, mailer)
}
