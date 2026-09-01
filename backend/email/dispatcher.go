package email

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
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
}

// ErrDeliveryUnavailable reports a missing synchronous transport.
var ErrDeliveryUnavailable = errors.New("email delivery is unavailable")

// DeliveryObserver records one completed synchronous Delivery call. Type and
// Template are stable labels; neither contains a recipient or message body.
type DeliveryObserver func(transport, template, caller string, duration time.Duration, err error)

// Dispatcher manages asynchronous and fail-closed email delivery with retry behaviour.
type Dispatcher struct {
	mailer         Mailer
	logger         *slog.Logger
	observe        DeliveryObserver
	defaultRetry   int
	defaultBackoff []time.Duration
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (d *Dispatcher) getLogger() *slog.Logger {
	return cmp.Or(d.logger, slog.Default())
}

// NewDispatcher constructs a Dispatcher with sensible defaults.
func NewDispatcher(mailer Mailer, logger *slog.Logger, observers ...DeliveryObserver) *Dispatcher {
	dispatcher := &Dispatcher{
		mailer:       mailer,
		logger:       logger,
		defaultRetry: 3,
		defaultBackoff: []time.Duration{
			time.Minute,
			5 * time.Minute,
			15 * time.Minute,
		},
	}
	if len(observers) > 0 {
		dispatcher.observe = observers[0]
	}
	return dispatcher
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
	if d == nil || d.mailer == nil {
		return
	}

	// Defensive copy: capture message state before async delivery.
	// This prevents races if caller mutates the DeliveryRequest after Dispatch returns.
	messageCopy := req.Message

	cfg := d.resolveConfigWithMessage(req, messageCopy)
	go d.deliverWithRetry(ctx, cfg)
}

// Deliver sends an email synchronously. It returns only after the transport
// accepts the message or the retry policy is exhausted/cancelled. Fail-closed
// callers must use this method and must never fall back to Dispatch.
func (d *Dispatcher) Deliver(ctx context.Context, req DeliveryRequest) (err error) {
	started := time.Now()
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic in synchronous email delivery: %v", recovered)
			d.getLogger().Error("synchronous email delivery panic recovered", slog.Any("panic", recovered))
			sentry.CurrentHub().Recover(recovered)
		}
		if d != nil && d.observe != nil {
			d.observe("email", req.Message.Template, req.Metadata.Type, time.Since(started), err)
		}
	}()

	if d == nil || d.mailer == nil {
		return ErrDeliveryUnavailable
	}
	if ctx == nil {
		return fmt.Errorf("email delivery: context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	cfg := d.resolveConfigWithMessage(req, req.Message)
	return d.deliverSynchronously(ctx, cfg)
}

func (d *Dispatcher) deliverSynchronously(ctx context.Context, cfg dispatchConfig) error {
	for attempt := 1; attempt <= cfg.maxAttempts; attempt++ {
		sendErr := d.send(ctx, cfg.message)
		if sendErr == nil {
			d.invokeCallback(ctx, cfg, attempt, DeliveryStatusSent, nil, true)
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			sendErr = ctxErr
		}

		final := attempt == cfg.maxAttempts || ctx.Err() != nil
		d.invokeCallback(ctx, cfg, attempt, DeliveryStatusFailed, sendErr, final)
		if final {
			return fmt.Errorf("email delivery failed after %d attempt(s): %w", attempt, sendErr)
		}
		if waitErr := waitForBackoff(ctx, backoffDuration(cfg.backoff, attempt)); waitErr != nil {
			return waitErr
		}
	}
	return ErrDeliveryUnavailable
}

func (d *Dispatcher) send(ctx context.Context, message Message) error {
	if mailer, ok := d.mailer.(ContextMailer); ok {
		return mailer.SendContext(ctx, message)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return d.mailer.Send(message)
}

func waitForBackoff(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in email delivery: %v", r)
			d.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()
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

	d.getLogger().Warn("email send attempt failed",
		"type", cfg.metadata.Type,
		"reference_id", cfg.metadata.ReferenceID,
		"recipient", cfg.metadata.Recipient,
		"attempt", attempt,
		"max_attempts", cfg.maxAttempts,
		"error", err,
	)

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
