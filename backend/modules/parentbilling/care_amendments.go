package parentbilling

import (
	"context"
	"errors"
	"time"
)

// Changes of one invoice in the review of a monthly run (#6197, ADR 0119
// addendum 2026-11-08 "Änderungen je Rechnung", decision A of the SJR
// alignment). Before the release the Abrechnung may change the amount, the
// recipient, the postal address and the positions of a single invoice of
// the run. The preview stays computed: a change is an entry the review
// appends, never an edited draft. The amount and the positions change by a
// correction line with a reason; the computed positions, reductions and
// shares stay as they are. A recipient or address change holds for this
// invoice of this run only, never for the family (#4689 keeps that).
//
// Invariants:
//   - Entries are only ever appended (parent_billing.care_run_amendments):
//     kind, old and new value, reason, account and time. A revocation is a
//     new entry; the database refuses an update and any entry of a released
//     run.
//   - The total of an invoice is the sum of its lines; a correction line
//     never takes it below zero, and a change that would is refused.
//   - Every entry is part of the fingerprint of the review: the release
//     refuses a state that changed since the reviewer saw it, and a change
//     names the fingerprint it was made on, so two changes at once cannot
//     overwrite each other unseen.
//   - An entry whose basis changed (the total it corrected, a position it
//     removed) is not applied but shown as a conflict; the release refuses
//     until the Abrechnung revokes it.
//   - After the release nothing changes: a correction is a Storno and a new
//     invoice (#4690). The new invoice of the month takes the changes of the
//     cancelled one over as carried entries, which the run may revoke.
//   - Which kinds of change a Träger allows is its setting; the defaults
//     allow all four (the customer has not decided yet, SJR item 2).

// Kinds of a change of one invoice.
const (
	// AmendAmount sets the total of the invoice by a correction line of the
	// difference.
	AmendAmount = "amount"
	// AmendAddPosition adds a position with its own label and amount.
	AmendAddPosition = "add_position"
	// AmendRemovePosition takes a computed position off by a correction line
	// of minus what it costs the family.
	AmendRemovePosition = "remove_position"
	// AmendRecipient changes the name and mail address the invoice goes to.
	AmendRecipient = "recipient"
	// AmendAddress changes the postal address of the invoice.
	AmendAddress = "address"
	// AmendRevoke takes an earlier change, or a carried one, back.
	AmendRevoke = "revoke"
)

// Conflicts of a change whose basis changed since it was made.
const (
	// AmendConflictBasis reports an amount change of a total that is no
	// longer the total it corrected.
	AmendConflictBasis = "basis_changed"
	// AmendConflictPosition reports the removal of a position the invoice no
	// longer has.
	AmendConflictPosition = "position_missing"
	// AmendConflictNegative reports a correction line that would take the
	// invoice below zero.
	AmendConflictNegative = "negative_total"
)

// Limits of a change.
const (
	// MaxAmendmentReasonLength is the longest reason of a change.
	MaxAmendmentReasonLength = 500
	// MaxAmendmentLabelLength is the longest label of an added position.
	MaxAmendmentLabelLength = 200
)

var (
	// ErrCareAmendmentConflict reports a change of an invoice or position
	// the preview does not have, a revocation of an entry that is not in
	// force, or a change that would take the invoice below zero.
	ErrCareAmendmentConflict = errors.New("the preview has no such invoice, position or change, or the change does not fit")
	// ErrCareAmendmentDisabled reports a kind of change the Träger does not
	// allow (its setting).
	ErrCareAmendmentDisabled = errors.New("the Träger does not allow this kind of change")
	// ErrCareAmendmentsUnresolved reports a release while a change of an
	// invoice it issues is in conflict.
	ErrCareAmendmentsUnresolved = errors.New("a change of an invoice is in conflict and must be revoked first")
)

// CareAmendable says which kinds of change the Träger allows in the review
// (settings parent_billing.care_review_*). It is part of the review.
type CareAmendable struct {
	Amount    bool `json:"amount"`
	Recipient bool `json:"recipient"`
	Address   bool `json:"address"`
	Positions bool `json:"positions"`
}

