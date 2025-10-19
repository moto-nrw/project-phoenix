package email

import "log"

// MockMailer is a mock Mailer
type MockMailer struct {
	SendFn      func(m Message) error
	SendInvoked bool
}

func logMessage(m Message) {
	log.Printf("MockMailer email queued to=%s subject=%s template=%s", m.To.Address, m.Subject, m.Template)
}

func NewMockMailer() *MockMailer {
	log.Println("ATTENTION: SMTP Mailer not configured => printing emails to stdout")
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
