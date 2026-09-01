package email

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"strings"
	"time"

	"github.com/wneessen/go-mail"
)

// SMTPMailer is a SMTP mailer.
type SMTPMailer struct {
	client      *mail.Client
	defaultFrom Email
	templates   *template.Template
	logger      *slog.Logger
}

type MailerConfig struct {
	Host        string
	Port        int
	User        string
	Password    string
	DefaultFrom Email
	TemplateDir string
	Logger      *slog.Logger
	AppEnv      string
}

const smtpShutdownTimeout = time.Second

// NewMailer returns a configured SMTP Mailer.
func NewMailer(cfg MailerConfig) (Mailer, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	appEnv := strings.ToLower(strings.TrimSpace(cfg.AppEnv))
	if appEnv == "test" {
		return NewMockMailer(), nil
	}
	if cfg.Host == "" {
		switch appEnv {
		case "production", "staging":
			return nil, fmt.Errorf("EMAIL_SMTP_HOST is required when APP_ENV=%s", appEnv)
		}
		return NewMockMailer(), nil
	}
	templates, err := parseTemplates(cfg.TemplateDir)
	if err != nil {
		return nil, err
	}

	// Configure TLS and auth based on port and credentials
	var clientOpts []mail.Option
	switch {
	case cfg.User == "" && cfg.Password == "":
		// No credentials: plain SMTP without TLS (e.g., Mailpit on port 1025)
		clientOpts = []mail.Option{
			mail.WithPort(cfg.Port),
			mail.WithTLSPolicy(mail.NoTLS),
		}
	case cfg.Port == 465:
		// Port 465: Implicit SSL/TLS (SSL from connection start)
		clientOpts = []mail.Option{
			mail.WithSSLPort(false), // Use implicit SSL
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(cfg.User),
			mail.WithPassword(cfg.Password),
		}
	default:
		// Port 587: STARTTLS (upgrade to TLS after connect)
		clientOpts = []mail.Option{
			mail.WithPort(cfg.Port),
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(cfg.User),
			mail.WithPassword(cfg.Password),
			mail.WithTLSPolicy(mail.TLSMandatory),
		}
	}

	client, err := mail.NewClient(cfg.Host, clientOpts...)
	if err != nil {
		return nil, err
	}
	s := &SMTPMailer{
		client:      client,
		defaultFrom: cfg.DefaultFrom,
		templates:   templates,
		logger:      logger,
	}
	return s, nil
}

// buildMessage turns a Message into the go-mail message that goes on the
// wire. Split out of Send so the header contract (From, Reply-To,
// List-Unsubscribe) is testable without an SMTP dial.
func (m *SMTPMailer) buildMessage(email Message) (*mail.Msg, error) {
	if err := email.parse(m.templates); err != nil {
		return nil, err
	}

	msg := mail.NewMsg()
	// Use go-mail's typed setters so RFC 5322 special characters in the
	// display name (e.g. "@", ",", quotes) and non-ASCII names are encoded
	// correctly instead of producing a malformed header.
	if err := msg.FromFormat(email.From.Name, email.From.Address); err != nil {
		return nil, fmt.Errorf("failed to set from address: %w", err)
	}
	if err := msg.AddToFormat(email.To.Name, email.To.Address); err != nil {
		return nil, fmt.Errorf("failed to set to address: %w", err)
	}
	// Replies to tenant-bound mail must reach the OGS, not moto. The From stays
	// the central authenticated sender so SPF/DKIM alignment is untouched —
	// only the return path moves (#1936).
	if email.ReplyTo.Address != "" {
		if err := msg.ReplyToFormat(email.ReplyTo.Name, email.ReplyTo.Address); err != nil {
			return nil, fmt.Errorf("failed to set reply-to address: %w", err)
		}
	}
	msg.Subject(email.Subject)
	msg.SetGenHeader(mail.HeaderListUnsubscribe, fmt.Sprintf("<mailto:%s?subject=unsubscribe>", email.From.Address))
	msg.SetGenHeader(mail.HeaderListUnsubscribePost, "List-Unsubscribe=One-Click")
	msg.SetBodyString(mail.TypeTextPlain, email.text)
	msg.AddAlternativeString(mail.TypeTextHTML, email.html)
	return msg, nil
}

// Send sends the mail via smtp.
func (m *SMTPMailer) Send(email Message) error {
	return m.SendContext(context.Background(), email)
}

// SendContext terminates an in-flight SMTP exchange when the caller cancels.
func (m *SMTPMailer) SendContext(ctx context.Context, email Message) error {
	if email.From.Address == "" {
		email.From = m.defaultFrom
	}

	msg, err := m.buildMessage(email)
	if err != nil {
		return err
	}

	m.logger.Info("sending email",
		slog.String("to", email.To.Address),
		slog.String("subject", email.Subject),
		slog.String("template", email.Template))
	err = m.sendMessageContext(ctx, msg)
	if err != nil {
		m.logger.Error("email send failed",
			slog.String("to", email.To.Address),
			slog.Any("error", err),
		)
		return err
	}
	m.logger.Info("email sent successfully",
		slog.String("to", email.To.Address))

	return nil
}

func (m *SMTPMailer) sendMessageContext(ctx context.Context, msg *mail.Msg) error {
	client, err := m.client.DialToSMTPClientWithContext(ctx)
	if err != nil {
		return err
	}

	result := make(chan error, 1)
	go func() {
		err := m.client.SendWithSMTPClient(client, msg)
		if err == nil {
			// SMTP has accepted the message before QUIT. A failed graceful close
			// must not turn that acceptance into a retry (and duplicate email).
			_ = client.UpdateDeadline(smtpShutdownTimeout)
			if closeErr := m.client.CloseWithSMTPClient(client); closeErr != nil {
				_ = client.Close()
			}
		} else {
			_ = client.Close()
		}
		result <- err
	}()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		// Prefer an already-completed SMTP result when cancellation races with
		// transport completion. Otherwise force-close the socket: go-mail's
		// graceful QUIT path can itself block while DATA is in progress.
		select {
		case err := <-result:
			return err
		default:
		}
		_ = client.Text.Close()
		if err := <-result; err == nil {
			return nil
		}
		return ctx.Err()
	}
}
