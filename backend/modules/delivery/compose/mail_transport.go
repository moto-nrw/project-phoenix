package compose

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/email"
	"github.com/moto-nrw/project-phoenix/modules/delivery/mail"
)

func NewMailTransport(config mail.MailerConfig) (mail.Mailer, error) { return email.NewMailer(config) }
func NewMailDispatcher(mailer mail.Mailer, logger *slog.Logger, observers ...mail.DeliveryObserver) mail.Dispatcher {
	return email.NewDispatcher(mailer, logger, observers...)
}
func IsLoggingMailTransport(mailer mail.Mailer) bool { _, ok := mailer.(*email.MockMailer); return ok }
