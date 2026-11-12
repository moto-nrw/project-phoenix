package parentbilling

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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

// FamilyContracts resolves existing care contracts for a set of family
// addresses in the caller's Träger transaction. The root binds the port.
type FamilyContracts interface {
	FamiliesWithContracts(ctx context.Context, organizationID int64, emails []string) ([]string, error)
}

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
var ErrFamilyNotFound = errors.New("no document or number of the Träger names this family")

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
	// PaymentMethods are the payment methods of its invoices (#4689), in
	// the order of PaymentMethods.
	PaymentMethods   []string      `json:"payment_methods"`
	InvoiceCount     int           `json:"invoice_count"`
	OpenInvoiceCount int           `json:"open_invoice_count"`
	DocumentCount    int           `json:"document_count"`
	LastIssuedOn     calendar.Date `json:"last_issued_on"`
	// FamilyEmail is the key of the family (FamilyKey); the number of the
	// family is set by it. FamilyDebtorNumber is the number the family has
	// (#6692), empty for none; DebtorNumbers are the numbers its documents
	// copied. HolidayOnly marks a family with no care contract and no
	// monthly invoice. A family without a document has the ID 0.
	FamilyEmail        string `json:"family_email"`
	FamilyDebtorNumber string `json:"family_debtor_number"`
	HolidayOnly        bool   `json:"holiday_only"`
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
	// Service is the holiday care ("holiday") or the monthly care ("care",
	// #4689); the run of a document is a run of its service.
	Service string `json:"service"`
	// RecipientKind and RecipientSecondName say who got the invoice (#4689).
	RecipientKind       string          `json:"recipient_kind"`
	RecipientSecondName string          `json:"recipient_second_name"`
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
	// PaymentMethod and DueOn are how and by when the invoice is paid
	// (#4689); empty and nil on documents issued before.
	PaymentMethod string         `json:"payment_method"`
	DueOn         *calendar.Date `json:"due_on"`

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
	// ListBillingFamilies lists the families with an issued document or a
	// number of their own (#6692), by name; the read of a number is recorded
	// in the family's trail.
	ListBillingFamilies(ctx context.Context, reviewer HolidayReviewer) ([]BillingFamily, error)
	// BillingFamily reads one family by its ID with every document. An ID
	// that is no document of the Träger is ErrFamilyNotFound.
	BillingFamily(ctx context.Context, reviewer HolidayReviewer, familyID int64) (BillingFamilyDetail, error)
	// SetFamilyDebtorNumber sets the number of a family by its key, also of
	// a family without a care contract (#6692); empty removes it. A key that
	// names no family of the Träger is ErrFamilyNotFound.
	SetFamilyDebtorNumber(ctx context.Context, reviewer HolidayReviewer, input FamilyDebtorNumberInput) (FamilyDebtorNumber, error)
	// DebtorNumberFormat is the format every number the office enters must
	// have (#6675), so a form can name it.
	DebtorNumberFormat(ctx context.Context, reviewer HolidayReviewer) (DebtorNumberFormat, error)
}
