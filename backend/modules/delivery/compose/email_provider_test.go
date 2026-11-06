package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lettermintConfig() EmailProviderConfig {
	return EmailProviderConfig{
		Provider: "lettermint", SecurityFrom: "noreply@mail.example", NoticeFrom: "nachrichten@mail.example",
		WebhookSecrets: "whsec_test", TeamToken: "lm_team_test", WebhookID: "hook-1",
		FallbackHost: "smtp.example", FallbackPort: 587, FallbackUser: "app@example.org", FallbackPassword: "test-only", FallbackFrom: "app@example.org",
	}
}

func TestWithoutProviderNothingIsRequired(t *testing.T) {
	t.Parallel()
	for _, provider := range []string{"", "  ", "smtp", "SMTP"} {
		config := EmailProviderConfig{Provider: provider}

		require.NoError(t, config.Validate(), provider)
		assert.False(t, config.UsesLettermint())
		sending, fallback := config.MailSettings()
		assert.False(t, sending.Enabled(), "no route headers")
		assert.Nil(t, fallback, "no second way out")
	}
}

func TestLettermintRequiresEveryValueAndNamesTheMissingOnes(t *testing.T) {
	t.Parallel()
	require.NoError(t, lettermintConfig().Validate())
	for key, clear := range map[string]func(*EmailProviderConfig){
		"EMAIL_FROM_SECURITY_ADDRESS":  func(c *EmailProviderConfig) { c.SecurityFrom = " " },
		"EMAIL_FROM_NOTICES_ADDRESS":   func(c *EmailProviderConfig) { c.NoticeFrom = "" },
		"EMAIL_WEBHOOK_SECRETS":        func(c *EmailProviderConfig) { c.WebhookSecrets = " , " },
		"LETTERMINT_TEAM_API_TOKEN":    func(c *EmailProviderConfig) { c.TeamToken = "" },
		"LETTERMINT_WEBHOOK_ID":        func(c *EmailProviderConfig) { c.WebhookID = "" },
		"EMAIL_FALLBACK_SMTP_HOST":     func(c *EmailProviderConfig) { c.FallbackHost = "" },
		"EMAIL_FALLBACK_SMTP_PORT":     func(c *EmailProviderConfig) { c.FallbackPort = 0 },
		"EMAIL_FALLBACK_SMTP_USER":     func(c *EmailProviderConfig) { c.FallbackUser = "" },
		"EMAIL_FALLBACK_SMTP_PASSWORD": func(c *EmailProviderConfig) { c.FallbackPassword = "" },
		"EMAIL_FALLBACK_FROM_ADDRESS":  func(c *EmailProviderConfig) { c.FallbackFrom = "" },
	} {
		config := lettermintConfig()
		clear(&config)

		err := config.Validate()

		require.Error(t, err, key)
		assert.Contains(t, err.Error(), key)
		assert.NotContains(t, err.Error(), "test-only", "values never enter the message")
	}
}

func TestUnknownProviderFailsTheStart(t *testing.T) {
	t.Parallel()
	err := EmailProviderConfig{Provider: "brevo"}.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "EMAIL_PROVIDER")
}

func TestLettermintMailSettings(t *testing.T) {
	t.Parallel()
	sending, fallback := lettermintConfig().MailSettings()

	assert.True(t, sending.Enabled())
	assert.Equal(t, "noreply@mail.example", sending.SecurityFrom)
	assert.Equal(t, "nachrichten@mail.example", sending.NoticeFrom)
	require.NotNil(t, fallback)
	assert.Equal(t, "app@example.org", fallback.From)
}
