package email

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

// The two mails of the public demo (#3465). Only they leave the demo
// environment.
const (
	TemplateDemoAccess = "demo-access.html"
	TemplateDemoLead   = "demo-lead.html"
)

// templateMFAEmailCode is the one dropped mail a flow waits for: a sign-in
// that asks for the code cannot finish without it.
const templateMFAEmailCode = "mfa-email-code.html"

// ErrNotDeliveredInDemo reports a mail the demo environment keeps although
// its flow depends on it, so that flow fails instead of waiting forever.
var ErrNotDeliveredInDemo = errors.New("email is not delivered in the demo environment")

// IsDemoEnvironment reports whether appEnv names the public demo. The demo
// routes, the demo access and the mail lock all decide with it, so none of
// them exists without the others.
func IsDemoEnvironment(appEnv string) bool {
	return strings.EqualFold(strings.TrimSpace(appEnv), "demo")
}

// RestrictToDemoMails is the mail lock of the demo environment: under
// APP_ENV=demo every template but the two demo mails is dropped, so a visitor
// with administrator rights cannot mail arbitrary addresses from our sender.
// A dropped mail reports success, because the visitor did nothing wrong and
// a retry would change nothing; only the sign-in code reports
// ErrNotDeliveredInDemo. Everywhere else the mailer is returned as is.
func RestrictToDemoMails(mailer Mailer, appEnv string, logger *slog.Logger) Mailer {
	if mailer == nil || !IsDemoEnvironment(appEnv) {
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
	// The dispatcher leaves the cancellation check to a ContextMailer.
	if err := ctx.Err(); err != nil {
		return err
	}
	if message.Template != TemplateDemoAccess && message.Template != TemplateDemoLead {
		m.logger.Info("email not delivered in the demo environment",
			slog.String("template", message.Template))
		if message.Template == templateMFAEmailCode {
			return ErrNotDeliveredInDemo
		}
		return nil
	}
	if mailer, ok := m.mailer.(ContextMailer); ok {
		return mailer.SendContext(ctx, message)
	}
	return m.mailer.Send(message)
}
