package domain

import (
	"regexp"
	"slices"
	"time"
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
// suppression list.
func (s EmailDeliveryState) BlocksAddress() bool {
	return s == EmailDeliveryFailed || s == EmailDeliveryComplained
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
}

// EmailDeliveryEventResult reports what recording an event did.
type EmailDeliveryEventResult struct {
	Matched  bool
	Recorded bool
	State    EmailDeliveryState
}

// SuppressionRetention is how long the suppression list keeps an address after
// its last event. A new bounce or complaint starts the period again.
const SuppressionRetention = 365 * 24 * time.Hour

var diagnosticAddress = regexp.MustCompile(`[A-Za-z0-9.!#$%&'*+/=?^_{|}~-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*`)

// RedactDiagnostic removes addresses from a server's answer before it is
// stored: servers echo the rejected mailbox in their reply.
func RedactDiagnostic(text string) string {
	return diagnosticAddress.ReplaceAllString(text, "[address]")
}
