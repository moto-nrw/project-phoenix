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
	// Security marks a sign-in code, password reset or staff invitation: it is
	// sent before other mail and the suppression list never holds it back.
	Security bool
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
	// Outcome is empty before a mail exists; AddressMark and Stopped can be set
	// without one. RecipientEmail is the person's current address, where another
	// mail goes, and AddressMark belongs to it. MailedTo is where this mail went:
	// Outcome belongs to that address and to no other.
	Outcome     EmailOutcome     `json:"outcome,omitempty"`
	MailedTo    *string          `json:"mailed_to,omitempty"`
	AddressMark EmailAddressMark `json:"address_mark,omitempty"`
	Stopped     bool             `json:"stopped,omitempty"`
	// StopShared says another holder of the address carries no stop, so mail to the
	// address still goes out.
	StopShared bool `json:"stop_shared,omitempty"`
	// CurrentAddressRefused says an earlier mail of this entity to RecipientEmail
	// ended in an unsubscribe or a complaint. Only the provider's current block then
	// says whether a new mail may go there, whatever the last mail's result is.
	CurrentAddressRefused bool `json:"-"`
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

// PushSubscriptionExpired reports a push service status that retires the
// subscription itself (RFC 8030 section 7.3, autopush 404/410).
func PushSubscriptionExpired(status int) bool {
	return status == 404 || status == 410
}

// PushRejectedPermanently reports a push service status that a retry of the
// same request cannot change: a malformed request or invalid subscription keys
// (400), missing or invalid VAPID authentication (401, 403; RFC 8292 sections 2
// and 4.2) and an oversized payload (413). 403 also answers an invalid JWT
// signature or claims, which affects every subscription, so it must not prune
// the subscription the way an expiry does.
func PushRejectedPermanently(status int) bool {
	switch status {
	case 400, 401, 403, 413:
		return true
	default:
		return false
	}
}

type Enqueued struct {
	ID        int64
	Transport Transport
	Duplicate bool
}

var (
	ErrCancelled = errors.New("delivery: provider cancelled the intent")
	ErrDeferred  = errors.New("delivery: provider deferred the intent")
	// ErrThrottled refines ErrDeferred: the mail server asked to send later
	// (#6737). The worker waits longer the older the mail is.
	ErrThrottled = errors.New("delivery: provider throttled the intent")
	// ErrPermanent marks a rejection that no retry can change. The intent
	// dead-letters on this attempt, so the failure is still reported.
	ErrPermanent = errors.New("delivery: provider rejected the intent permanently")
	// ErrRecipientRejected refines ErrPermanent: the server refused this
	// address, so it goes on the school's suppression list.
	ErrRecipientRejected   = errors.New("delivery: provider rejected the recipient permanently")
	ErrIdempotencyConflict = errors.New("delivery: idempotency key reused with different intent")
)

// What a webhook delivery can fail on. Each decides the sender's next step: a
// refusal is final, ErrEmailWebhookNotConfigured and a store failure are worth
// a retry.
var (
	ErrEmailWebhookUnknownProvider = errors.New("delivery: unknown email webhook provider")
	ErrEmailWebhookNotConfigured   = errors.New("delivery: email webhook has no signing secret")
	ErrEmailWebhookSignature       = errors.New("delivery: email webhook signature rejected")
	ErrEmailWebhookPayload         = errors.New("delivery: email webhook payload rejected")
)

// ErrEmailSuppressionNeedsReview reports the block of a complaint: it stays
// until the provider's support reviewed it, and nobody at moto lifts it.
var ErrEmailSuppressionNeedsReview = errors.New("delivery: suppression after a complaint needs a review")

// ErrEmailSuppressionNotFound reports an address with no connection to the
// school: it is not on the school's list, and with a provider the school has no
// mail to it either. Nothing changed, also at the provider, whose block applies
// to the whole team. A caller that only wants the address free can treat it as
// done.
var ErrEmailSuppressionNotFound = errors.New("delivery: the school has no suppression for this address")

// ErrEmailSuppressionStaysAtProvider reports a manual or unsubscribe block at
// the provider. It is someone's decision, so the address stays blocked and
// nothing changed, neither at the provider nor on the school's list.
var ErrEmailSuppressionStaysAtProvider = errors.New("delivery: the provider keeps a manual or unsubscribe block")

type Status struct {
	IdempotencyKey *string
	ID             int64
	// Kind is the template of the intent; RelatedID the entity it was
	// enqueued for, nil without one (#6976).
	Kind      string
	RelatedID *int64
	// CreatedAt is when the intent was enqueued (#6976).
	CreatedAt      time.Time
	Transport      Transport
	State          State
	Attempts       int
	LastError      *string
	ProviderResult *ProviderResult
	NextAttemptAt  time.Time
	SentAt         *time.Time
	DeadLetterAt   *time.Time
	CancelledAt    *time.Time
	// DeliveryState is what the mail provider reported after it accepted
	// the mail (#5932): one of the DeliveryState values, empty while
	// nothing was reported. A sent mail can still bounce.
	DeliveryStateAt *time.Time
	DeliveryState   string
	// Outcome is set for a mail; a push has none.
	Outcome EmailOutcome
}

// The provider outcomes of a sent mail (#5932). DeliveryFailed is a bounce
// for good, DeliverySuppressed a mail the provider withheld for a known bad
// address; neither reached the recipient.
const (
	DeliveryDelayed      = "delayed"
	DeliveryDelivered    = "delivered"
	DeliverySuppressed   = "suppressed"
	DeliveryUnsubscribed = "unsubscribed"
	DeliveryFailed       = "failed"
	DeliveryComplained   = "complained"
)

type Observation struct {
	Operation string
	Transport Transport
	Template  string
	Duration  time.Duration
	Count     int
	Err       error
}

// parentMailTemplates are the mails to parents, the only mail the suppression list
// and a school's stop hold back (#5932, #5934). A Träger's care contract counts:
// it goes to a family of one of its schools. A Träger's billing letter does not,
// and neither does mail to staff. An unlisted template goes out.
var parentMailTemplates = map[string]bool{
	"guardian_invitation":                true,
	"guardian_welcome":                   true,
	"parent_announcement":                true,
	"parent_message":                     true,
	"enrollment_submitted":               true,
	"enrollment_approved":                true,
	"enrollment_waitlisted":              true,
	"enrollment_rejected":                true,
	"enrollment_rollover_opt_in":         true,
	"enrollment_rollover_opt_out":        true,
	"enrollment_change_request_question": true,
	"enrollment_change_request_approved": true,
	"enrollment_change_request_rejected": true,
	"holiday_registration_received":      true,
	"holiday_place_granted":              true,
	"holiday_closure_notice":             true,
	"holiday_registration_accepted":      true,
	"holiday_parent_account_offer":       true,
	"appointment_published":              true,
	"appointment_updated":                true,
	"appointment_cancelled":              true,
	"appointment_reminder":               true,
	"care_contract":                      true,
	"care_contract_notice":               true,
}

// IsParentMail reports whether a mail template goes to parents.
func IsParentMail(template string) bool { return parentMailTemplates[template] }

// HolidayClosureMailRecovery exposes only the key and whether a retry needs outcome clarification.
type HolidayClosureMailRecovery struct {
	IdempotencyKey string
	Unknown        bool
}
