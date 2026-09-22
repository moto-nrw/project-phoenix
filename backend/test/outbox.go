package test

import (
	"context"
	"sync"

	"github.com/moto-nrw/project-phoenix/models/platform"
)

// CapturingOutbox records what a flow enqueues on the e-mail outbox, so a
// behaviour suite can pin the mails a flow queues without wiring the outbox
// stack or naming the Organisation & Tenancy outbox contract itself. It
// satisfies the outbox port the composed flows consume.
type CapturingOutbox struct {
	mu       sync.Mutex
	requests []platform.OutboxEnqueueRequest
}

// NewCapturingOutbox returns an empty capture.
func NewCapturingOutbox() *CapturingOutbox {
	return &CapturingOutbox{}
}

// EnqueueOutbox records the request and accepts it.
func (o *CapturingOutbox) EnqueueOutbox(_ context.Context, req platform.OutboxEnqueueRequest) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.requests = append(o.requests, req)
	return nil
}

// Requests returns a copy of every request enqueued so far, in order.
func (o *CapturingOutbox) Requests() []platform.OutboxEnqueueRequest {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]platform.OutboxEnqueueRequest, len(o.requests))
	copy(out, o.requests)
	return out
}
