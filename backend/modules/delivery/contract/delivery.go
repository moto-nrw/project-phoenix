// Package contract publishes Delivery's native intent and provider values.
package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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

// DispatchOutcomeUnknown is the last error of a dead letter whose worker
// stopped during the provider call: the provider may have accepted it, so
// it is neither sent nor failed for sure.
const DispatchOutcomeUnknown = "delivery outcome unknown: worker stopped during provider call"

type RelatedEntity struct {
	Type string
	ID   int64
}

type EmailRecipient struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

type EmailIntent struct {
	TenantID int64
	// OrganizationID names a mail of a Träger instead of a school (ADR
	// 0110). It requires a zero TenantID and the caller's Träger transaction
	// of that Träger.
	OrganizationID int64
	Template       string
	Recipient      EmailRecipient
	Payload        json.RawMessage
	IdempotencyKey string
	Related        RelatedEntity
	DeliverAfter   time.Time
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
	Deferred       bool      `json:"deferred,omitempty"`
	SubscriptionID int64     `json:"subscription_id"`
	AccountID      int64     `json:"account_id"`
	Endpoint       string    `json:"endpoint"`
	P256DH         string    `json:"p256dh"`
	Auth           string    `json:"auth"`
	Portal         string    `json:"portal"`
	UpdatedAt      time.Time `json:"updated_at"`
	AudienceScope  string    `json:"audience_scope,omitempty"`
	StudentIDs     []int64   `json:"student_ids,omitempty"`
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

// ValidatePushEndpoint restricts provider requests to browser-managed push
// services. It is applied again by the worker because persisted snapshots may
// predate subscription validation.
func ValidatePushEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint must be a valid URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Hostname() == "" {
		return errors.New("endpoint must be an https URL")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return errors.New("endpoint must not contain user information or a fragment")
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return errors.New("endpoint must use the default https port")
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "fcm.googleapis.com" || host == "updates.push.services.mozilla.com" || host == "web.push.apple.com" {
		return nil
	}
	if strings.HasSuffix(host, ".notify.windows.com") && host != "notify.windows.com" {
		return nil
	}
	return errors.New("endpoint host is not a trusted push service")
}

type Enqueued struct {
	ID        int64
	Transport Transport
	Duplicate bool
}

var (
	ErrCancelled           = errors.New("delivery: provider cancelled the intent")
	ErrDeferred            = errors.New("delivery: provider deferred the intent")
	ErrIdempotencyConflict = errors.New("delivery: idempotency key reused with different intent")
)

type Status struct {
	IdempotencyKey *string
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
