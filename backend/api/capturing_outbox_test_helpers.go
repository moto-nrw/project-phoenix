//go:build testhooks

package api

import (
	"context"
	"sync"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
)

// CapturingOutbox records what a flow enqueues on Delivery's mail outbox, so
// a behaviour suite can pin the mails a flow queues without wiring the
// outbox stack. It serves MailOutbox and MailResultOutbox, the outbox the
// root's Identity & Access and Enrollment mails go to.
type CapturingOutbox struct {
	mu       sync.Mutex
	requests []deliveryCompose.MailEnqueueRequest
	failures map[string]func(context.Context) error
	// failWhen fails the requests it returns an error for; nil records them.
	failWhen func(ctx context.Context, kind string, payload map[string]any) error
}

// NewCapturingOutbox returns an empty capture.
func NewCapturingOutbox() *CapturingOutbox {
	return &CapturingOutbox{}
}

// NewFailingCapturingOutbox returns an empty capture on which every request
// of kind runs fail instead of being recorded, so a suite can break one mail
// of a flow and watch the rest of it.
func NewFailingCapturingOutbox(kind string, fail func(ctx context.Context) error) *CapturingOutbox {
	return &CapturingOutbox{failures: map[string]func(context.Context) error{kind: fail}}
}

// NewCapturingOutboxFailingWhen returns an empty capture that fails every
// request failWhen returns an error for, given its kind and payload, and
// records the others, so a suite can break the mail of one recipient (#4602).
func NewCapturingOutboxFailingWhen(failWhen func(ctx context.Context, kind string, payload map[string]any) error) *CapturingOutbox {
	return &CapturingOutbox{failWhen: failWhen}
}

// EnqueueOutbox records the request and accepts it, unless the capture
// fails its kind.
func (o *CapturingOutbox) EnqueueOutbox(ctx context.Context, req deliveryCompose.MailEnqueueRequest) error {
	_, err := o.EnqueueOutboxWithResult(ctx, req)
	return err
}

// EnqueueOutboxWithResult records the request and exposes its stable capture
// ID to flows that need to order a follow-up delivery behind it.
func (o *CapturingOutbox) EnqueueOutboxWithResult(ctx context.Context, req deliveryCompose.MailEnqueueRequest) (deliveryCompose.MailEnqueued, error) {
	if o.failWhen != nil {
		if err := o.failWhen(ctx, req.Kind, req.Payload); err != nil {
			return deliveryCompose.MailEnqueued{}, err
		}
	}
	o.mu.Lock()
	fail := o.failures[req.Kind]
	if fail == nil {
		o.requests = append(o.requests, req)
	}
	id := int64(len(o.requests))
	o.mu.Unlock()
	if fail != nil {
		return deliveryCompose.MailEnqueued{}, fail(ctx)
	}
	return deliveryCompose.MailEnqueued{ID: id}, nil
}

// Requests returns a copy of every request enqueued so far, in order.
func (o *CapturingOutbox) Requests() []deliveryCompose.MailEnqueueRequest {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]deliveryCompose.MailEnqueueRequest, len(o.requests))
	copy(out, o.requests)
	return out
}
