package parentbilling

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The letters of the monthly invoices (#4684, ADR 0165). A released monthly
// run gets the letters, the mail, the inbox of the parents, the print and the
// Storno of the holiday invoices (ADR 0119, 0121, 0123, 0126): the same
// letter layout of the Träger, the same mail kind on Delivery's outbox, the
// same number range "care" and counter as the invoices, the same refusal of
// every change. Issuing and sending stay two steps: the Abrechnung creates
// the letters and then sends them; a family sees a letter only once it was
// sent.
//
// Invariants:
//   - One letter per invoice and per Storno, rendered once (uq_care_letters_invoice).
//   - Every send is one outbox entry; a send while the last mail still waits
//     is refused, a series sends nothing twice. Only the first send pushes.
//   - A send goes only to the address the invoice copied, and only while that
//     address is still the address of a party of the contract.
//   - A Storno of an invoice neither mailed nor downloaded for printing has
//     no letter and sends nothing to the family; its invoice is not sent again.
//   - A cancelled invoice is billed again by a later run of its month, which
//     names it (replaces_invoice_id), at most once.

// Kinds of the letter of a monthly invoice.
const (
	// CareLetterInvoice is the letter of the invoice.
	CareLetterInvoice = "invoice"
	// CareLetterCancellation is the letter of its Storno.
	CareLetterCancellation = "cancellation"
)

// The kinds the parents inbox shows for the letters of a monthly invoice.
const (
	ParentDocumentKindCareInvoiceLetter      = "care_invoice_letter"
	ParentDocumentKindCareCancellationLetter = "care_cancellation_letter"
)

// MailRelatedCareRun is the related entity type of the mails of a monthly
// run.
const MailRelatedCareRun = "parent_billing_care_run"

// The mail kinds of the letters of a monthly invoice and its Storno; they
// travel as MailKindBillingLetter like the holiday letters.
const (
	MailDocumentCareInvoice      = "care_invoice"
	MailDocumentCareCancellation = "care_cancellation"
)

var germanMonthNames = [...]string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September",
	"Oktober", "November", "Dezember"}

// CareMonthName prints a billing month as "November 2026".
func CareMonthName(month calendar.Date) string {
	if month.IsZero() {
		return ""
	}
	return fmt.Sprintf("%s %d", germanMonthNames[month.Month()-1], month.Year())
}

// CarePeriodName names a monthly run as its letters, mails and the inbox of
// the parents do: "Monatsbeitrag November 2026".
func CarePeriodName(month calendar.Date) string {
	return "Monatsbeitrag " + CareMonthName(month)
}

// CareLetterPrintWithoutEmail prints only the letters of families without an
// e-mail address.
const CareLetterPrintWithoutEmail = "without_email"

var (
	// ErrCareInvoiceNotFound reports an invoice that is not part of the run.
	ErrCareInvoiceNotFound = errors.New("monthly invoice not found")
	// ErrCareInvoiceCancelled reports a Storno or a send of an invoice that
	// is cancelled.
	ErrCareInvoiceCancelled = errors.New("the monthly invoice is cancelled")
	// ErrCareRecipientChanged reports a send to an address that is no longer
	// the address of a party of the contract. The Storno and a new invoice
	// copy the party of today.
	ErrCareRecipientChanged = errors.New("the recipient of the monthly invoice is no longer a party of the contract")
)

// CareLetters are the letters of a released monthly run as the run shows
// them: what the Abrechnung wrote, nil before the letters are created, and
// the state of every invoice and Storno.
type CareLetters struct {
	Letters   *CareLetterSet      `json:"letters"`
	Documents []CareLetterMailing `json:"documents"`
	Summary   CareLetterSummary   `json:"summary"`
}

// CareLetterSet is what the Abrechnung wrote for the letters of a run.
type CareLetterSet struct {
	PaymentDueOn calendar.Date `json:"payment_due_on"`
	Notes        string        `json:"notes"`
	CreatedAt    time.Time     `json:"created_at"`
}

// CareLetterInput is what the Abrechnung writes for the letters of a run.
type CareLetterInput struct {
	PaymentDueOn calendar.Date
	Notes        string
}

// CareLetterMailing is one invoice or Storno of a run with its letter and the
// state of its last mail. Status is LetterMailNotSent before the first mail.
// Cancelled marks an invoice with a Storno; HasLetter is false for an invoice
// whose letters are not created yet and for a Storno of an invoice that was
// neither mailed nor downloaded for printing, which has no letter.
type CareLetterMailing struct {
	InvoiceID          int64         `json:"invoice_id"`
	Kind               string        `json:"kind"`
	Number             string        `json:"number"`
	InvoiceNumber      string        `json:"invoice_number"`
	IssuedOn           calendar.Date `json:"issued_on"`
	ChildFirstName     string        `json:"child_first_name"`
	ChildLastName      string        `json:"child_last_name"`
	SchoolName         string        `json:"school_name"`
	RecipientFirstName string        `json:"recipient_first_name"`
	RecipientLastName  string        `json:"recipient_last_name"`
	RecipientEmail     string        `json:"recipient_email"`
	TotalCents         int64         `json:"total_cents"`
	Reason             string        `json:"reason,omitempty"`
	Cancelled          bool          `json:"cancelled"`
	CancellationNumber string        `json:"cancellation_number,omitempty"`
	HasLetter          bool          `json:"has_letter"`
	Printed            bool          `json:"printed"`
	// InInbox tells whether a sent letter lands in the parents inbox too: only
	// when the invoice names the child's record (ADR 0165). Without one the
	// family gets the letter by mail or on paper only.
	InInbox       bool       `json:"in_inbox"`
	Replaced      bool       `json:"replaced"`
	Status        string     `json:"status"`
	Sends         int        `json:"sends"`
	Attempts      int        `json:"attempts"`
	RequestedAt   *time.Time `json:"requested_at"`
	SentAt        *time.Time `json:"sent_at"`
	MailOutcome   string     `json:"mail_outcome,omitempty"`
	ProviderBlock string     `json:"provider_block,omitempty"`
}

