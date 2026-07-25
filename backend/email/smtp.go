package email

import (
	"fmt"
	"log/slog"

	"github.com/spf13/viper"
	"github.com/wneessen/go-mail"
)

// SMTPMailer is a SMTP mailer.
type SMTPMailer struct {
	client      *mail.Client
	defaultFrom Email
}

// NewMailer returns a configured SMTP Mailer.
func NewMailer() (Mailer, error) {
	if err := parseTemplates(); err != nil {
		return nil, err
	}

	smtp := struct {
		Host     string
		Port     int
		User     string
		Password string
	}{
		viper.GetString("email_smtp_host"),
		viper.GetInt("email_smtp_port"),
		viper.GetString("email_smtp_user"),
		viper.GetString("email_smtp_password"),
	}

	if smtp.Host == "" {
		return NewMockMailer(), nil
	}

	defaultFrom := NewEmail(viper.GetString("email_from_name"), viper.GetString("email_from_address"))

	// Configure TLS and auth based on port and credentials
	var clientOpts []mail.Option
	switch {
	case smtp.User == "" && smtp.Password == "":
		// No credentials: plain SMTP without TLS (e.g., Mailpit on port 1025)
		clientOpts = []mail.Option{
			mail.WithPort(smtp.Port),
			mail.WithTLSPolicy(mail.NoTLS),
		}
	case smtp.Port == 465:
		// Port 465: Implicit SSL/TLS (SSL from connection start)
		clientOpts = []mail.Option{
			mail.WithSSLPort(false), // Use implicit SSL
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(smtp.User),
			mail.WithPassword(smtp.Password),
		}
	default:
		// Port 587: STARTTLS (upgrade to TLS after connect)
		clientOpts = []mail.Option{
			mail.WithPort(smtp.Port),
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(smtp.User),
			mail.WithPassword(smtp.Password),
			mail.WithTLSPolicy(mail.TLSMandatory),
		}
	}

	client, err := mail.NewClient(smtp.Host, clientOpts...)
	if err != nil {
		return nil, err
	}
	s := &SMTPMailer{
		client:      client,
		defaultFrom: defaultFrom,
	}
	return s, nil
}

// Send sends the mail via smtp.
func (m *SMTPMailer) Send(email Message) error {
	if email.From.Address == "" {
		email.From = m.defaultFrom
	}

	if err := email.parse(); err != nil {
		return err
	}

	msg := mail.NewMsg()
	// Use go-mail's typed setters so RFC 5322 special characters in the
	// display name (e.g. "@", ",", quotes) and non-ASCII names are encoded
	// correctly instead of producing a malformed header.
	if err := msg.FromFormat(email.From.Name, email.From.Address); err != nil {
		return fmt.Errorf("failed to set from address: %w", err)
	}
	if err := msg.AddToFormat(email.To.Name, email.To.Address); err != nil {
		return fmt.Errorf("failed to set to address: %w", err)
	}
	msg.Subject(email.Subject)
	msg.SetGenHeader(mail.HeaderListUnsubscribe, fmt.Sprintf("<mailto:%s?subject=unsubscribe>", email.From.Address))
	msg.SetGenHeader(mail.HeaderListUnsubscribePost, "List-Unsubscribe=One-Click")
	if email.MessageID != "" {
		msg.SetMessageIDWithValue(email.MessageID)
		// Some relays rewrite Message-ID. A custom header survives more hops
		// and is echoed back in most providers' webhook header blocks, so it
		// is a second chance at correlating a delivery event.
		msg.SetGenHeader("X-Phoenix-Outbox-Id", email.MessageID)
	}
	msg.SetBodyString(mail.TypeTextPlain, email.text)
	msg.AddAlternativeString(mail.TypeTextHTML, email.html)

	slog.Default().Info("sending email",
		slog.String("to", email.To.Address),
		slog.String("subject", email.Subject),
		slog.String("template", email.Template))
	if err := m.client.DialAndSend(msg); err != nil {
		slog.Default().Error("email send failed",
			slog.String("to", email.To.Address),
			slog.Any("error", err),
		)
		return err
	}
	slog.Default().Info("email sent successfully",
		slog.String("to", email.To.Address))

	return nil
}
