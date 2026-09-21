package email

import (
	"context"
	"log/slog"
	"strings"
)

// The two mails of the public demo (#3465). Only they leave the demo
// environment.
const (
	TemplateDemoAccess = "demo-access.html"
	TemplateDemoLead   = "demo-lead.html"
)

// RestrictToDemoMails is the mail lock of the demo environment: under
// APP_ENV=demo every template but the two demo mails is dropped, so a visitor
// with administrator rights cannot mail arbitrary addresses from our sender.
// A dropped mail reports success, because the visitor did nothing wrong and
// a retry would change nothing. Everywhere else the mailer is returned as is.
func RestrictToDemoMails(mailer Mailer, appEnv string, logger *slog.Logger) Mailer {
	if mailer == nil || !strings.EqualFold(strings.TrimSpace(appEnv), "demo") {
		return mailer
	}
	return demoRestrictedMailer{mailer: mailer, logger: loggerOrDefault(logger)}
}

type demoRestrictedMailer struct {
	mailer Mailer
	logger *slog.Logger
}

func (m demoRestrictedMailer) Send(message Message) error {
	return m.SendContext(context.Background(), message)
}

func (m demoRestrictedMailer) SendContext(ctx context.Context, message Message) error {
	if message.Template != TemplateDemoAccess && message.Template != TemplateDemoLead {
		m.logger.Info("email not delivered in the demo environment",
			slog.String("template", message.Template))
		return nil
	}
	if mailer, ok := m.mailer.(ContextMailer); ok {
		return mailer.SendContext(ctx, message)
	}
	return m.mailer.Send(message)
}
