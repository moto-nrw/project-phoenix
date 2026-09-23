package test

import (
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
)

// EmailMessage and Mailer name the delivery contract a test double sends or
// receives, so a behaviour suite outside the Delivery Platform can implement
// a refusing or flaky transport without importing the transport package.
type EmailMessage = email.Message
type Mailer = email.Mailer

// CapturingMailer records messages sent during tests.
// It implements email.Mailer and captures all sent messages for verification.
//
// Usage:
//
//	mailer := test.NewCapturingMailer()
//	// ... inject mailer into service ...
//	mailer.WaitForMessages(1, 500*time.Millisecond)
//	msgs := mailer.Messages()
//	assert.Equal(t, "expected subject", msgs[0].Subject)
type CapturingMailer struct {
	mu       sync.Mutex
	messages []email.Message
	ch       chan struct{}
}

// NewCapturingMailer creates a new capturing mailer for tests.
func NewCapturingMailer() *CapturingMailer {
	return &CapturingMailer{
		ch: make(chan struct{}, 16),
	}
}

// InDemoEnvironment puts the mail lock of APP_ENV=demo (#3465) in front of
// the capture, as the production transport carries it: the capture then
// holds only what would leave the demo environment.
func (m *CapturingMailer) InDemoEnvironment() email.Mailer {
	return email.RestrictToDemoMails(m, "demo", nil)
}

// Send implements email.Mailer by capturing the message.
func (m *CapturingMailer) Send(msg email.Message) error {
	m.mu.Lock()
	m.messages = append(m.messages, msg)
	m.mu.Unlock()

	select {
	case m.ch <- struct{}{}:
	default:
	}
	return nil
}

// Messages returns a copy of all captured messages.
func (m *CapturingMailer) Messages() []email.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]email.Message, len(m.messages))
	copy(out, m.messages)
	return out
}

// WaitForMessages waits until at least count messages have been captured
// or timeout is reached. Returns true if count was reached.
func (m *CapturingMailer) WaitForMessages(count int, timeout time.Duration) bool {
	if count <= 0 {
		return true
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		if len(m.Messages()) >= count {
			return true
		}
		select {
		case <-m.ch:
			if len(m.Messages()) >= count {
				return true
			}
		case <-timer.C:
			return len(m.Messages()) >= count
		}
	}
}

// Templates returns the template name of every captured message, in order.
func (m *CapturingMailer) Templates() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.messages))
	for i, msg := range m.messages {
		out[i] = msg.Template
	}
	return out
}

// MessageWithTemplate returns the first captured message of the template.
func (m *CapturingMailer) MessageWithTemplate(template string) (email.Message, bool) {
	for _, msg := range m.Messages() {
		if msg.Template == template {
			return msg, true
		}
	}
	return email.Message{}, false
}

// Clear removes all captured messages.
func (m *CapturingMailer) Clear() {
	m.mu.Lock()
	m.messages = nil
	m.mu.Unlock()
}
