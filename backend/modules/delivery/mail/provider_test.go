package mail

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lettermintConfig() ProviderConfig {
	return ProviderConfig{
		Provider: "lettermint", SecurityFrom: "noreply@mail.example", NoticeFrom: "nachrichten@mail.example",
		WebhookSecrets: "whsec_test", TeamToken: "lm_team_test", WebhookID: "hook-1",
		FallbackHost: "smtp.example", FallbackPort: 587, FallbackUser: "app@example.org", FallbackPassword: "test-only", FallbackFrom: "app@example.org",
	}
}

func TestWithoutProviderNothingIsRequired(t *testing.T) {
	t.Parallel()
	for _, provider := range []string{"", "  ", "smtp", "SMTP"} {
		config := ProviderConfig{Provider: provider}

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
	for key, clear := range map[string]func(*ProviderConfig){
		"EMAIL_FROM_SECURITY_ADDRESS":  func(c *ProviderConfig) { c.SecurityFrom = " " },
		"EMAIL_FROM_NOTICES_ADDRESS":   func(c *ProviderConfig) { c.NoticeFrom = "" },
		"EMAIL_WEBHOOK_SECRETS":        func(c *ProviderConfig) { c.WebhookSecrets = " , " },
		"LETTERMINT_TEAM_API_TOKEN":    func(c *ProviderConfig) { c.TeamToken = "" },
		"LETTERMINT_WEBHOOK_ID":        func(c *ProviderConfig) { c.WebhookID = "" },
		"EMAIL_FALLBACK_SMTP_HOST":     func(c *ProviderConfig) { c.FallbackHost = "" },
		"EMAIL_FALLBACK_SMTP_PORT":     func(c *ProviderConfig) { c.FallbackPort = 0 },
		"EMAIL_FALLBACK_SMTP_USER":     func(c *ProviderConfig) { c.FallbackUser = "" },
		"EMAIL_FALLBACK_SMTP_PASSWORD": func(c *ProviderConfig) { c.FallbackPassword = "" },
		"EMAIL_FALLBACK_FROM_ADDRESS":  func(c *ProviderConfig) { c.FallbackFrom = "" },
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
	err := ProviderConfig{Provider: "brevo"}.Validate()

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
