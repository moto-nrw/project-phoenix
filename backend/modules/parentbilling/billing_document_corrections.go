package parentbilling

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Storno, Gutschrift and Neuausstellung of an issued invoice (spec #4611
// user story 72, #4690, ADR 0119 addendum). An issued invoice never changes.
// The Abrechnung cancels it with a document of its own that negates every
// position, or credits part of it with a document of its own, both with a
// reason and a number from the same number range as the invoices. A
// cancelled invoice no longer bills its registration: the next run of the
// period issues it again with a new number, and that invoice names the one
// it replaces. Each document gets its letter like an invoice.

// Kinds of an issued document.
const (
	DocumentInvoice      = "invoice"
	DocumentCancellation = "cancellation"
	DocumentCredit       = "credit"
)

// LineCredit is the kind of the position of a credit; its Note is the
// reason.
const LineCredit = "credit"

// MaxDocumentReasonLength is the longest reason of a cancellation or a
// credit.
const MaxDocumentReasonLength = 500

var (
	// ErrBillingDocumentNotFound reports an invoice that is not part of the
	// run, or a document that is no invoice.
	ErrBillingDocumentNotFound = errors.New("invoice not found in the billing run")
	// ErrInvoiceCancelled reports a cancellation or credit of an invoice
	// that is already cancelled.
	ErrInvoiceCancelled = errors.New("the invoice is already cancelled")
	// ErrInvoiceCredited reports a cancellation of an invoice that already
	// has a credit; the rest is credited instead.
	ErrInvoiceCredited = errors.New("the invoice already has a credit")
)

// BillingCancellationInput is the reason of a cancellation. Reissue says
// that a new invoice follows; the letter of the cancellation tells the
// family so.
type BillingCancellationInput struct {
	Reason  string
	Reissue bool
}

// BillingCreditInput credits AmountCents of one child of an invoice, with
// its reason.
type BillingCreditInput struct {
	ChildID     int64
	AmountCents int64
	Reason      string
}

// BillingDocumentLink is a document of the history of an invoice: its
// cancellation, its credits and the invoice that replaces it, which may be
// part of another run.
type BillingDocumentLink struct {
	ID         int64         `json:"id"`
	RunID      int64         `json:"run_id"`
	Kind       string        `json:"kind"`
	Number     string        `json:"number"`
	IssuedOn   calendar.Date `json:"issued_on"`
	TotalCents int64         `json:"total_cents"`
	Reason     string        `json:"reason"`
}
