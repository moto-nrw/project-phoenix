package email

import (
	"context"
	"log/slog"

	mailcontract "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
)

const templateMFAEmailCode = "mfa-email-code.html"

// RestrictToDemoMails is the mail lock of the demo environment: under
// APP_ENV=demo every template but the two demo mails is dropped, so a visitor
// with administrator rights cannot mail arbitrary addresses from our sender.
// A dropped mail reports success, because the visitor did nothing wrong and
// a retry would change nothing; only the sign-in code reports
// ErrNotDeliveredInDemo unless the internal demo runner accepts the code.
// Everywhere else the mailer is returned as is.
func RestrictToDemoMails(mailer mailcontract.Mailer, appEnv string, logger *slog.Logger) mailcontract.Mailer {
	if mailer == nil || !mailcontract.IsDemoEnvironment(appEnv) {
		return mailer
	}
	return demoRestrictedMailer{mailer: mailer, logger: loggerOrDefault(logger), codeAddress: demoCodeAddress}
}

type demoRestrictedMailer struct {
	mailer      mailcontract.Mailer
	logger      *slog.Logger
	codeAddress string
}

func (m demoRestrictedMailer) Send(message mailcontract.Message) error {
	return m.SendContext(context.Background(), message)
}

func (m demoRestrictedMailer) SendContext(ctx context.Context, message mailcontract.Message) error {
	// The dispatcher leaves the cancellation check to a ContextMailer.
	if err := ctx.Err(); err != nil {
		return err
	}
	if message.Template != mailcontract.TemplateDemoAccess && message.Template != mailcontract.TemplateDemoLead {
		if message.Template == templateMFAEmailCode {
			return sendDemoCode(ctx, m.codeAddress, message)
		}
		m.logger.Info("email not delivered in the demo environment",
			slog.String("template", message.Template))
		return nil
	}
	if mailer, ok := m.mailer.(mailcontract.ContextMailer); ok {
		return mailer.SendContext(ctx, message)
	}
	return m.mailer.Send(message)
}
