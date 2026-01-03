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
// The message is copied before async delivery to avoid races if caller mutates the request.
func (d *Dispatcher) Dispatch(req DeliveryRequest) {
	if d.mailer == nil {
		return
	}

	ctx := req.Context
	if ctx == nil {
		ctx = context.Background()
	}

	// Defensive copy: capture message state before async delivery.
	// This prevents races if caller mutates the DeliveryRequest after Dispatch returns.
	messageCopy := req.Message

	cfg := d.resolveConfigWithMessage(req, messageCopy)
	go d.deliverWithRetry(ctx, cfg)
}

// dispatchConfig holds resolved configuration for a delivery attempt
type dispatchConfig struct {
	message     Message
	metadata    DeliveryMetadata
	callback    DeliveryCallback
	maxAttempts int
	backoff     []time.Duration
}

// resolveConfigWithMessage applies defaults and prepares config for delivery using the provided message copy
func (d *Dispatcher) resolveConfigWithMessage(req DeliveryRequest, message Message) dispatchConfig {
	cfg := dispatchConfig{
		message:     message,
		metadata:    req.Metadata,
		callback:    req.Callback,
		maxAttempts: req.MaxAttempts,
		backoff:     req.BackoffPolicy,
	}

	if cfg.maxAttempts <= 0 {
		cfg.maxAttempts = d.defaultRetry
	}
	if len(cfg.backoff) == 0 {
		cfg.backoff = d.defaultBackoff
	}
	return cfg
}

// deliverWithRetry attempts to send the email with retries
func (d *Dispatcher) deliverWithRetry(ctx context.Context, cfg dispatchConfig) {
	for attempt := 1; attempt <= cfg.maxAttempts; attempt++ {
		if d.tryDelivery(ctx, cfg, attempt) {
			return
		}
		if attempt < cfg.maxAttempts {
			time.Sleep(backoffDuration(cfg.backoff, attempt))
		}
	}
}

// tryDelivery attempts a single delivery; returns true if successful
func (d *Dispatcher) tryDelivery(ctx context.Context, cfg dispatchConfig, attempt int) bool {
	err := d.mailer.Send(cfg.message)
	if err == nil {
		d.invokeCallback(ctx, cfg, attempt, DeliveryStatusSent, nil, true)
		return true
	}

	log.Printf("Email send attempt failed type=%s id=%d recipient=%s attempt=%d/%d err=%v",
		cfg.metadata.Type, cfg.metadata.ReferenceID, cfg.metadata.Recipient, attempt, cfg.maxAttempts, err)

	d.invokeCallback(ctx, cfg, attempt, DeliveryStatusFailed, err, attempt == cfg.maxAttempts)
	return false
}

// invokeCallback safely calls the callback if present
func (d *Dispatcher) invokeCallback(ctx context.Context, cfg dispatchConfig, attempt int, status DeliveryStatus, err error, final bool) {
	if cfg.callback == nil {
		return
	}
	cfg.callback(ctx, DeliveryResult{
		Metadata:   cfg.metadata,
		Attempt:    attempt,
		MaxAttempt: cfg.maxAttempts,
		Status:     status,
		Err:        err,
		SentAt:     time.Now(),
		Final:      final,
	})
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
