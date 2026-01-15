package test

import (
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/port"
)

// CapturingMailer records messages sent during tests.
// It implements port.EmailSender and captures all sent messages for verification.
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
	messages []port.EmailMessage
	ch       chan struct{}
}

// NewCapturingMailer creates a new capturing mailer for tests.
func NewCapturingMailer() *CapturingMailer {
	return &CapturingMailer{
		ch: make(chan struct{}, 16),
	}
}

// Send implements port.EmailSender by capturing the message.
func (m *CapturingMailer) Send(msg port.EmailMessage) error {
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
func (m *CapturingMailer) Messages() []port.EmailMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]port.EmailMessage, len(m.messages))
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

// Clear removes all captured messages.
func (m *CapturingMailer) Clear() {
	m.mu.Lock()
	m.messages = nil
	m.mu.Unlock()
}
