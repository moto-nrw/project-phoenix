// Package mailer provides email sending adapters.
package mailer

import (
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
)

// SMTPAdapter wraps the legacy email.Mailer to implement port.EmailSender.
// This adapter bridges the existing email infrastructure with the Hexagonal Architecture
// port interface, allowing gradual migration without breaking changes.
type SMTPAdapter struct {
	mailer email.Mailer
}

// NewSMTPAdapter creates a new adapter wrapping an email.Mailer implementation.
func NewSMTPAdapter(mailer email.Mailer) *SMTPAdapter {
	return &SMTPAdapter{mailer: mailer}
}

// Send implements port.EmailSender by converting the port message type
// to the legacy email.Message type.
func (a *SMTPAdapter) Send(msg port.EmailMessage) error {
	legacyMsg := email.Message{
		From:     email.NewEmail(msg.From.Name, msg.From.Address),
		To:       email.NewEmail(msg.To.Name, msg.To.Address),
		Subject:  msg.Subject,
		Template: msg.Template,
		Content:  msg.Content,
	}

	// If pre-rendered content is provided, the legacy mailer doesn't support it directly.
	// For now, we use templates. Pre-rendered support would require extending email.Message.
	// This is documented for future migration when the email package is fully replaced.

	return a.mailer.Send(legacyMsg)
}

// Ensure SMTPAdapter implements port.EmailSender
var _ port.EmailSender = (*SMTPAdapter)(nil)
