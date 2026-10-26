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
		file, err := b.render(letterContent{run: run, document: document, input: input, letterhead: head})
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
// released run with the input and stores nothing but the read.
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
	file, err := b.render(letterContent{run: run, document: documents[0], input: input, letterhead: head})
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if err := b.recordRead(ctx, reviewer, run.ID, documents[0].ID, "preview"); err != nil {
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
	return head, documents, nil
}

func (b *BillingLetters) render(content letterContent) (parentbilling.BillingLetterFile, error) {
	file, err := b.renderer.RenderLetter(confirmationLetter(content), "bestaetigung-"+content.document.Number)
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
