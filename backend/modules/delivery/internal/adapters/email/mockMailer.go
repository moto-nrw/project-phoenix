package email

import (
	"context"
	"log/slog"
	"sync/atomic"

	mailcontract "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
)

// MockMailer is a mock Mailer
type MockMailer struct {
	SendFn func(m mailcontract.Message) error
	// SendInvoked is atomic because Dispatcher delivers each message on its
	// own goroutine; flows that send several emails write this concurrently.
	SendInvoked atomic.Bool
}

func logMessage(m mailcontract.Message) {
	slog.Default().Info("mock email queued",
		slog.String("subject", m.Subject),
		slog.String("template", m.Template))
}

func NewMockMailer() *MockMailer {
	slog.Default().Warn("SMTP mailer not configured, using mock mailer (emails will be logged only)")
	return &MockMailer{
		SendFn: func(m mailcontract.Message) error {
			logMessage(m)
			return mailcontract.ErrDeliveryUnavailable
		},
	}
}

func (s *MockMailer) Send(m mailcontract.Message) error {
	s.SendInvoked.Store(true)
	return s.SendFn(m)
}

func (s *MockMailer) SendContext(ctx context.Context, m mailcontract.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.Send(m)
}
