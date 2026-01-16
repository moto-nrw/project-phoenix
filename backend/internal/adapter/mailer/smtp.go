// Package mailer provides email sending implementations.
// This is an adapter that implements the port.EmailSender interface.
package mailer

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/spf13/viper"
	"github.com/vanng822/go-premailer/premailer"
	"github.com/wneessen/go-mail"
)

var templates *template.Template

// SMTPMailer is an SMTP mailer that implements port.EmailSender.
type SMTPMailer struct {
	client      *mail.Client
	defaultFrom port.EmailAddress
}

// Ensure SMTPMailer implements port.EmailSender
var _ port.EmailSender = (*SMTPMailer)(nil)

// NewSMTPMailer returns a configured SMTP Mailer.
// If EMAIL_MOCK is enabled, returns a MockMailer instead.
func NewSMTPMailer() (port.EmailSender, error) {
	if err := parseTemplates(); err != nil {
		return nil, err
	}

	if viper.GetBool("email_mock") {
		return NewMockMailer(), nil
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
		return nil, fmt.Errorf("EMAIL_SMTP_HOST is required (set EMAIL_MOCK=true to use mock mailer)")
	}
	if smtp.Port == 0 {
		return nil, fmt.Errorf("EMAIL_SMTP_PORT is required")
	}
	if smtp.User == "" {
		return nil, fmt.Errorf("EMAIL_SMTP_USER is required")
	}
	if smtp.Password == "" {
		return nil, fmt.Errorf("EMAIL_SMTP_PASSWORD is required")
	}

	defaultFrom := port.EmailAddress{
		Name:    strings.TrimSpace(viper.GetString("email_from_name")),
		Address: strings.TrimSpace(viper.GetString("email_from_address")),
	}

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

// Send sends the mail via SMTP.
func (m *SMTPMailer) Send(email port.EmailMessage) error {
	if email.From.Address == "" {
		email.From = m.defaultFrom
	}

	html, text, err := renderMessage(email)
	if err != nil {
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
	msg.SetBodyString(mail.TypeTextPlain, text)
	msg.AddAlternativeString(mail.TypeTextHTML, html)

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"to":       email.To.Address,
			"subject":  email.Subject,
			"template": email.Template,
		}).Info("Sending email")
	}
	if err := m.client.DialAndSend(msg); err != nil {
		if logger.Logger != nil {
			logger.Logger.WithFields(map[string]interface{}{
				"to":    email.To.Address,
				"error": err,
			}).Error("Email send failed")
		}
		return err
	}
	if logger.Logger != nil {
		logger.Logger.WithField("to", email.To.Address).Info("Email sent successfully")
	}

	return nil
}

// renderMessage renders the email template to HTML and plain text.
func renderMessage(email port.EmailMessage) (html, text string, err error) {
	// Use pre-rendered content if available
	if email.PrerenderedHTML != "" {
		html = email.PrerenderedHTML
		text = email.PrerenderedText
		if text == "" {
			text, err = html2text.FromString(html, html2text.Options{PrettyTables: true})
		}
		return html, text, err
	}

	// Render from template
	buf := new(bytes.Buffer)
	if err := templates.ExecuteTemplate(buf, email.Template, email.Content); err != nil {
		return "", "", err
	}
	prem, err := premailer.NewPremailerFromString(buf.String(), premailer.NewOptions())
	if err != nil {
		return "", "", err
	}

	html, err = prem.Transform()
	if err != nil {
		return "", "", err
	}

	text, err = html2text.FromString(html, html2text.Options{PrettyTables: true})
	if err != nil {
		return "", "", err
	}
	return html, text, nil
}

func parseTemplates() error {
	templates = template.New("").Funcs(fMap)
	return filepath.Walk("./templates", func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, ".html") {
			_, err = templates.ParseFiles(path)
			return err
		}
		return err
	})
}

var fMap = template.FuncMap{
	"formatAsDate":     formatAsDate,
	"formatAsDuration": formatAsDuration,
}

func formatAsDate(t time.Time) string {
	year, month, day := t.Date()
	return fmt.Sprintf("%d.%d.%d", day, month, year)
}

func formatAsDuration(t time.Time) string {
	dur := time.Until(t)
	hours := int(dur.Hours())
	mins := int(dur.Minutes())

	v := ""
	if hours != 0 {
		v += strconv.Itoa(hours) + " hours and "
	}
	v += strconv.Itoa(mins) + " minutes"
	return v
}
