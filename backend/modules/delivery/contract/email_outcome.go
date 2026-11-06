package contract

import (
	"strings"
	"time"
)

// EmailOutcome is what a school may be told about one mail. A mail the mail
// server accepted is EmailAccepted until a provider reports more; only a
// provider's report makes it EmailDelivered. Plain SMTP and the fallback mailbox
// report nothing, so their mails stay EmailAccepted.
type EmailOutcome string

const (
	EmailQueued        EmailOutcome = "queued"
	EmailAccepted      EmailOutcome = "accepted"
	EmailDelivered     EmailOutcome = "delivered"
	EmailDelayed       EmailOutcome = "delayed"
	EmailUndeliverable EmailOutcome = "undeliverable"
	// EmailBlocked is a mail that never left because the address is blocked.
	EmailBlocked    EmailOutcome = "blocked"
	EmailComplained EmailOutcome = "complained"
	// EmailSendFailed is a mail moto gave up on before a server took it.
	EmailSendFailed EmailOutcome = "send_failed"
	// EmailOutcomeUnknown is a mail whose worker stopped during the provider call.
	EmailOutcomeUnknown EmailOutcome = "unknown"
	EmailCancelled      EmailOutcome = "cancelled"
)

// EmailAddressMark is why a school's list holds an address, and it stays until
// someone at the school checked the address.
type EmailAddressMark string

const (
	// EmailMarkFailed is an address that refused a mail for good.
	EmailMarkFailed EmailAddressMark = "failed"
	// EmailMarkComplained is an address whose owner reported a mail as unwanted.
	// The school cannot lift it.
	EmailMarkComplained EmailAddressMark = "complained"
	// EmailMarkSchoolStop is an address the school itself stopped mail to.
	EmailMarkSchoolStop EmailAddressMark = "school_stop"
)

// EmailSendBlock says why sending again would do nothing or do harm.
type EmailSendBlock string

const (
	// EmailSendBlockAddress: the list holds the mail back.
	EmailSendBlockAddress EmailSendBlock = "address"
	// EmailSendBlockDelayed: the server is still trying, a second mail would
	// only arrive twice.
	EmailSendBlockDelayed EmailSendBlock = "delayed"
)

// EmailOutcomeOf derives the outcome from the outbox state of a mail, the state
// its events led to, its last error and the mark of its address.
func EmailOutcomeOf(status string, deliveryState, lastError *string, mark EmailAddressMark) EmailOutcome {
	if deliveryState != nil {
		switch *deliveryState {
		case "delivered":
			return EmailDelivered
		case "delayed":
			return EmailDelayed
		case "failed":
			return EmailUndeliverable
		case "suppressed":
			return EmailBlocked
		case "complained":
			return EmailComplained
		}
	}
	switch State(status) {
	case StateSent:
		return EmailAccepted
	case StateDeadLetter:
		if lastError != nil && *lastError == DispatchOutcomeUnknown {
			return EmailOutcomeUnknown
		}
		return EmailSendFailed
	case StateCancelled:
		// The list held the mail back, and its own event is still on the way.
		if mark != "" {
			return EmailBlocked
		}
		return EmailCancelled
	default:
		return EmailQueued
	}
}

// EmailSendBlockOf decides whether a new mail is pointless. The mark only holds
// a new mail back when the mail is one the list applies to; the caller knows.
func EmailSendBlockOf(mark EmailAddressMark, latest EmailOutcome) EmailSendBlock {
	if mark != "" {
		return EmailSendBlockAddress
	}
	if latest == EmailDelayed {
		return EmailSendBlockDelayed
	}
	return ""
}

// EmailOutcomeReport is the outcome of one mail. It carries no address: a
// consumer that shows one owns it.
type EmailOutcomeReport struct {
	MailID      int64
	Kind        string
	RelatedType string
	RelatedID   int64
	Address     string
	Outcome     EmailOutcome
	Mark        EmailAddressMark
	OutcomeAt   *time.Time
	Attempts    int
}

// EmailOutcomeQuery names the mails of some related entities, of the given
// kinds; no kind means every kind.
type EmailOutcomeQuery struct {
	RelatedType string
	RelatedIDs  []int64
	Kinds       []string
}

// NormalizedEmailAddress is the form a school's list keys an address by.
func NormalizedEmailAddress(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}
