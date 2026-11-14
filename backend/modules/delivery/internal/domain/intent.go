package domain

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrCancelled = errors.New("delivery provider cancelled the intent")
	ErrDeferred  = errors.New("delivery provider deferred the intent")
	// ErrThrottled refines ErrDeferred: the mail server asked to send later.
	ErrThrottled = errors.New("delivery provider throttled the intent")
	ErrPermanent = errors.New("delivery provider rejected the intent permanently")
	// ErrRecipientRejected refines ErrPermanent: the server refused this address.
	ErrRecipientRejected   = errors.New("delivery provider rejected the recipient permanently")
	ErrIdempotencyConflict = errors.New("delivery idempotency key reused with different intent")
)

type Transport string

const (
	TransportEmail Transport = "email"
	TransportPush  Transport = "push"
)

type State string

const (
	StatePending    State = "pending"
	StateClaimed    State = "claimed"
	StateSent       State = "sent"
	StateDeadLetter State = "dead_letter"
	StateCancelled  State = "cancelled"
)

type EmailDelivery struct {
	ID                int64
	TenantID          int64
	RelatedEntityType string
	RelatedEntityID   int64
	OutboxID          *int64
	GuardianProfileID *int64
	AccountID         *int64
	RecipientEmail    *string
	Reachability      string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Intent struct {
	ID             int64
	TenantID       int64
	OrganizationID int64
	Transport      Transport
	Template       string
	// Security marks sign-in codes, password resets and staff invitations: they
	// are claimed first and the suppression list never holds them back.
	Security bool
	// UseFallback sends a security mail over the fallback transport: the
	// provider reported that it could not deliver it (#5933).
	UseFallback bool
	// DeliveryToken names this mail in its Message-ID and X-Moto-Outbox-Id
	// header. Recipients see it, so it reveals nothing.
	DeliveryToken     string
	IdempotencyKey    *string
	RelatedEntityType *string
	RelatedEntityID   *int64
	Recipient         json.RawMessage
	Payload           json.RawMessage
	Status            string
	Attempts          int
	NextRetryAt       time.Time
	LeaseToken        *string
	LeaseExpiresAt    *time.Time
	ProviderResult    json.RawMessage
	LastError         *string
	SentAt            *time.Time
	DeadLetterAt      *time.Time
	CancelledAt       *time.Time
	// DeliveryState is what the mail provider reported after acceptance
	// (EmailDeliveryState); empty while nothing was reported.
	DeliveryState   string
	DeliveryStateAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Enqueued struct {
	ID        int64
	Duplicate bool
	Conflict  bool
}

type FinalizeResult struct {
	Finalized bool
	State     string
}

type Observation struct {
	Operation string
	Transport string
	Template  string
	Duration  time.Duration
	Count     int
	Err       error
}
