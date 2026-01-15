package email

import "github.com/moto-nrw/project-phoenix/logging"

// MockMailer is a mock Mailer
type MockMailer struct {
	SendFn      func(m Message) error
	SendInvoked bool
}

func logMessage(m Message) {
	logging.Logger.WithFields(map[string]interface{}{
		"to":       m.To.Address,
		"subject":  m.Subject,
		"template": m.Template,
	}).Info("MockMailer email queued")
}

func NewMockMailer() *MockMailer {
	logging.Logger.Warn("SMTP Mailer not configured - printing emails to stdout")
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