// CareLetterSummary counts the letters of a run by the state of their last
// mail. WithoutEmail counts the letters of families without an address; they
// print. Open letters are neither sent, waiting nor failed.
type CareLetterSummary struct {
	Letters      int `json:"letters"`
	Sent         int `json:"sent"`
	Queued       int `json:"queued"`
	Failed       int `json:"failed"`
	NotSent      int `json:"not_sent"`
	WithoutEmail int `json:"without_email"`
	// WithoutInbox counts the letters that reach the family by mail or on
	// paper only, because their child has no record yet.
	WithoutInbox int `json:"without_inbox"`
}

// CareLetterRecord is a stored letter of an invoice or its Storno; File is
// nil where the reader asked without the PDF.
type CareLetterRecord struct {
	ID        int64
	InvoiceID int64
	Kind      string
	File      *BillingLetterFile
	// Printed means this letter was included in a successful print download.
	Printed bool
}

// CareLetterSend is one stored send of a letter: the outbox entry, the
// address it went to and when.
type CareLetterSend struct {
	LetterID       int64
	DeliveryID     int64
	RecipientEmail string
	CreatedAt      time.Time
}

// CareCancellation is the Storno of an issued monthly invoice. TotalCents is
// the negated total of the invoice.
type CareCancellation struct {
	ID         int64
	InvoiceID  int64
	Number     string
	IssuedOn   calendar.Date
	Reason     string
	TotalCents int64
	// Replaced means a later released invoice names this invoice as its source.
	Replaced bool
}

// CareCancellationInput is the reason of a Storno, at most 500 characters.
type CareCancellationInput struct {
	Reason string
}

// CareLetterPost is Delivery as the letters of a monthly run are mailed. The
// root binds it next to BillingLetterPost; both join the caller's Träger
// transaction.
type CareLetterPost interface {
	// EnqueueCareLetterMail puts the mail of the letter on Delivery's outbox
	// with the monthly run as its related entity.
	EnqueueCareLetterMail(ctx context.Context, mail BillingLetterMail) (int64, error)
	// CareLetterMailStates reads the mails of a monthly run Delivery still
	// keeps.
	CareLetterMailStates(ctx context.Context, organizationID, runID int64) ([]BillingLetterMailState, error)
}

// CareLetterService lets the Abrechnung of a Träger create, send, print and
// cancel the letters of a released monthly run. Every operation names the
// person of the session and runs in the Träger transaction of its Träger,
// which the caller opens; every read and change is recorded in the run's
// trail.
type CareLetterService interface {
	// CareLetters reads the letters of the run with the state of every
	// invoice and Storno.
	CareLetters(ctx context.Context, reviewer HolidayReviewer, runID int64) (CareLetters, error)
	// CreateCareLetters renders and stores the letter of every invoice of
	// the run that is not cancelled. A run has its letters once.
	CreateCareLetters(ctx context.Context, reviewer HolidayReviewer, runID int64, input CareLetterInput) (CareLetters, error)
	// PreviewCareLetter renders the letter of the first invoice with the
	// input and stores nothing.
	PreviewCareLetter(ctx context.Context, reviewer HolidayReviewer, runID int64, input CareLetterInput) (BillingLetterFile, error)
	// CareLetter reads the stored letter of an invoice or its Storno.
	CareLetter(ctx context.Context, reviewer HolidayReviewer, runID, invoiceID int64, kind string) (BillingLetterFile, error)
	// MailCareLetters mails every letter with an address and no mail yet.
	MailCareLetters(ctx context.Context, reviewer HolidayReviewer, runID int64) (BillingLetterSeries, error)
	// MailCareLetter mails one letter, first or again, unless its last mail
	// still waits.
	MailCareLetter(ctx context.Context, reviewer HolidayReviewer, runID, invoiceID int64, kind string) (CareLetterMailing, error)
	// PrintCareLetters combines the letters of the run into one PDF; only
	// CareLetterPrintWithoutEmail keeps the families without an address.
	PrintCareLetters(ctx context.Context, reviewer HolidayReviewer, runID int64, only string) (BillingLetterFile, error)
	// CancelCareInvoice issues the Storno of an invoice of the run. An invoice
	// mailed or downloaded for printing gets its Storno letter at once;
	// one never shared gets none, and the family receives nothing.
	CancelCareInvoice(ctx context.Context, reviewer HolidayReviewer, runID, invoiceID int64, input CareCancellationInput) (CareLetters, error)
}
