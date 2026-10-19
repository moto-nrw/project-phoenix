package email

import (
	"github.com/moto-nrw/project-phoenix/modules/delivery/mail"
)

func IsLoggingMailTransport(mailer mail.Mailer) bool { _, ok := mailer.(*MockMailer); return ok }
