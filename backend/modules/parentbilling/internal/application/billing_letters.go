package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// runEventLetters is the action of the trail when the letters of a run are
// created; reading one is a read with the letter in its details.
const runEventLetters = "letters"

// LetterStore persists the letters of the released runs of a Träger in its
// Träger transaction.
type LetterStore interface {
	BillingRun(ctx context.Context, organizationID, runID int64) (parentbilling.BillingRun, error)
	LockBillingRun(ctx context.Context, organizationID, runID int64) (parentbilling.BillingRun, error)
	RunDocuments(ctx context.Context, organizationID, runID int64) ([]parentbilling.BillingDocument, error)
	// RunLetters reads what the Abrechnung wrote for the letters of a run;
	// nil before they are created.
	RunLetters(ctx context.Context, organizationID, runID int64) (*parentbilling.BillingLetters, error)
	// InsertRunLetters reports letters that already exist as
	// ErrBillingLettersExist.
	InsertRunLetters(ctx context.Context, organizationID, runID, accountID int64, input parentbilling.BillingLetterInput) (parentbilling.BillingLetters, error)
	// InsertDocumentLetter stores the PDF; the database adds its SHA-256.
	InsertDocumentLetter(ctx context.Context, organizationID, runID, documentID int64, file parentbilling.BillingLetterFile) error
	// DocumentLetter reads the stored letter of an invoice of the run and
	// reports a missing one as ErrBillingLetterNotFound.
	DocumentLetter(ctx context.Context, organizationID, runID, documentID int64) (parentbilling.BillingLetterFile, error)
	InsertRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error
	// The numbering and storing of a cancellation or a credit (#4690).
	LockPeriodBilling(ctx context.Context, organizationID int64, service string, periodID int64) error
	ListNumberRanges(ctx context.Context, organizationID int64) ([]parentbilling.NumberRange, error)
	ReserveNumbers(ctx context.Context, organizationID int64, service string, year, count int) (int64, error)
	IssueDocument(ctx context.Context, organizationID int64, run parentbilling.BillingRun, year int, sequence int64, document parentbilling.BillingDocument) (int64, error)
	ReferencingDocuments(ctx context.Context, organizationID int64, documentIDs []int64) ([]parentbilling.BillingDocument, error)
}

// BillingLetters serves parentbilling.BillingLetterService. The layout of
// the Träger comes through the root-bound port; the letter renderer lays
// it out.
type BillingLetters struct {
	store       LetterStore
	letterheads parentbilling.BillingLetterheads
	renderer    LetterRenderer
	today       func() calendar.Date
}

var _ parentbilling.BillingLetterService = (*BillingLetters)(nil)

// NewBillingLetters builds the letters. today bounds the payment date.
func NewBillingLetters(store LetterStore, letterheads parentbilling.BillingLetterheads, renderer LetterRenderer, today func() calendar.Date) *BillingLetters {
	if store == nil || letterheads == nil || renderer == nil || today == nil {
		panic("parent billing: the store, the letterheads, the renderer and the clock are required")
	}
	return &BillingLetters{store: store, letterheads: letterheads, renderer: renderer, today: today}
}

// CreateBillingLetters renders the letter of every invoice of a released
// run once and stores it. The run is held while it does,
// so two creations run one after another. The caller rolls the transaction
// back when this fails.
func (b *BillingLetters) CreateBillingLetters(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, input parentbilling.BillingLetterInput) (parentbilling.BillingLetters, error) {
	input, err := b.prepare(reviewer, input)
	if err != nil {
		return parentbilling.BillingLetters{}, err
	}
	run, err := b.releasedRun(ctx, reviewer.OrganizationID, runID, true)
	if err != nil {
		return parentbilling.BillingLetters{}, err
	}
	existing, err := b.store.RunLetters(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return parentbilling.BillingLetters{}, fmt.Errorf("parent billing: read letters: %w", err)
	}
	if existing != nil {
		return parentbilling.BillingLetters{}, parentbilling.ErrBillingLettersExist
	}
	head, documents, err := b.material(ctx, reviewer, run)
	if err != nil {
		return parentbilling.BillingLetters{}, err
	}
	letters, err := b.store.InsertRunLetters(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, input)
	if err != nil {
		return parentbilling.BillingLetters{}, err
	}
	for _, document := range documents {
		file, err := b.render(runLetterContent(run, documents, document, input, head))
		if err != nil {
			return parentbilling.BillingLetters{}, err
		}
		if err := b.store.InsertDocumentLetter(ctx, reviewer.OrganizationID, run.ID, document.ID, file); err != nil {
			return parentbilling.BillingLetters{}, fmt.Errorf("parent billing: store letter: %w", err)
		}
	}
	if err := b.store.InsertRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventLetters,
		map[string]any{"document_count": len(documents)}); err != nil {
		return parentbilling.BillingLetters{}, fmt.Errorf("parent billing: record letters: %w", err)
	}
	return letters, nil
}

