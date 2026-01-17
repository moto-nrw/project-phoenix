package mailer

import (
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
)

// MockMailer is a mock Mailer that implements port.EmailSender.
// It logs email metadata instead of sending them, useful for development/testing.
type MockMailer struct {
	SendFn      func(m port.EmailMessage) error
	SendInvoked bool
}

// Ensure MockMailer implements port.EmailSender
var _ port.EmailSender = (*MockMailer)(nil)

func logMessage(m port.EmailMessage) {
	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"template": m.Template,
		}).Info("MockMailer email queued")
	}
}

// NewMockMailer creates a MockMailer that logs emails instead of sending them.
func NewMockMailer() *MockMailer {
	if logger.Logger != nil {
		logger.Logger.Warn("SMTP Mailer not configured - logging email metadata to stdout")
	}
	return &MockMailer{
		SendFn: func(m port.EmailMessage) error {
			logMessage(m)
			return nil
		},
	}
}

// Send logs the email instead of sending it.
func (s *MockMailer) Send(m port.EmailMessage) error {
	s.SendInvoked = true
	return s.SendFn(m)
}
