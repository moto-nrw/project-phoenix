package email

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/logging"
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

	// Configure TLS based on port
	var clientOpts []mail.Option
	if smtp.Port == 465 {
		// Port 465: Implicit SSL/TLS (SSL from connection start)
		clientOpts = []mail.Option{
			mail.WithSSLPort(false), // Use implicit SSL
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(smtp.User),
			mail.WithPassword(smtp.Password),
		}
	} else {
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
	// Format addresses in RFC 5322 format: "Name <email@example.com>"
	fromAddr := fmt.Sprintf("%s <%s>", email.From.Name, email.From.Address)
	if err := msg.SetAddrHeader("From", fromAddr); err != nil {
		return fmt.Errorf("failed to set from address: %w", err)
	}
	toAddr := fmt.Sprintf("%s <%s>", email.To.Name, email.To.Address)
	if err := msg.SetAddrHeader("To", toAddr); err != nil {
		return fmt.Errorf("failed to set to address: %w", err)
	}
	msg.Subject(email.Subject)
	msg.SetBodyString(mail.TypeTextPlain, email.text)
	msg.AddAlternativeString(mail.TypeTextHTML, email.html)

	if logging.Logger != nil {
		logging.Logger.WithFields(map[string]interface{}{
			"to":       email.To.Address,
			"subject":  email.Subject,
			"template": email.Template,
		}).Info("Sending email")
	}
	if err := m.client.DialAndSend(msg); err != nil {
		if logging.Logger != nil {
			logging.Logger.WithFields(map[string]interface{}{
				"to":    email.To.Address,
				"error": err,
			}).Error("Email send failed")
		}
		return err
	}
	if logging.Logger != nil {
		logging.Logger.WithField("to", email.To.Address).Info("Email sent successfully")
	}

	return nil
}