// PreviewBillingLetter renders the letter of the first invoice of a
// released run with the input and stores nothing but the read. It picks an
// invoice, never a Storno or a Gutschrift, and builds the letter as the
// creation does, so preview and letter agree (#4690).
func (b *BillingLetters) PreviewBillingLetter(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, input parentbilling.BillingLetterInput) (parentbilling.BillingLetterFile, error) {
	input, err := b.prepare(reviewer, input)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	run, err := b.releasedRun(ctx, reviewer.OrganizationID, runID, false)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	head, documents, err := b.material(ctx, reviewer, run)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	invoice, ok := firstInvoice(documents)
	if !ok {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrBillingRunEmpty
	}
	file, err := b.render(runLetterContent(run, documents, invoice, input, head))
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if err := b.recordRead(ctx, reviewer, run.ID, invoice.ID, "preview"); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	return file, nil
}

// BillingLetter reads the stored letter of an invoice and records the read.
func (b *BillingLetters) BillingLetter(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, documentID int64) (parentbilling.BillingLetterFile, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if runID <= 0 {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrBillingRunNotFound
	}
	if documentID <= 0 {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrBillingLetterNotFound
	}
	run, err := b.store.BillingRun(ctx, reviewer.OrganizationID, runID)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	file, err := b.store.DocumentLetter(ctx, reviewer.OrganizationID, run.ID, documentID)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if err := b.recordRead(ctx, reviewer, run.ID, documentID, "download"); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	return file, nil
}

func (b *BillingLetters) prepare(reviewer parentbilling.HolidayReviewer, input parentbilling.BillingLetterInput) (parentbilling.BillingLetterInput, error) {
	if err := validReviewer(reviewer); err != nil {
		return input, err
	}
	return normalizeLetterInput(input, b.today())
}

// firstInvoice is the first invoice among the documents of a run.
func firstInvoice(documents []parentbilling.BillingDocument) (parentbilling.BillingDocument, bool) {
	for _, document := range documents {
		if document.Kind == parentbilling.DocumentInvoice || document.Kind == "" {
			return document, true
		}
	}
	return parentbilling.BillingDocument{}, false
}

// releasedRun reads a run that has issued its invoices; lock holds it.
func (b *BillingLetters) releasedRun(ctx context.Context, organizationID, runID int64, lock bool) (parentbilling.BillingRun, error) {
	if runID <= 0 {
		return parentbilling.BillingRun{}, parentbilling.ErrBillingRunNotFound
	}
	read := b.store.BillingRun
	if lock {
		read = b.store.LockBillingRun
	}
	run, err := read(ctx, organizationID, runID)
	if err != nil {
		return parentbilling.BillingRun{}, err
	}
	if run.Status != parentbilling.RunReleased {
		return parentbilling.BillingRun{}, parentbilling.ErrBillingRunNotReleased
	}
	return run, nil
}

// material reads the layout of the Träger and the issued invoices. A
// letter asks for payment, so the Träger needs a bank account.
func (b *BillingLetters) material(ctx context.Context, reviewer parentbilling.HolidayReviewer, run parentbilling.BillingRun) (parentbilling.Letterhead, []parentbilling.BillingDocument, error) {
	head, err := b.letterheads.Letterhead(ctx, reviewer)
	if err != nil {
		return parentbilling.Letterhead{}, nil, fmt.Errorf("parent billing: read letterhead: %w", err)
	}
	if head.BankAccount == nil {
		return parentbilling.Letterhead{}, nil, parentbilling.ErrBankAccountMissing
	}
	documents, err := b.store.RunDocuments(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return parentbilling.Letterhead{}, nil, fmt.Errorf("parent billing: read invoices: %w", err)
	}
	if len(documents) == 0 {
		return parentbilling.Letterhead{}, nil, parentbilling.ErrBillingRunEmpty
	}
	if err := debtorNumbersForPaymentText(head, documents); err != nil {
		return parentbilling.Letterhead{}, nil, err
	}
	return head, documents, nil
}

