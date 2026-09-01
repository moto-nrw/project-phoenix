// Package delivery is the public durable Delivery capability.
package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

type Transport string

const (
	TransportEmail Transport = "email"
	TransportPush  Transport = "push"
)

const (
	ReachabilityOK       = "ok"
	ReachabilityNoEmail  = "no_email"
	ReachabilityNoPortal = "no_portal"
	ReachabilityExcluded = "excluded"
)

type State string

const (
	StatePending    State = "pending"
	StateClaimed    State = "claimed"
	StateSent       State = "sent"
	StateDeadLetter State = "dead_letter"
	StateCancelled  State = "cancelled"
)

type RelatedEntity struct {
	Type string
	ID   int64
}

type EmailRecipient struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

type EmailIntent struct {
	TenantID       int64
	Template       string
	Recipient      EmailRecipient
	Payload        json.RawMessage
	IdempotencyKey string
	Related        RelatedEntity
}

type EmailDelivery struct {
	ID                int64
	OutboxID          *int64
	GuardianProfileID *int64
	AccountID         *int64
	RecipientEmail    *string
	Reachability      string
}

func (d EmailDelivery) Queued() bool { return d.OutboxID != nil }

type EmailDeliveryStatus struct {
	DeliveryID        int64      `json:"delivery_id"`
	GuardianProfileID *int64     `json:"guardian_profile_id,omitempty"`
	AccountID         *int64     `json:"account_id,omitempty"`
	FirstName         string     `json:"first_name"`
	LastName          string     `json:"last_name"`
	RecipientEmail    *string    `json:"recipient_email,omitempty"`
	Reachability      string     `json:"reachability"`
	EmailStatus       string     `json:"email_status"`
	LastError         *string    `json:"last_error,omitempty"`
	SentAt            *time.Time `json:"sent_at,omitempty"`
	Attempts          int        `json:"attempts"`
}

type GuardianDisplay struct {
	GuardianProfileID int64
	FirstName         string
	LastName          string
}

