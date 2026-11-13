package parentbilling

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The confirmation letter of a holiday registration that is its invoice
// (spec #4611 user stories 73, 91 and 92, #4669, ADR 0121), modelled on
// the Bissendorf letter. Once a run is released, the Abrechnung writes the
// payment date, a free text for the parents and the emergency numbers of
// the hosting OGS, and the letter of every issued invoice is rendered once
// in the layout of the Träger: its logo, colour, letterhead, footer,
// contact person and default bank account. A letter never changes, like
// its invoice.

// Limits of the letters of a run.
const (
	// MaxLetterNotesLength is the longest free text of a run.
	MaxLetterNotesLength = 4000
	// MaxLetterEmergencyContacts is the most emergency numbers a run
	// prints.
	MaxLetterEmergencyContacts = 10
	// MaxLetterContactLength is the longest name or number of an emergency
	// contact.
	MaxLetterContactLength = 120
)

var (
	// ErrBillingRunNotReleased reports letters or the overview (#4672) of a
	// run that has no issued invoice yet.
	ErrBillingRunNotReleased = errors.New("the billing run is not released yet")
	// ErrBillingLettersExist reports a second creation of the letters of a
	// run.
	ErrBillingLettersExist = errors.New("the letters of the billing run were already created")
	// ErrBillingLetterNotFound reports a letter that was not created, or an
	// invoice that is not part of the run.
	ErrBillingLetterNotFound = errors.New("billing letter not found")
	// ErrBankAccountMissing reports letters of a Träger without a bank
	// account to pay to.
	ErrBankAccountMissing = errors.New("the Träger has no bank account for the letters")
	// ErrLetterheadTooLong reports a letterhead or footer of the Träger
	// that leaves too little room on a page for the letter.
	ErrLetterheadTooLong = errors.New("the letterhead or footer of the Träger is too long for a letter")
	// ErrPaymentTextDebtorNumberMissing reports letters whose payment text
	// names the debtor number while an invoice of the run has none (#5622).
	ErrPaymentTextDebtorNumberMissing = errors.New("the payment text names the debtor number but an invoice has none")
)

// PaymentTextDebtorNumberMissingError names the invoices of a run without
// a debtor number when the payment text of the Träger asks for one. It is
// ErrPaymentTextDebtorNumberMissing.
type PaymentTextDebtorNumberMissingError struct {
	InvoiceNumbers []string
}

func (e *PaymentTextDebtorNumberMissingError) Error() string {
	return fmt.Sprintf("%s: %d invoices", ErrPaymentTextDebtorNumberMissing, len(e.InvoiceNumbers))
}

func (e *PaymentTextDebtorNumberMissingError) Unwrap() error {
	return ErrPaymentTextDebtorNumberMissing
}

// BillingEmergencyContact is the emergency number of a hosting OGS.
type BillingEmergencyContact struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// BillingLetterInput is what the Abrechnung writes for the letters of a
// run.
type BillingLetterInput struct {
	PaymentDueOn      calendar.Date
	Notes             string
	EmergencyContacts []BillingEmergencyContact
}

// BillingLetters are the letters of a released run as the run shows them.
type BillingLetters struct {
	PaymentDueOn       calendar.Date             `json:"payment_due_on"`
	Notes              string                    `json:"notes"`
	EmergencyContacts  []BillingEmergencyContact `json:"emergency_contacts"`
	CreatedAt          time.Time                 `json:"created_at"`
	CreatedByAccountID *int64                    `json:"created_by_account_id"`
	// TemplateVersion is the version of the Träger's template for the
	// confirmation letter the letters were written with; 0 is the standard
	// template (#4677).
	TemplateVersion int `json:"template_version"`
}

// BillingLetterFile is the PDF of one letter.
type BillingLetterFile struct {
	Filename    string
	ContentType string
	Data        []byte
}

// LetterheadBankAccount is the account the letters ask the parents to pay
// to.
type LetterheadBankAccount struct {
	AccountHolder string
	IBAN          string
	BIC           string
	BankName      string
}

// Letterhead is the layout of the Träger a letter prints: the logo (PNG or
// JPEG, nil without), the colour and the texts of letterhead and footer,
// the contact person and the default bank account (nil without). The
// payment text is the Träger's own payment sentence with its placeholders
// (#5622); empty keeps the built-in sentence.
type Letterhead struct {
	Logo         []byte
	AccentColor  string
	SenderText   string
	FooterText   string
	ContactName  string
	ContactPhone string
	ContactEmail string
	BankAccount  *LetterheadBankAccount
	PaymentText  string
	// Template is the Träger's current template of the confirmation letter
	// (#4677).
	Template LetterTemplate
}

// LetterTemplate is a version of the template of the confirmation letter
// with its placeholders (#4677, ADR 0152) and the layout of its table of
// amounts (#5691). Version 0 is the standard template.
type LetterTemplate struct {
	Version    int
	Subject    string
	Salutation string
	Body       string
	Closing    string
	// GroupByChild names each child once above its positions with a
	// subtotal; HideTimeModels leaves the time model out of a booking's
	// line. The week always stays: it is the period of the service.
	GroupByChild   bool
	HideTimeModels bool
	// LegalRemedy is the Rechtsbehelfsbelehrung of the template of the fee
	// notice, empty for every other kind (#4688).
	LegalRemedy string
}

// BillingLetterheads is the master data and the branding settings of the
// Träger as the letters read them. The root binds it. It runs in the
// Träger transaction of the reviewer's Träger and logs the read of the
// contact person and the bank account (ADR 0112). It carries the current
// template of the confirmation letter (#4677).
type BillingLetterheads interface {
	Letterhead(ctx context.Context, reviewer HolidayReviewer) (Letterhead, error)
	// The letter of an invoice or a fee notice reads the template of its
	// kind (#4688).
	BillingDocumentTemplates
}

// BillingLetterService lets the Abrechnung of a Träger create the letters
// of a released run, look at one before and read each one after, and cancel
// or credit an issued invoice with a document and a letter of its own
// (#4690). Every operation runs in the Träger transaction of the session's
// Träger, which the caller opens, and is recorded in the run's trail.
type BillingLetterService interface {
	// CancelBillingDocument issues the cancellation of an invoice of the
	// run. Once the run has its letters, the cancellation gets its letter at
	// once; otherwise it gets it with the letters of the run.
	CancelBillingDocument(ctx context.Context, reviewer HolidayReviewer, runID, documentID int64, input BillingCancellationInput) (BillingDocument, error)
	// CreditBillingDocument issues a credit for one child of an invoice of
	// the run, at most what is left of the child's amount, with its letter
	// like a cancellation.
	CreditBillingDocument(ctx context.Context, reviewer HolidayReviewer, runID, documentID int64, input BillingCreditInput) (BillingDocument, error)
	// CreateBillingLetters renders and stores the letter of every invoice
	// of the run. A run has its letters once.
	CreateBillingLetters(ctx context.Context, reviewer HolidayReviewer, runID int64, input BillingLetterInput) (BillingLetters, error)
	// PreviewBillingLetter renders the letter of the first invoice of the
	// run with the input, without storing anything.
	PreviewBillingLetter(ctx context.Context, reviewer HolidayReviewer, runID int64, input BillingLetterInput) (BillingLetterFile, error)
	// BillingLetter reads the stored letter of an invoice of the run.
	BillingLetter(ctx context.Context, reviewer HolidayReviewer, runID, documentID int64) (BillingLetterFile, error)
}
