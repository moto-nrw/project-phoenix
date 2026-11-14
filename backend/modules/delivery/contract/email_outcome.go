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
	// EmailBlocked is a mail that never left because the school's list held it.
	EmailBlocked EmailOutcome = "blocked"
	// EmailProviderBlocked is a mail the provider withheld. Its block may come from
	// any school of the team, and only EmailProviderBlock says whether it still holds.
	EmailProviderBlocked EmailOutcome = "provider_blocked"
	// EmailHeldByStop is a mail that stayed back because the school stopped mail to
	// the person. The address itself is fine.
	EmailHeldByStop EmailOutcome = "held_by_stop"
	// EmailHeldByDeparture is a mailing that stayed back because no child of the
	// person is in care at the school any more (#5960). The address itself is fine.
	EmailHeldByDeparture EmailOutcome = "held_by_departure"
	EmailComplained      EmailOutcome = "complained"
	// EmailUnsubscribed is a person who unsubscribed. The address itself is fine.
	EmailUnsubscribed EmailOutcome = "unsubscribed"
	// EmailOverdue is a delayed mail past EmailDelayLimit without a final event.
	EmailOverdue EmailOutcome = "overdue"
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
)

// The reasons a held mail ends with. They are its last error, so a mail still says
// why it stayed back after its event went missing or the hold ended.
const (
	MailHeldByList = "Diese Adresse ist gesperrt, weil eine frühere Mail nicht zugestellt werden konnte."
	MailHeldByStop = "Die Schule hat die Mails an diese Person gestoppt."
	// MailHeldByDeparture ends a mailing to a family whose last child left (#5960).
	MailHeldByDeparture = "Kein Kind dieser Person wird noch an der Schule betreut."
)

// EmailProviderBlock is the provider's block of an address at this moment, read
// exactly for that address. Empty means nobody asked.
type EmailProviderBlock string

const (
	// EmailProviderBlockNone: the provider holds nothing, or there is no provider.
	EmailProviderBlockNone EmailProviderBlock = "none"
	// EmailProviderBlockActive: the provider withholds mail to the address. It does
	// not say why: the reason may belong to another school.
	EmailProviderBlockActive EmailProviderBlock = "active"
	// EmailProviderBlockUnreadable: the state could not be read, so nothing is offered.
	EmailProviderBlockUnreadable EmailProviderBlock = "unreadable"
)

// HeldAtProvider reports an outcome that leaves a block at the provider, so that
// only its current state decides about another mail. It also limits what moto asks:
// the provider is only asked about an address the asking school's own mail ran into.
func (o EmailOutcome) HeldAtProvider() bool {
	return o == EmailProviderBlocked || o == EmailUnsubscribed || o == EmailComplained
}

// AddressesHeldAtProvider names the addresses whose latest mail makes the provider's
// current block worth asking about.
func AddressesHeldAtProvider(latest map[string]EmailOutcomeReport) []string {
	var addresses []string
	for address, report := range latest {
		if report.Outcome.HeldAtProvider() {
			addresses = append(addresses, address)
		}
	}
	return addresses
}

// EmailDelayLimit ends the wait for a delayed mail. Mail servers give up after a
// few days, usually with a bounce (RFC 5321 suggests four to five). If no final
// event came by then, a new mail is allowed again and an alarm goes off.
const EmailDelayLimit = 5 * 24 * time.Hour

// EmailSendBlock says why sending again would do nothing or do harm.
type EmailSendBlock string

const (
	// EmailSendBlockAddress: the list holds the mail back.
	EmailSendBlockAddress EmailSendBlock = "address"
	// EmailSendBlockStopped: the school stopped mail to this person.
	EmailSendBlockStopped EmailSendBlock = "stopped"
	// EmailSendBlockDeparted: no child of the address holders is in care.
	// It only holds distribution mail, never contracts or billing letters.
	EmailSendBlockDeparted EmailSendBlock = "departed"
	// EmailSendBlockProvider: the provider withholds mail to the address now.
	EmailSendBlockProvider EmailSendBlock = "provider"
	// EmailSendBlockUnreadable: the provider's state could not be read.
	EmailSendBlockUnreadable EmailSendBlock = "provider_unknown"
	// EmailSendBlockDelayed: the server is still trying, a second mail would
	// only arrive twice.
	EmailSendBlockDelayed EmailSendBlock = "delayed"
)

// EmailOutcomeOf derives the outcome from the outbox state of a mail, the state
// its events led to, its last error and the mark of its address.
func EmailOutcomeOf(status string, deliveryState, lastError *string, mark EmailAddressMark) EmailOutcome {
	if State(status) == StateCancelled && lastError != nil && *lastError == MailHeldByStop {
		return EmailHeldByStop
	}
	if State(status) == StateCancelled && lastError != nil && *lastError == MailHeldByDeparture {
		return EmailHeldByDeparture
	}
	if deliveryState != nil {
		if outcome, known := reportedOutcome(status, *deliveryState); known {
			return outcome
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
		// The list held the mail back, and its own event is missing.
		if mark != "" || (lastError != nil && *lastError == MailHeldByList) {
			return EmailBlocked
		}
		return EmailCancelled
	default:
		return EmailQueued
	}
}

// reportedOutcome names the state a mail's events led to.
func reportedOutcome(status, deliveryState string) (EmailOutcome, bool) {
	switch deliveryState {
	case "delivered":
		return EmailDelivered, true
	case "delayed":
		return EmailDelayed, true
	case "failed":
		return EmailUndeliverable, true
	case "suppressed":
		if State(status) == StateSent {
			return EmailProviderBlocked, true
		}
		return EmailBlocked, true
	case "unsubscribed":
		return EmailUnsubscribed, true
	case "complained":
		return EmailComplained, true
	}
	return "", false
}

// EmailOutcomeAt lets a delayed mail count as overdue once the wait has ended.
func EmailOutcomeAt(outcome EmailOutcome, stateAt *time.Time, now time.Time) EmailOutcome {
	if outcome == EmailDelayed && stateAt != nil && now.Sub(*stateAt) > EmailDelayLimit {
		return EmailOverdue
	}
	return outcome
}

// EmailSendBlockOf decides whether a new mail is pointless. Stop and mark only hold
// for mail the list and the stop apply to; the caller knows. The provider's block is
// the current one, never the last mail's result.
func EmailSendBlockOf(mark EmailAddressMark, stopped bool, latest EmailOutcome, provider EmailProviderBlock) EmailSendBlock {
	switch {
	case stopped:
		return EmailSendBlockStopped
	case mark != "":
		return EmailSendBlockAddress
	case provider == EmailProviderBlockActive:
		return EmailSendBlockProvider
	case provider == EmailProviderBlockUnreadable:
		return EmailSendBlockUnreadable
	case latest == EmailDelayed:
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
	Stopped     bool
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