// Allows reports whether the Träger allows a kind of change; a revocation
// is always allowed.
func (a CareAmendable) Allows(kind string) bool {
	switch kind {
	case AmendAmount:
		return a.Amount
	case AmendRecipient:
		return a.Recipient
	case AmendAddress:
		return a.Address
	case AmendAddPosition, AmendRemovePosition:
		return a.Positions
	case AmendRevoke:
		return true
	}
	return false
}

// CareAmendmentSettings reads which kinds of change a Träger allows. The
// root binds it to the Träger level of the settings.
type CareAmendmentSettings interface {
	CareAmendable(ctx context.Context, organizationID int64) (CareAmendable, error)
}

// CareRecipientName is the name and mail address of a recipient.
type CareRecipientName struct {
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	SecondName string `json:"second_name"`
	Email      string `json:"email"`
}

// CareAmendmentValue is one side of a change: the total of an amount
// change, the label and amount of a position, a recipient or an address.
type CareAmendmentValue struct {
	TotalCents  *int64             `json:"total_cents,omitempty"`
	Label       string             `json:"label,omitempty"`
	AmountCents *int64             `json:"amount_cents,omitempty"`
	Recipient   *CareRecipientName `json:"recipient,omitempty"`
	Address     *PostalAddress     `json:"address,omitempty"`
}

// CareRevocation is the entry that took a change back.
type CareRevocation struct {
	Reason    string    `json:"reason"`
	AccountID *int64    `json:"account_id"`
	RevokedAt time.Time `json:"revoked_at"`
}

// CareAmendment is a change of an invoice as its history shows it. On a
// preview ID names the entry of the run, or the issued correction it was
// carried from (Carried); on an issued invoice the stored correction.
// AmountCents is the correction line it adds to the total, 0 for a
// recipient or address change. A revoked change or one in conflict adds
// nothing.
type CareAmendment struct {
	ID          int64              `json:"id"`
	Kind        string             `json:"kind"`
	Label       string             `json:"label"`
	AmountCents int64              `json:"amount_cents"`
	PositionKey string             `json:"position_key,omitempty"`
	Before      CareAmendmentValue `json:"before"`
	After       CareAmendmentValue `json:"after"`
	Reason      string             `json:"reason"`
	AccountID   *int64             `json:"account_id"`
	AmendedAt   time.Time          `json:"amended_at"`
	// Carried names the cancelled invoice the change came over from.
	Carried           bool   `json:"carried"`
	CarriedFromNumber string `json:"carried_from_number,omitempty"`
	// Revocation is set once the change is taken back.
	Revocation *CareRevocation `json:"revocation,omitempty"`
	// Conflict names why a change of a preview is not applied.
	Conflict string `json:"conflict,omitempty"`
}

// Applied reports whether the change shapes the invoice.
func (a CareAmendment) Applied() bool { return a.Revocation == nil && a.Conflict == "" }

// CareAmendmentInput is one change of an invoice of a preview. ContractID
// and Service name the invoice; Fingerprint is the review the change was
// made on. Kind decides the other fields: TotalCents for an amount change,
// Label and AmountCents for an added position, PositionKey for a removed
// one, Recipient or Address, and RevokesID (with RevokesCarried for a
// carried change) for a revocation. Reason is always required.
type CareAmendmentInput struct {
	ContractID     int64
	Service        string
	Kind           string
	Fingerprint    string
	Reason         string
	TotalCents     *int64
	Label          string
	AmountCents    *int64
	PositionKey    string
	Recipient      *CareRecipientName
	Address        *PostalAddress
	RevokesID      int64
	RevokesCarried bool
}

// CareInvoiceKey names an invoice of a month: its contract and its service
// (ServiceCare, or ServiceMeal for a Mensa invoice of its own).
type CareInvoiceKey struct {
	ContractID int64
	Service    string
}

// CareAmendmentEntry is an entry of the review as it is stored: a change,
// or the revocation of the change RevokesID names (of the run) or the
// carried correction RevokesCorrectionID names.
type CareAmendmentEntry struct {
	ID                  int64
	Invoice             CareInvoiceKey
	Kind                string
	Label               string
	AmountCents         int64
	PositionKey         string
	Before              CareAmendmentValue
	After               CareAmendmentValue
	Reason              string
	RevokesID           *int64
	RevokesCorrectionID *int64
	AccountID           *int64
	AmendedAt           time.Time
}
