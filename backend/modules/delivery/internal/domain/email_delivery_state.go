package domain

import (
	"errors"
	"regexp"
	"slices"
	"time"
)

var (
	// ErrProviderNotFound reports something the provider does not (or no longer)
	// keep, such as a mail after its retention.
	ErrProviderNotFound = errors.New("delivery: provider does not know the message")
	// ErrProviderRateLimited asks to stop calling the provider for now.
	ErrProviderRateLimited = errors.New("delivery: provider rate limit reached")
	// ErrSuppressionNeedsReview reports a complaint's block: only the provider's
	// support can lift it after a review.
	ErrSuppressionNeedsReview = errors.New("delivery: suppression after a complaint needs a review")
	// ErrSuppressionStaysAtProvider reports a block someone set on purpose, by
	// hand or through an unsubscribe. Nothing was removed.
	ErrSuppressionStaysAtProvider = errors.New("delivery: the provider keeps a manual or unsubscribe block")
)

// EmailDeliveryState says what happened to a mail after the mail server
// accepted it. The empty value means nothing has been reported yet.
type EmailDeliveryState string

const (
	EmailDeliveryDelayed    EmailDeliveryState = "delayed"
	EmailDeliveryDelivered  EmailDeliveryState = "delivered"
	EmailDeliverySuppressed EmailDeliveryState = "suppressed"
	EmailDeliveryFailed     EmailDeliveryState = "failed"
	EmailDeliveryComplained EmailDeliveryState = "complained"
)

// emailDeliveryStateOrder ranks the states by how much they say about the
// address. Events can repeat and arrive in any order, and a complaint or a
// bounce can arrive after delivery. The highest state of a mail's events is
// therefore its state, whatever order they came in.
var emailDeliveryStateOrder = []EmailDeliveryState{
	EmailDeliveryDelayed,
	EmailDeliveryDelivered,
	EmailDeliverySuppressed,
	EmailDeliveryFailed,
	EmailDeliveryComplained,
}

// NextEmailDeliveryState is the state of a mail after one more event.
func NextEmailDeliveryState(current, event EmailDeliveryState) EmailDeliveryState {
	if slices.Index(emailDeliveryStateOrder, event) > slices.Index(emailDeliveryStateOrder, current) {
		return event
	}
	return current
}

// BlocksAddress reports whether the state puts the address on the school's
// suppression list. A provider that holds a mail back knows the address from
// another school or an older block; only the school's list can tell the school
// why its mail did not leave, so the address goes there too.
func (s EmailDeliveryState) BlocksAddress() bool {
	return s == EmailDeliveryFailed || s == EmailDeliveryComplained || s == EmailDeliverySuppressed
}

// EmailSuppressionReason is why a school's list holds an address.
type EmailSuppressionReason string

const (
	EmailSuppressionFailed     EmailSuppressionReason = "failed"
	EmailSuppressionComplained EmailSuppressionReason = "complained"
	EmailSuppressionSchoolStop EmailSuppressionReason = "school_stop"
)

// SuppressionReason is the reason the state leaves on the school's list.
func (s EmailDeliveryState) SuppressionReason() EmailSuppressionReason {
	if s == EmailDeliveryComplained {
		return EmailSuppressionComplained
	}
	return EmailSuppressionFailed
}

// EmailDeliveryEvent is one reported event of a mail. The mail is named by
// the token of its X-Moto-Outbox-Id header, its Message-ID or the ID the
// provider gave it; an event without any match changes nothing.
type EmailDeliveryEvent struct {
	Token             string
	MessageID         string
	ProviderMessageID string
	Provider          string
	ProviderEventID   string
	State             EmailDeliveryState
	OccurredAt        time.Time
	Diagnostic        string
	// KeepsAddress marks a failure that says nothing about the address: the
	// provider's IPs on a blocklist or moto's own mail rejected by a filter. The
	// address stays off the school's list.
	KeepsAddress bool
	// Reroute asks to send a security mail again over the fallback transport.
	Reroute bool
}

// EmailDeliveryEventResult reports what recording an event did.
type EmailDeliveryEventResult struct {
	Matched  bool
	Recorded bool
	State    EmailDeliveryState
	// Rerouted reports that the event put a security mail back in the queue for
	// the fallback transport.
	Rerouted bool
}

// ReportedEmailEvent is one verified webhook delivery. An event moto has no
// state for carries no Event: it is acknowledged, never an error.
type ReportedEmailEvent struct {
	Name  string
	Known bool
	Event *EmailDeliveryEvent
}

// ProviderMailQuery selects open mails: accepted in the window and not asked
// about since CheckedBefore.
type ProviderMailQuery struct {
	SentAfter     time.Time
	SentBefore    time.Time
	CheckedBefore time.Time
	Limit         int
}

// ProviderMail is an accepted mail whose outcome is still open.
type ProviderMail struct {
	ID                int64
	ProviderMessageID string
	State             EmailDeliveryState
	SentAt            time.Time
	// CheckedAt is when the reconciliation last asked about the mail, if ever.
	CheckedAt *time.Time
}

var diagnosticAddress = regexp.MustCompile(`(?:"[^"]*"|[^\s<>,;:()\[\]"]+)@[^\s<>,;:()\[\]"]+`)

// RedactDiagnostic removes addresses from a server's answer before it is
// stored: servers echo the rejected mailbox in their reply.
func RedactDiagnostic(text string) string {
	return diagnosticAddress.ReplaceAllString(text, "[address]")
}
