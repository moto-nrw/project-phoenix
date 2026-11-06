package compose

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application/lettermint"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/delivery/mail"
)

const (
	emailProviderSMTP       = "smtp"
	emailProviderLettermint = "lettermint"
)

// EmailProviderConfig is the mail provider of a deployment. An empty Provider
// keeps plain SMTP, which needs none of the other values: the switch to
// Lettermint is one human step per environment (#5935), and until then a
// deployment must start exactly as before. Once Provider names Lettermint every
// value is required, and a missing one fails the start.
type EmailProviderConfig struct {
	Provider         string
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
}

func (c EmailProviderConfig) provider() string {
	return strings.ToLower(strings.TrimSpace(c.Provider))
}

// UsesLettermint reports whether mail leaves over Lettermint.
func (c EmailProviderConfig) UsesLettermint() bool { return c.provider() == emailProviderLettermint }

// Validate names every missing key at once. It never repeats a value.
func (c EmailProviderConfig) Validate() error {
	switch c.provider() {
	case "", emailProviderSMTP:
		return nil
	case emailProviderLettermint:
	default:
		return fmt.Errorf("EMAIL_PROVIDER must be %q or %q", emailProviderSMTP, emailProviderLettermint)
	}
	missing := []string{}
	for _, value := range []struct{ key, value string }{
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
func (c EmailProviderConfig) MailSettings() (mail.LettermintSending, *mail.FallbackTransport) {
	if !c.UsesLettermint() {
		return mail.LettermintSending{}, nil
	}
	return mail.LettermintSending{SecurityFrom: strings.TrimSpace(c.SecurityFrom), NoticeFrom: strings.TrimSpace(c.NoticeFrom)},
		&mail.FallbackTransport{
			Host: strings.TrimSpace(c.FallbackHost), Port: c.FallbackPort, User: c.FallbackUser,
			Password: c.FallbackPassword, From: strings.TrimSpace(c.FallbackFrom),
		}
}

// newEmailEvents composes the webhook, the reconciliation and the lifting of a
// block. The webhook exists in every deployment and answers 503 without a secret.
func newEmailEvents(config EmailProviderConfig, store *application.WorkerCommands, logger *slog.Logger, observe ports.Observer) (*application.EmailWebhooks, *application.EmailReconciliation) {
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("component", "email_events")
	var api ports.EmailProviderAPI
	if config.UsesLettermint() {
		api = lettermint.NewClient(lettermint.ClientConfig{TeamToken: config.TeamToken, WebhookID: config.WebhookID})
	}
	fallback := config.UsesLettermint()
	webhooks := application.NewEmailWebhooks(store, []ports.EmailWebhookProvider{lettermint.NewWebhook(config.WebhookSecrets)}, fallback, logger, observe)
	return webhooks, application.NewEmailReconciliation(store, api, fallback, logger, observe)
}
