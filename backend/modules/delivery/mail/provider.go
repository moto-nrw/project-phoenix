package mail

import (
	"fmt"
	"log/slog"
	"strings"
)

const (
	emailProviderSMTP       = "smtp"
	emailProviderLettermint = "lettermint"
)

// ProviderConfig is the mail provider of a deployment. An empty Provider
// keeps plain SMTP, which needs none of the other values: the switch to
// Lettermint is one human step per environment (#5935), and until then a
// deployment must start exactly as before. Once Provider names Lettermint every
// value is required, and a missing one fails the start: without a host the mail
// would vanish in the logging transport, and without credentials it would leave
// without TLS. With them the transport insists on TLS.
type ProviderConfig struct {
	Provider         string
	SMTPHost         string
	SMTPUser         string
	SMTPPassword     string
	SecurityFrom     string
	NoticeFrom       string
	WebhookSecrets   string
	TeamToken        string
	WebhookID        string
	FallbackHost     string
	FallbackPort     int
	FallbackUser     string
	FallbackPassword string
	FallbackFrom     string
	Logger           *slog.Logger
}

func (c ProviderConfig) provider() string {
	return strings.ToLower(strings.TrimSpace(c.Provider))
}

// UsesLettermint reports whether mail leaves over Lettermint.
func (c ProviderConfig) UsesLettermint() bool { return c.provider() == emailProviderLettermint }

// Validate names every missing key at once. It never repeats a value.
func (c ProviderConfig) Validate() error {
	switch c.provider() {
	case "", emailProviderSMTP:
		return nil
	case emailProviderLettermint:
	default:
		return fmt.Errorf("EMAIL_PROVIDER must be %q or %q", emailProviderSMTP, emailProviderLettermint)
	}
	missing := []string{}
	for _, value := range []struct{ key, value string }{
		{"EMAIL_SMTP_HOST", c.SMTPHost},
		{"EMAIL_SMTP_USER", c.SMTPUser},
		{"EMAIL_SMTP_PASSWORD", c.SMTPPassword},
		{"EMAIL_FROM_SECURITY_ADDRESS", c.SecurityFrom},
		{"EMAIL_FROM_NOTICES_ADDRESS", c.NoticeFrom},
		{"EMAIL_WEBHOOK_SECRETS", strings.Trim(c.WebhookSecrets, ", ")},
		{"LETTERMINT_TEAM_API_TOKEN", c.TeamToken},
		{"LETTERMINT_WEBHOOK_ID", c.WebhookID},
		{"EMAIL_FALLBACK_SMTP_HOST", c.FallbackHost},
		{"EMAIL_FALLBACK_SMTP_USER", c.FallbackUser},
		{"EMAIL_FALLBACK_SMTP_PASSWORD", c.FallbackPassword},
		{"EMAIL_FALLBACK_FROM_ADDRESS", c.FallbackFrom},
	} {
		if strings.TrimSpace(value.value) == "" {
			missing = append(missing, value.key)
		}
	}
	if c.FallbackPort <= 0 {
		missing = append(missing, "EMAIL_FALLBACK_SMTP_PORT")
	}
	if len(missing) > 0 {
		return fmt.Errorf("EMAIL_PROVIDER=%s requires %s", emailProviderLettermint, strings.Join(missing, ", "))
	}
	return nil
}

// MailSettings are the transport's settings. Plain SMTP gets none, so the
// transport adds no header and no second way out.
func (c ProviderConfig) MailSettings() (LettermintSending, *FallbackTransport) {
	if !c.UsesLettermint() {
		return LettermintSending{}, nil
	}
	return LettermintSending{SecurityFrom: strings.TrimSpace(c.SecurityFrom), NoticeFrom: strings.TrimSpace(c.NoticeFrom)},
		&FallbackTransport{
			Host: strings.TrimSpace(c.FallbackHost), Port: c.FallbackPort, User: c.FallbackUser,
			Password: c.FallbackPassword, From: strings.TrimSpace(c.FallbackFrom),
		}
}
