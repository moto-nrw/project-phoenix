package timetracking

import (
	"context"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
)

type capturingAbsenceEmails struct {
	mu       sync.Mutex
	messages []AbsenceEmailMessage
	changed  chan struct{}
}

func newCapturingAbsenceEmails() *capturingAbsenceEmails {
	return &capturingAbsenceEmails{changed: make(chan struct{})}
}

func (c *capturingAbsenceEmails) Enqueue(ctx context.Context, message AbsenceEmailMessage) error {
	tenant.RegisterAfterCommit(ctx, func() { c.capture(message) })
	return nil
}

func (c *capturingAbsenceEmails) capture(message AbsenceEmailMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, message)
	close(c.changed)
	c.changed = make(chan struct{})
}

func (c *capturingAbsenceEmails) Messages() []AbsenceEmailMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]AbsenceEmailMessage(nil), c.messages...)
}

func (c *capturingAbsenceEmails) WaitForMessages(count int, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		c.mu.Lock()
		ready := len(c.messages) >= count
		changed := c.changed
		c.mu.Unlock()
		if ready {
			return true
		}
		select {
		case <-changed:
		case <-timer.C:
			return false
		}
	}
}