// debtorNumbersForPaymentText refuses letters whose payment text names the
// debtor number while an invoice that asks for payment has none: no letter
// prints a gap where the reference to pay with belongs (#5622). An invoice
// of 0 € prints the fixed sentence without a reference.
func debtorNumbersForPaymentText(head parentbilling.Letterhead, documents []parentbilling.BillingDocument) error {
	var missing []string
	for _, document := range documents {
		if lacksDebtorNumberForPaymentText(head.PaymentText, document) {
			missing = append(missing, document.Number)
		}
	}
	if len(missing) > 0 {
		return &parentbilling.PaymentTextDebtorNumberMissingError{InvoiceNumbers: missing}
	}
	return nil
}

// lacksDebtorNumberForPaymentText reports whether the payment text names the
// debtor number while the invoice asks for payment without one. The billing
// run counts these invoices before the release, which copies the number and
// locks it (#5621, #5622). A cancellation or a credit asks for no payment
// (#4690).
func lacksDebtorNumberForPaymentText(paymentText string, document parentbilling.BillingDocument) bool {
	return document.TotalCents > 0 && strings.TrimSpace(document.DebtorNumber) == "" &&
		strings.Contains(paymentText, placeholderDebtorNumber)
}

func (b *BillingLetters) render(content letterContent) (parentbilling.BillingLetterFile, error) {
	letter, prefix := confirmationLetter(content), "bestaetigung-"
	switch content.document.Kind {
	case parentbilling.DocumentCancellation:
		letter, prefix = cancellationLetter(content), "storno-"
	case parentbilling.DocumentCredit:
		letter, prefix = creditLetter(content), "gutschrift-"
	}
	file, err := b.renderer.RenderLetter(letter, prefix+content.document.Number)
	if err != nil {
		return parentbilling.BillingLetterFile{}, fmt.Errorf("parent billing: render letter: %w", err)
	}
	return file, nil
}

func (b *BillingLetters) recordRead(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, documentID int64, letter string) error {
	if err := b.store.InsertRunEvent(ctx, reviewer.OrganizationID, runID, reviewer.AccountID, runEventRead,
		map[string]any{"document_id": documentID, "letter": letter}); err != nil {
		return fmt.Errorf("parent billing: record letter read: %w", err)
	}
	return nil
}

// normalizeLetterInput checks the input of the letters: a payment date not
// before today, a free text within its length and complete emergency
// numbers.
func normalizeLetterInput(input parentbilling.BillingLetterInput, today calendar.Date) (parentbilling.BillingLetterInput, error) {
	input.Notes = strings.TrimSpace(strings.ReplaceAll(input.Notes, "\r\n", "\n"))
	errs := validation.Errors{}
	switch {
	case input.PaymentDueOn.IsZero():
		errs["payment_due_on"] = errors.New("is required")
	case input.PaymentDueOn.Before(today):
		errs["payment_due_on"] = errors.New("must not be in the past")
	}
	if len([]rune(input.Notes)) > parentbilling.MaxLetterNotesLength {
		errs["notes"] = errors.New("is too long")
	}
	contacts, err := normalizeEmergencyContacts(input.EmergencyContacts)
	if err != nil {
		errs["emergency_contacts"] = err
	}
	input.EmergencyContacts = contacts
	if len(errs) > 0 {
		return input, errs
	}
	return input, nil
}

func normalizeEmergencyContacts(contacts []parentbilling.BillingEmergencyContact) ([]parentbilling.BillingEmergencyContact, error) {
	if len(contacts) > parentbilling.MaxLetterEmergencyContacts {
		return nil, errors.New("too many emergency numbers")
	}
	normalized := make([]parentbilling.BillingEmergencyContact, 0, len(contacts))
	for _, contact := range contacts {
		contact.Name, contact.Phone = strings.TrimSpace(contact.Name), strings.TrimSpace(contact.Phone)
		if contact.Name == "" || contact.Phone == "" {
			return nil, errors.New("every emergency number needs a name and a number")
		}
		if len([]rune(contact.Name)) > parentbilling.MaxLetterContactLength || len([]rune(contact.Phone)) > parentbilling.MaxLetterContactLength {
			return nil, errors.New("an emergency number is too long")
		}
		normalized = append(normalized, contact)
	}
	return normalized, nil
}
