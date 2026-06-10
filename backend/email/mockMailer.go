package email

import (
	"log/slog"
	"sync/atomic"
)

// MockMailer is a mock Mailer
type MockMailer struct {
	SendFn func(m Message) error
	// SendInvoked is atomic because Dispatcher delivers each message on its
	// own goroutine; flows that send several emails write this concurrently.
	SendInvoked atomic.Bool
}

func logMessage(m Message) {
	slog.Default().Info("mock email queued",
		slog.String("to", m.To.Address),
		slog.String("subject", m.Subject),
		slog.String("template", m.Template))
}

func NewMockMailer() *MockMailer {
	slog.Default().Warn("SMTP mailer not configured, using mock mailer (emails will be logged only)")
	return &MockMailer{
		SendFn: func(m Message) error {
			logMessage(m)
			return nil
		},
	}
}

func (s *MockMailer) Send(m Message) error {
	s.SendInvoked.Store(true)
	return s.SendFn(m)
}
