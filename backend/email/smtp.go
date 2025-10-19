package email

import (
	"fmt"
	"log"

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

	client, err := mail.NewClient(smtp.Host, mail.WithPort(smtp.Port),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(smtp.User), mail.WithPassword(smtp.Password))
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
	if err := msg.SetAddrHeader("From", email.From.Address, email.From.Name); err != nil {
		return fmt.Errorf("failed to set from address: %w", err)
	}
	if err := msg.SetAddrHeader("To", email.To.Address, email.To.Name); err != nil {
		return fmt.Errorf("failed to set to address: %w", err)
	}
	msg.Subject(email.Subject)
	msg.SetBodyString(mail.TypeTextPlain, email.text)
	msg.AddAlternativeString(mail.TypeTextHTML, email.html)

	log.Printf("Sending email to=%s subject=%s template=%s", email.To.Address, email.Subject, email.Template)
	if err := m.client.DialAndSend(msg); err != nil {
		log.Printf("Email send failed to=%s error=%v", email.To.Address, err)
		return err
	}
	log.Printf("Email sent successfully to=%s", email.To.Address)

	return nil
}
