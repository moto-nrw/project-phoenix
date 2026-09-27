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
	failures map[string]func(context.Context) error
}

// NewCapturingOutbox returns an empty capture.
func NewCapturingOutbox() *CapturingOutbox {
	return &CapturingOutbox{}
}

// FailKind makes every request of kind run fail instead of being recorded,
// so a suite can break one mail of a flow and watch the rest of it.
func (o *CapturingOutbox) FailKind(kind string, fail func(ctx context.Context) error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.failures == nil {
		o.failures = map[string]func(context.Context) error{}
	}
	o.failures[kind] = fail
}

// EnqueueOutbox records the request and accepts it, unless FailKind named
// its kind.
func (o *CapturingOutbox) EnqueueOutbox(ctx context.Context, req platform.OutboxEnqueueRequest) error {
	o.mu.Lock()
	fail := o.failures[req.Kind]
	if fail == nil {
		o.requests = append(o.requests, req)
	}
	o.mu.Unlock()
	if fail != nil {
		return fail(ctx)
	}
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
