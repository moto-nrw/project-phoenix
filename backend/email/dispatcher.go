package email

import (
	"context"
	"log"
	"time"
)

// DeliveryStatus represents the current state of an outbound email.
type DeliveryStatus string

const (
	// DeliveryStatusPending indicates the message is awaiting another attempt.
	DeliveryStatusPending DeliveryStatus = "pending"
	// DeliveryStatusSent indicates the message was delivered successfully.
	DeliveryStatusSent DeliveryStatus = "sent"
	// DeliveryStatusFailed indicates the message failed to send.
	DeliveryStatusFailed DeliveryStatus = "failed"
)

// DeliveryMetadata captures contextual identifiers for the email.
type DeliveryMetadata struct {
	// Type is a short identifier for the feature (e.g., "invitation", "password_reset").
	Type string
	// ReferenceID is a database identifier associated with the email (e.g., invitation token ID).
	ReferenceID int64
	// Token optionally stores the public token value for diagnostics.
	Token string
	// Recipient holds the destination email address for logging.
	Recipient string
}

// DeliveryResult captures the outcome of an email attempt.
type DeliveryResult struct {
	Metadata   DeliveryMetadata
	Attempt    int
	MaxAttempt int
	Status     DeliveryStatus
	Err        error
	SentAt     time.Time
	Final      bool
}

// DeliveryCallback receives delivery results from the dispatcher.
type DeliveryCallback func(ctx context.Context, result DeliveryResult)

// DeliveryRequest defines a new email to be dispatched.
type DeliveryRequest struct {
	Message       Message
	Metadata      DeliveryMetadata
	Callback      DeliveryCallback
	MaxAttempts   int
	BackoffPolicy []time.Duration
	// Context is used for callback invocations. If nil, context.Background() is used.
	Context context.Context
}

// Dispatcher manages asynchronous email delivery with retry behaviour.
type Dispatcher struct {
	mailer         Mailer
	defaultRetry   int
	defaultBackoff []time.Duration
}

// NewDispatcher constructs a Dispatcher with sensible defaults.
func NewDispatcher(mailer Mailer) *Dispatcher {
	return &Dispatcher{
		mailer:       mailer,
		defaultRetry: 3,
		defaultBackoff: []time.Duration{
			time.Minute,
			5 * time.Minute,
			15 * time.Minute,
		},
	}
}

// SetDefaults overrides the default retry behaviour; primarily used in tests.
func (d *Dispatcher) SetDefaults(maxAttempts int, backoff []time.Duration) {
	if d == nil {
		return
	}
	if maxAttempts > 0 {
		d.defaultRetry = maxAttempts
	}
	if len(backoff) > 0 {
		copied := make([]time.Duration, len(backoff))
		copy(copied, backoff)
		d.defaultBackoff = copied
	}
}

// Dispatch sends an email asynchronously; results are communicated via callback.
func (d *Dispatcher) Dispatch(req DeliveryRequest) {
	if d.mailer == nil {
		return
	}

	maxAttempts := req.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = d.defaultRetry
	}

	backoff := req.BackoffPolicy
	if len(backoff) == 0 {
		backoff = d.defaultBackoff
	}

	callbackCtx := req.Context
	if callbackCtx == nil {
		callbackCtx = context.Background()
	}

	messageCopy := req.Message // copy avoids races if caller mutates

	go func() {
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			err := d.mailer.Send(messageCopy)
			if err == nil {
				if req.Callback != nil {
					req.Callback(callbackCtx, DeliveryResult{
						Metadata:   req.Metadata,
						Attempt:    attempt,
						MaxAttempt: maxAttempts,
						Status:     DeliveryStatusSent,
						SentAt:     time.Now(),
						Final:      true,
					})
				}
				return
			}

			log.Printf("Email send attempt failed type=%s id=%d recipient=%s attempt=%d/%d err=%v",
				req.Metadata.Type, req.Metadata.ReferenceID, req.Metadata.Recipient, attempt, maxAttempts, err)

			if req.Callback != nil {
				req.Callback(callbackCtx, DeliveryResult{
					Metadata:   req.Metadata,
					Attempt:    attempt,
					MaxAttempt: maxAttempts,
					Status:     DeliveryStatusFailed,
					Err:        err,
					Final:      attempt == maxAttempts,
				})
			}

			if attempt == maxAttempts {
				return
			}

			sleepFor := backoffDuration(backoff, attempt)
			time.Sleep(sleepFor)
		}
	}()
}

func backoffDuration(backoff []time.Duration, attempt int) time.Duration {
	index := attempt - 1
	if index < 0 {
		index = 0
	}
	if index >= len(backoff) {
		return backoff[len(backoff)-1]
	}
	return backoff[index]
}
