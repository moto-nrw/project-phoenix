package parentbilling

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The invoices and amounts of a family (spec #4611 user story 81, #4691).
// The Abrechnung sees per family every issued document of the Träger, who
// pays each invoice and what is still open. moto does not match payments
// (out of scope): open is what the invoices ask for after their
// cancellations and credits, with no regard to money that came in.
//
// A family is the recipient of the documents: every document addressed to
// the same address, compared without case, like the inbox of the parents
// portal (ADR 0126) and the payer of ADR 0088 section 7. Its ID is the ID of
// its first document, which never changes because issued documents never
// change. The payer choice of #4689 replaces the address as the key in one
// place, the store's grouping.

// Who pays an invoice, from its positions.
const (
	// PaymentFamily is an invoice the family pays itself.
	PaymentFamily = "family"
	// PaymentCostBearer is an invoice whose contribution a cost bearer
	// takes over (#5627); the family pays what is left, often nothing.
	PaymentCostBearer = "cost_bearer"
	// PaymentExempt is an invoice the exemption for families receiving
	// benefits sets to nothing.
	PaymentExempt = "exempt"
)

// States of an invoice in the view of a family.
const (
	// InvoiceOpen asks for its whole amount.
	InvoiceOpen = "open"
	// InvoicePartlyCredited asks for what its credits leave.
	InvoicePartlyCredited = "partly_credited"
	// InvoiceCredited is credited in full and asks for nothing.
	InvoiceCredited = "credited"
	// InvoiceCancelled is cancelled and asks for nothing.
	InvoiceCancelled = "cancelled"
	// InvoiceNothingDue never asked the family for money: a cost bearer
	// pays it or the exemption freed it.
	InvoiceNothingDue = "nothing_due"
)

// ErrFamilyNotFound reports a family that has no document of the Träger of
// the session.
var ErrFamilyNotFound = errors.New("no document of the Träger names this family")

// BillingAmounts are the sums of a family in whole cent, all positive:
// what its invoices asked for, what the cancellations took back, what the
// credits took back, and what is still open.
type BillingAmounts struct {
	IssuedCents    int64 `json:"issued_cents"`
	CancelledCents int64 `json:"cancelled_cents"`
	CreditedCents  int64 `json:"credited_cents"`
	OpenCents      int64 `json:"open_cents"`
}

// BillingPayment is who pays an invoice: the family, a cost bearer by the
// name the invoice copied, or no one because of the exemption.
type BillingPayment struct {
	Kind           string `json:"kind"`
	CostBearerName string `json:"cost_bearer_name"`
}

// BillingFamily is one family as the list shows it: the recipient of its
// latest document, the debtor numbers its documents carry, who pays its
// invoices, and its sums.
type BillingFamily struct {
	ID                 int64            `json:"id"`
	RecipientFirstName string           `json:"recipient_first_name"`
	RecipientLastName  string           `json:"recipient_last_name"`
	RecipientEmail     string           `json:"recipient_email"`
	DebtorNumbers      []string         `json:"debtor_numbers"`
	Payments           []BillingPayment `json:"payments"`
	InvoiceCount       int              `json:"invoice_count"`
	OpenInvoiceCount   int              `json:"open_invoice_count"`
	DocumentCount      int              `json:"document_count"`
	LastIssuedOn       calendar.Date    `json:"last_issued_on"`
	BillingAmounts
}

// BillingFamilyDocument is one issued document of a family: an invoice, a
// cancellation or a credit, with the period of its run and the first names
// of its children. A cancellation or credit names its invoice in
// ReferenceDocumentID and ReferenceNumber and carries its Reason. An
// invoice carries who pays it, its state and what it still asks for.
//
// The recipient, the family key and the facts behind the payment come from
// the store; the family shows the recipient once and never sends the rest.
type BillingFamilyDocument struct {
	ID                  int64           `json:"id"`
	RunID               int64           `json:"run_id"`
	Kind                string          `json:"kind"`
	Number              string          `json:"number"`
	IssuedOn            calendar.Date   `json:"issued_on"`
	PeriodName          string          `json:"period_name"`
	ServiceFrom         calendar.Date   `json:"service_from"`
	ServiceUntil        calendar.Date   `json:"service_until"`
	ReferenceDocumentID *int64          `json:"reference_document_id"`
	ReferenceNumber     string          `json:"reference_number"`
	Reason              string          `json:"reason"`
	ChildNames          []string        `json:"child_names"`
	DebtorNumber        string          `json:"debtor_number"`
	TotalCents          int64           `json:"total_cents"`
	Payment             *BillingPayment `json:"payment"`
	Status              string          `json:"status"`
	OpenCents           int64           `json:"open_cents"`

	RecipientFirstName string   `json:"-"`
	RecipientLastName  string   `json:"-"`
	RecipientEmail     string   `json:"-"`
	FamilyKey          string   `json:"-"`
	CostBearerNames    []string `json:"-"`
	Exempt             bool     `json:"-"`
}

// BillingFamilyDetail is a family with all its documents, the newest first.
type BillingFamilyDetail struct {
	BillingFamily
	Documents []BillingFamilyDocument `json:"documents"`
}

// BillingFamilies lets the Abrechnung of a Träger see the documents and
// amounts of each family. Every operation names the person of the session
// and runs in the Träger transaction of its Träger, which the caller opens;
// every read is recorded in the trail of each run whose documents it shows.
type BillingFamilies interface {
	// ListBillingFamilies lists the families with an issued document, by
	// name.
	ListBillingFamilies(ctx context.Context, reviewer HolidayReviewer) ([]BillingFamily, error)
	// BillingFamily reads one family by its ID with every document. An ID
	// that is no document of the Träger is ErrFamilyNotFound.
	BillingFamily(ctx context.Context, reviewer HolidayReviewer, familyID int64) (BillingFamilyDetail, error)
}