type PushRecipient struct {
	SubscriptionID int64     `json:"subscription_id"`
	AccountID      int64     `json:"account_id"`
	Endpoint       string    `json:"endpoint"`
	P256DH         string    `json:"p256dh"`
	Auth           string    `json:"auth"`
	Portal         string    `json:"portal"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type PushPayload struct {
	Title    string `json:"title"`
	Body     string `json:"body,omitempty"`
	DeepLink string `json:"deepLink,omitempty"`
	Type     string `json:"type,omitempty"`
	Priority string `json:"priority,omitempty"`
}

type PushIntent struct {
	TenantID       int64
	Template       string
	Recipient      PushRecipient
	Payload        PushPayload
	IdempotencyKey string
	Related        RelatedEntity
}

type Enqueued struct {
	ID        int64
	Transport Transport
	Duplicate bool
}

var (
	ErrCancelled           = errors.New("delivery: provider cancelled the intent")
	ErrIdempotencyConflict = errors.New("delivery: idempotency key reused with different intent")
)

type Status struct {
	ID             int64
	Transport      Transport
	State          State
	Attempts       int
	LastError      *string
	ProviderResult *ProviderResult
	NextAttemptAt  time.Time
	SentAt         *time.Time
	DeadLetterAt   *time.Time
	CancelledAt    *time.Time
}

type Observation struct {
	Operation string
	Transport Transport
	Template  string
	Duration  time.Duration
	Count     int
	Err       error
}

type engine interface {
	EnqueueEmail(context.Context, EmailIntent) (Enqueued, error)
	EnqueuePush(context.Context, PushIntent) (Enqueued, error)
	Cancel(context.Context, int64, Transport, RelatedEntity, string) (int64, error)
	Statuses(context.Context, int64, Transport, RelatedEntity) ([]Status, error)
	ReplaceEmailDeliveries(context.Context, int64, RelatedEntity, []EmailDelivery) error
	DeleteEmailDeliveries(context.Context, int64, RelatedEntity) (int64, error)
	EmailDeliveryStatuses(context.Context, int64, RelatedEntity) ([]EmailDeliveryStatus, error)
	AttachEmailOutbox(context.Context, int64, int64, int64) error
	ClaimFailedEmailDelivery(context.Context, int64, int64) (bool, error)
}

func (m *Module) ReplaceEmailDeliveries(ctx context.Context, tenantID int64, related RelatedEntity, rows []EmailDelivery) error {
	if tenantID <= 0 {
		return errors.New("delivery: tenant is required")
	}
	related.Type = strings.TrimSpace(related.Type)
	if err := validateRelated(related); err != nil {
		return err
	}
	for _, row := range rows {
		if err := validateReachability(row.Reachability); err != nil {
			return err
		}
	}
	return m.engine.ReplaceEmailDeliveries(ctx, tenantID, related, rows)
}

func (m *Module) DeleteEmailDeliveries(ctx context.Context, tenantID int64, related RelatedEntity) (int64, error) {
	if tenantID <= 0 {
		return 0, errors.New("delivery: tenant is required")
	}
	related.Type = strings.TrimSpace(related.Type)
	if err := validateRelated(related); err != nil {
		return 0, err
	}
	return m.engine.DeleteEmailDeliveries(ctx, tenantID, related)
}

func (m *Module) EmailDeliveryStatuses(ctx context.Context, tenantID int64, related RelatedEntity) ([]EmailDeliveryStatus, error) {
	if tenantID <= 0 {
		return nil, errors.New("delivery: tenant is required")
	}
	related.Type = strings.TrimSpace(related.Type)
	if err := validateRelated(related); err != nil {
		return nil, err
	}
	return m.engine.EmailDeliveryStatuses(ctx, tenantID, related)
}

func (m *Module) AttachEmailOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error {
	if tenantID <= 0 || deliveryID <= 0 || outboxID <= 0 {
		return errors.New("delivery: tenant, delivery, and outbox ids are required")
	}
	return m.engine.AttachEmailOutbox(ctx, tenantID, deliveryID, outboxID)
}

func (m *Module) ClaimFailedEmailDelivery(ctx context.Context, tenantID, deliveryID int64) (bool, error) {
	if tenantID <= 0 || deliveryID <= 0 {
		return false, errors.New("delivery: tenant and delivery ids are required")
	}
	return m.engine.ClaimFailedEmailDelivery(ctx, tenantID, deliveryID)
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("delivery: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) EnqueueEmail(ctx context.Context, intent EmailIntent) (Enqueued, error) {
	intent.Template = strings.TrimSpace(intent.Template)
	intent.IdempotencyKey = strings.TrimSpace(intent.IdempotencyKey)
	intent.Related.Type = strings.TrimSpace(intent.Related.Type)
	intent.Recipient.Address = strings.TrimSpace(intent.Recipient.Address)
	intent.Recipient.Name = strings.TrimSpace(intent.Recipient.Name)
	if intent.TenantID <= 0 {
		return Enqueued{}, errors.New("delivery: tenant is required")
	}
	if intent.Template == "" {
		return Enqueued{}, errors.New("delivery: email template is required")
	}
	parsedRecipient, err := mail.ParseAddress(intent.Recipient.Address)
	if err != nil {
		return Enqueued{}, fmt.Errorf("delivery: invalid email recipient: %w", err)
	}
	intent.Recipient.Address = parsedRecipient.Address
	if intent.Recipient.Name == "" {
		intent.Recipient.Name = parsedRecipient.Name
	}
	if len(intent.Payload) == 0 || !json.Valid(intent.Payload) {
		return Enqueued{}, errors.New("delivery: email payload must be valid JSON")
	}
	if err := validateRelated(intent.Related); err != nil {
		return Enqueued{}, err
	}
	return m.engine.EnqueueEmail(ctx, intent)
}

func (m *Module) EnqueuePush(ctx context.Context, intent PushIntent) (Enqueued, error) {
	intent.Template = strings.TrimSpace(intent.Template)
	intent.IdempotencyKey = strings.TrimSpace(intent.IdempotencyKey)
	intent.Related.Type = strings.TrimSpace(intent.Related.Type)
	intent.Recipient.Endpoint = strings.TrimSpace(intent.Recipient.Endpoint)
	intent.Recipient.P256DH = strings.TrimSpace(intent.Recipient.P256DH)
	intent.Recipient.Auth = strings.TrimSpace(intent.Recipient.Auth)
	intent.Payload.Title = strings.TrimSpace(intent.Payload.Title)
	if intent.TenantID <= 0 {
		return Enqueued{}, errors.New("delivery: tenant is required")
	}
	if intent.Template == "" || intent.Recipient.Endpoint == "" || intent.Recipient.P256DH == "" || intent.Recipient.Auth == "" {
		return Enqueued{}, errors.New("delivery: push template and recipient keys are required")
	}
	if intent.Payload.Title == "" {
		return Enqueued{}, errors.New("delivery: push title is required")
	}
	if err := validateRelated(intent.Related); err != nil {
		return Enqueued{}, err
	}
	return m.engine.EnqueuePush(ctx, intent)
}

func (m *Module) Cancel(ctx context.Context, tenantID int64, transport Transport, related RelatedEntity, reason string) (int64, error) {
	if tenantID <= 0 {
		return 0, errors.New("delivery: tenant is required")
	}
	if err := validateTransport(transport); err != nil {
		return 0, err
	}
	related.Type = strings.TrimSpace(related.Type)
	if err := validateRelated(related); err != nil {
		return 0, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return 0, errors.New("delivery: cancellation reason is required")
	}
	return m.engine.Cancel(ctx, tenantID, transport, related, reason)
}

func (m *Module) Statuses(ctx context.Context, tenantID int64, transport Transport, related RelatedEntity) ([]Status, error) {
	if tenantID <= 0 {
		return nil, errors.New("delivery: tenant is required")
	}
	if err := validateTransport(transport); err != nil {
		return nil, err
	}
	related.Type = strings.TrimSpace(related.Type)
	if err := validateRelated(related); err != nil {
		return nil, err
	}
	return m.engine.Statuses(ctx, tenantID, transport, related)
}

func validateTransport(transport Transport) error {
	if transport != TransportEmail && transport != TransportPush {
		return fmt.Errorf("delivery: unknown transport %q", transport)
	}
	return nil
}

func validateRelated(related RelatedEntity) error {
	if (related.Type == "") != (related.ID == 0) || related.ID < 0 {
		return errors.New("delivery: related entity type and positive id must be supplied together")
	}
	return nil
}

func validateReachability(value string) error {
	switch value {
	case ReachabilityOK, ReachabilityNoEmail, ReachabilityNoPortal, ReachabilityExcluded:
		return nil
	default:
		return fmt.Errorf("delivery: unknown reachability %q", value)
	}
}
