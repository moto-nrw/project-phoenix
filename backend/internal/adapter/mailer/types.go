package mailer

import "github.com/moto-nrw/project-phoenix/internal/core/port"

// Type aliases for test compatibility and adapter convenience.
type Message = port.EmailMessage
type Email = port.EmailAddress
type Mailer = port.EmailSender

// NewEmail creates a new email address with optional display name.
func NewEmail(name, address string) Email {
	return Email{
		Name:    name,
		Address: address,
	}
}

// NewMailer returns an SMTP mailer or falls back to MockMailer when SMTP config is missing.
// This preserves legacy test expectations.
func NewMailer() (Mailer, error) {
	mailer, err := NewSMTPMailer()
	if err != nil {
		return NewMockMailer(), nil
	}
	return mailer, nil
}
