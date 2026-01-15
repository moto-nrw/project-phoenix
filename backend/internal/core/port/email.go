// Package port defines interfaces (contracts) for adapters.
// These interfaces are defined by what the domain/service layer needs,
// following the Dependency Inversion Principle.
package port

// EmailSender defines the contract for sending emails.
// Implementations can be SMTP, SendGrid, Mailgun, etc.
// This follows the Hexagonal Architecture pattern where the domain
// defines what it needs, and adapters implement it.
type EmailSender interface {
	// Send delivers an email message.
	// Returns an error if the message cannot be sent.
	Send(msg EmailMessage) error
}

// EmailMessage represents an email to be sent.
// This is a domain concept, independent of any specific email provider.
type EmailMessage struct {
	// From is the sender email address
	From EmailAddress

	// To is the recipient email address
	To EmailAddress

	// Subject is the email subject line
	Subject string

	// Template is the name of the template to use (e.g., "invitation.html")
	Template string

	// Content is the data to render in the template
	Content any

	// PrerenderedHTML is optional pre-rendered HTML content
	// If set, Template and Content are ignored
	PrerenderedHTML string

	// PrerenderedText is optional pre-rendered plain text content
	PrerenderedText string
}

// EmailAddress represents an email address with optional display name.
type EmailAddress struct {
	Name    string
	Address string
}
