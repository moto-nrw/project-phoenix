package test

import (
	"errors"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
)

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

// Clear removes all captured messages.
func (m *CapturingMailer) Clear() {
	m.mu.Lock()
	m.messages = nil
	m.mu.Unlock()
}

// FlakyMailer fails a configurable number of initial attempts before succeeding.
// This is useful for testing retry behavior.
//
// Usage:
//
//	mailer := test.NewFlakyMailer(2, errors.New("smtp timeout"))
//	// First 2 sends will fail, then succeed
type FlakyMailer struct {
	mu        sync.Mutex
	failCount int
	err       error
	attempts  int
	messages  []email.Message
}

// NewFlakyMailer creates a mailer that fails the first n attempts.
func NewFlakyMailer(failures int, err error) *FlakyMailer {
	if failures < 0 {
		failures = 0
	}
	if err == nil {
		err = errors.New("mailer failure")
	}
	return &FlakyMailer{failCount: failures, err: err}
}

// Send implements email.Mailer with configurable failures.
func (m *FlakyMailer) Send(msg email.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts++
	if m.attempts <= m.failCount {
		return m.err
	}
	m.messages = append(m.messages, msg)
	return nil
}

// Attempts returns the number of send attempts made.
func (m *FlakyMailer) Attempts() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attempts
}

// Messages returns a copy of successfully sent messages.
func (m *FlakyMailer) Messages() []email.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]email.Message, len(m.messages))
	copy(out, m.messages)
	return out
}

// FailingMailer always fails with the configured error.
// Useful for testing error handling in email workflows.
type FailingMailer struct {
	err error
}

// NewFailingMailer creates a mailer that always fails.
func NewFailingMailer(err error) *FailingMailer {
	if err == nil {
		err = errors.New("mailer always fails")
	}
	return &FailingMailer{err: err}
}

// Send implements email.Mailer by always returning an error.
func (m *FailingMailer) Send(msg email.Message) error {
	return m.err
}
