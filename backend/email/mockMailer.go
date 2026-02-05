package email

import "log/slog"

// MockMailer is a mock Mailer
type MockMailer struct {
	SendFn      func(m Message) error
	SendInvoked bool
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
	s.SendInvoked = true
	return s.SendFn(m)
}
