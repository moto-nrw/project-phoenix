package mailer

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
)

type DeliveryStatus = port.DeliveryStatus

const (
	DeliveryStatusPending = port.DeliveryStatusPending
	DeliveryStatusSent    = port.DeliveryStatusSent
	DeliveryStatusFailed  = port.DeliveryStatusFailed
)

type DeliveryMetadata = port.DeliveryMetadata
type DeliveryResult = port.DeliveryResult
type DeliveryCallback = port.DeliveryCallback
type DeliveryRequest = port.DeliveryRequest

// Dispatcher manages asynchronous email delivery with retry behaviour.
type Dispatcher struct {
	mailer         port.EmailSender
	defaultRetry   int
	defaultBackoff []time.Duration
}

// NewDispatcher constructs a Dispatcher with sensible defaults.
func NewDispatcher(mailer port.EmailSender) *Dispatcher {
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
// Pass context.Background() if no specific context is needed for callbacks.
func (d *Dispatcher) Dispatch(ctx context.Context, req DeliveryRequest) {
	if d.mailer == nil {
		return
	}

	// Defensive copy: capture message state before async delivery.
	// This prevents races if caller mutates the DeliveryRequest after Dispatch returns.
	messageCopy := req.Message

	cfg := d.resolveConfigWithMessage(req, messageCopy)
	go d.deliverWithRetry(ctx, cfg)
}

// dispatchConfig holds resolved configuration for a delivery attempt
type dispatchConfig struct {
	message     port.EmailMessage
	metadata    DeliveryMetadata
	callback    DeliveryCallback
	maxAttempts int
	backoff     []time.Duration
}

// resolveConfigWithMessage applies defaults and prepares config for delivery using the provided message copy
func (d *Dispatcher) resolveConfigWithMessage(req DeliveryRequest, message port.EmailMessage) dispatchConfig {
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

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"type":       cfg.metadata.Type,
			"id":         cfg.metadata.ReferenceID,
			"recipient":  cfg.metadata.Recipient,
			"attempt":    attempt,
			"maxAttempt": cfg.maxAttempts,
			"error":      err,
		}).Warn("Email send attempt failed")
	}

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
