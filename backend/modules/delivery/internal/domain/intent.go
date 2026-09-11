package domain

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrCancelled           = errors.New("delivery provider cancelled the intent")
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

type RelatedEntity struct {
	Type string
	ID   int64
}

type EnqueueInput struct {
	TenantID       int64
	Transport      Transport
	Template       string
	IdempotencyKey string
	Related        RelatedEntity
	Recipient      json.RawMessage
	Payload        json.RawMessage
}

type ProviderResult struct {
	MessageID  string          `json:"message_id,omitempty"`
	StatusCode int             `json:"status_code,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
}

type WorkerStats struct {
	Claimed      int
	Sent         int
	Cancelled    int
	Retried      int
	DeadLettered int
	LeaseLost    int
}

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

type EmailDeliveryStatus struct {
	DeliveryID        int64
	GuardianProfileID *int64
	AccountID         *int64
	FirstName         string
	LastName          string
	RecipientEmail    *string
	Reachability      string
	EmailStatus       string
	LastError         *string
	SentAt            *time.Time
	Attempts          int
}

type GuardianDisplay struct {
	GuardianProfileID int64
	FirstName         string
	LastName          string
}

type Intent struct {
	ID                int64
	TenantID          int64
	Transport         Transport
	Template          string
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
	CreatedAt         time.Time
	UpdatedAt         time.Time
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
