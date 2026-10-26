package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	// ErrBillingRunNotReleased reports letters of a run that has no issued
	// invoice yet.
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
)

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
// the contact person and the default bank account (nil without).
type Letterhead struct {
	Logo         []byte
	AccentColor  string
	SenderText   string
	FooterText   string
	ContactName  string
	ContactPhone string
	ContactEmail string
	BankAccount  *LetterheadBankAccount
}

// BillingLetterheads is the master data and the branding settings of the
// Träger as the letters read them. The root binds it. It runs in the
// Träger transaction of the reviewer's Träger and logs the read of the
// contact person and the bank account (ADR 0112).
type BillingLetterheads interface {
	Letterhead(ctx context.Context, reviewer HolidayReviewer) (Letterhead, error)
}

// BillingLetterService lets the Abrechnung of a Träger create the letters
// of a released run, look at one before and read each one after. Every
// operation runs in the Träger transaction of the session's Träger, which
// the caller opens, and is recorded in the run's trail.
type BillingLetterService interface {
	// CreateBillingLetters renders and stores the letter of every invoice
	// of the run. A run has its letters once.
	CreateBillingLetters(ctx context.Context, reviewer HolidayReviewer, runID int64, input BillingLetterInput) (BillingLetters, error)
	// PreviewBillingLetter renders the letter of the first invoice of the
	// run with the input, without storing anything.
	PreviewBillingLetter(ctx context.Context, reviewer HolidayReviewer, runID int64, input BillingLetterInput) (BillingLetterFile, error)
	// BillingLetter reads the stored letter of an invoice of the run.
	BillingLetter(ctx context.Context, reviewer HolidayReviewer, runID, documentID int64) (BillingLetterFile, error)
}
