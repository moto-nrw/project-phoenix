// Package mail publishes Delivery's native mail values and transport capabilities.
package mail

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
)

type Mailer interface {
	Send(Message) error
}

// ContextMailer is implemented by transports that report caller cancellation.
// Mailer stays source-compatible for the durable outbox and existing adapters;
// synchronous Delivery requires the production SMTP adapter to implement this
// stronger contract.
type ContextMailer interface {
	SendContext(context.Context, Message) error
}

// Message struct holds all parts of a specific email Message.
type Message struct {
	From Email
	To   Email
	// ReplyTo is where answers go when that is not the sender. Tenant-bound
	// mail (Eltern-Einladung, Anmeldung, Elternmitteilung) keeps the central
	// authenticated From and points replies at the OGS instead, so a parent
	// answering does not reach moto (#1936). The zero value emits no header,
	// which is what every global system mail keeps.
	ReplyTo  Email
	Subject  string
	Template string
	Content  any
}

// Email struct holds email address and recipient name.
type Email struct {
	Name    string
	Address string
}

// NewEmail returns an email address.
func NewEmail(name, address string) Email {
	return Email{
		Name:    name,
		Address: address,
	}
}

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

// interactiveBackoff spaces the retries of a mail a person is waiting for.
var interactiveBackoff = []time.Duration{time.Second, 5 * time.Second, 15 * time.Second}

// WithInteractiveRetry gives a mail a person is waiting for, such as an
// invitation, a sign-in code or a password reset, its retry policy: three
// attempts, one, five and fifteen seconds apart. A non-nil backoff replaces
// the spacing, which tests use.
func WithInteractiveRetry(request DeliveryRequest, backoff []time.Duration) DeliveryRequest {
	if backoff == nil {
		backoff = append([]time.Duration(nil), interactiveBackoff...)
	}
	request.MaxAttempts = 3
	request.BackoffPolicy = backoff
	return request
}

type MailerConfig struct {
	Host        string
	Port        int
	User        string
	Password    string
	DefaultFrom Email
	TemplateDir string
	Logger      *slog.Logger
	AppEnv      string
}

// ReplyToIdentity is the reply address for a tenant-bound email.
type ReplyToIdentity struct {
	Name    string
	Address string
}

// IsZero reports whether no reply address is configured.
func (i ReplyToIdentity) IsZero() bool { return i.Address == "" }

// ReplyToResolver resolves the reply address of one tenant.
type ReplyToResolver interface {
	ResolveReplyTo(ctx context.Context, tenantID int64) (ReplyToIdentity, error)
}

// ResolveReplyToIdentity degrades a failed lookup to no reply address so a
// missing return path cannot prevent a message from being sent.
func ResolveReplyToIdentity(ctx context.Context, resolver ReplyToResolver, tenantID int64, logger *slog.Logger) ReplyToIdentity {
	if resolver == nil {
		return ReplyToIdentity{}
	}
	identity, err := resolver.ResolveReplyTo(ctx, tenantID)
	if err != nil {
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("failed to resolve tenant reply-to, sending without it",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return ReplyToIdentity{}
	}
	return identity
}

// The two mails of the public demo (#3465). Only they leave the demo
// environment.
const (
	TemplateDemoAccess = "demo-access.html"
	TemplateDemoLead   = "demo-lead.html"
)

// ErrNotDeliveredInDemo reports a mail the demo environment keeps although
// its flow depends on it, so that flow fails instead of waiting forever.
var ErrNotDeliveredInDemo = errors.New("email is not delivered in the demo environment")

// IsDemoEnvironment reports whether appEnv names the public demo. The demo
// routes, the demo access and the mail lock all decide with it, so none of
// them exists without the others.
func IsDemoEnvironment(appEnv string) bool {
	return strings.EqualFold(strings.TrimSpace(appEnv), "demo")
}

// Dispatcher is the native mail delivery capability used by composition roots.
// Implementations preserve synchronous acceptance and asynchronous callbacks.
type Dispatcher interface {
	Dispatch(context.Context, DeliveryRequest)
	Deliver(context.Context, DeliveryRequest) error
}
