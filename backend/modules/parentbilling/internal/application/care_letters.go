package application

import (
	"context"
	"errors"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The letters, the mail, the print and the Storno of the monthly invoices
// (#4684, ADR 0165). They reuse the layout, the renderer, the mail kind, the
// outbox, the inbox push, the number range and the refusal of every change of
// the holiday letters; only the tables are the monthly run's own.

// Actions of the trail of a monthly run for its letters.
const (
	careEventLetters = "letters"
	careEventMail    = "mail"
	careEventCancel  = "cancel"
)

// CareLetterStore persists the letters of the released monthly runs of a
// Träger in its Träger transaction.
type CareLetterStore interface {
	CareRun(ctx context.Context, organizationID, runID int64, lock bool) (parentbilling.CareRun, error)
	LockCareBilling(ctx context.Context, organizationID int64) error
	CareRunInvoices(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareInvoice, error)
	// CareRunLetters reads what the Abrechnung wrote; nil before the letters
	// are created.
	CareRunLetters(ctx context.Context, organizationID, runID int64) (*parentbilling.CareLetterSet, error)
	// InsertCareRunLetters reports letters that already exist as
	// ErrBillingLettersExist.
	InsertCareRunLetters(ctx context.Context, organizationID, runID, accountID int64, input parentbilling.CareLetterInput) (parentbilling.CareLetterSet, error)
	InsertCareLetter(ctx context.Context, organizationID, runID, invoiceID int64, kind string, file parentbilling.BillingLetterFile) (int64, error)
	// CareRunLetterRecords reads the letters of a run, with their PDF when
	// withFiles is set.
	CareRunLetterRecords(ctx context.Context, organizationID, runID int64, withFiles bool) ([]parentbilling.CareLetterRecord, error)
	// CareLetterRecord reads only the requested PDF, scoped to its Träger and run.
	CareLetterRecord(ctx context.Context, organizationID, runID, invoiceID int64, kind string) (parentbilling.CareLetterRecord, error)
	CareRunCancellations(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareCancellation, error)
	// InsertCareCancellation reports a second Storno of an invoice as
	// ErrCareInvoiceCancelled and a number another invoice carries as
	// ErrInvoiceNumberTaken.
	InsertCareCancellation(ctx context.Context, organizationID, runID, accountID int64, year int, sequence int64,
		cancellation parentbilling.CareCancellation) (parentbilling.CareCancellation, error)
	// CareRunMailings reads every send of the letters of a run in the order
	// they were stored under the run lock.
	CareRunMailings(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareLetterSend, error)
	InsertCareMailing(ctx context.Context, organizationID, runID, accountID int64, send parentbilling.CareLetterSend) (parentbilling.CareLetterSend, error)
	InsertCareRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error
	ListNumberRanges(ctx context.Context, organizationID int64) ([]parentbilling.NumberRange, error)
	ReserveNumbers(ctx context.Context, organizationID int64, service string, year, count int) (int64, error)
}

// CareLetterRecipients reads the party an invoice of a contract goes to now.
type CareLetterRecipients interface {
	Recipients(ctx context.Context, organizationID int64, contractIDs []int64) (map[int64]parentbilling.CareRecipient, error)
}

// CareLetters serves parentbilling.CareLetterService.
type CareLetters struct {
	store       CareLetterStore
	letterheads parentbilling.BillingLetterheads
	recipients  CareLetterRecipients
	post        parentbilling.BillingLetterPost
	carePost    parentbilling.CareLetterPost
	renderer    LetterRenderer
	combiner    LetterCombiner
	today       func() calendar.Date
}

var _ parentbilling.CareLetterService = (*CareLetters)(nil)

// CareLetterPorts are the ports the root binds for the monthly letters.
type CareLetterPorts struct {
	Letterheads parentbilling.BillingLetterheads
	Recipients  CareLetterRecipients
	Post        parentbilling.BillingLetterPost
	CarePost    parentbilling.CareLetterPost
}

// NewCareLetters builds the letters of the monthly runs.
func NewCareLetters(store CareLetterStore, ports CareLetterPorts, renderer LetterRenderer, combiner LetterCombiner, today func() calendar.Date) *CareLetters {
	if store == nil || ports.Letterheads == nil || ports.Recipients == nil || ports.Post == nil || ports.CarePost == nil ||
		renderer == nil || combiner == nil || today == nil {
		panic("parent billing: the store, the ports, the renderer, the combiner and the clock are required")
	}
	return &CareLetters{store: store, letterheads: ports.Letterheads, recipients: ports.Recipients, post: ports.Post,
		carePost: ports.CarePost, renderer: renderer, combiner: combiner, today: today}
}

// careLetterRun is a released monthly run with its invoices, their Storno,
// letters and sends.
type careLetterRun struct {
	run           parentbilling.CareRun
	letters       *parentbilling.CareLetterSet
	invoices      []parentbilling.CareInvoice
	cancellations map[int64]parentbilling.CareCancellation
	records       map[careLetterKey]parentbilling.CareLetterRecord
	sends         map[int64][]parentbilling.CareLetterSend
}

type careLetterKey struct {
	invoiceID int64
	kind      string
}

func (c careLetterRun) record(invoiceID int64, kind string) (parentbilling.CareLetterRecord, bool) {
	record, ok := c.records[careLetterKey{invoiceID: invoiceID, kind: kind}]
	return record, ok
}

// wasSent tells whether the letter of the invoice went out at least once.
func (c careLetterRun) wasSent(invoiceID int64) bool {
	record, ok := c.record(invoiceID, parentbilling.CareLetterInvoice)
	return ok && len(c.sends[record.ID]) > 0
}

// CareLetters reads the letters of a released run with the state of every
// invoice and Storno and records the read.
func (l *CareLetters) CareLetters(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.CareLetters, error) {
	current, err := l.letterRun(ctx, reviewer, runID, false, false)
	if err != nil {
		return parentbilling.CareLetters{}, err
	}
	return l.overview(ctx, reviewer, current)
}

// CreateCareLetters renders the letter of every invoice of a released run
// that is not cancelled, once, and stores it. A cancelled invoice was never
// sent, so neither it nor its Storno gets a letter.
func (l *CareLetters) CreateCareLetters(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, input parentbilling.CareLetterInput) (parentbilling.CareLetters, error) {
	input, err := l.prepare(reviewer, input)
	if err != nil {
		return parentbilling.CareLetters{}, err
	}
	current, err := l.letterRun(ctx, reviewer, runID, true, false)
	if err != nil {
		return parentbilling.CareLetters{}, err
	}
	if current.letters != nil {
		return parentbilling.CareLetters{}, parentbilling.ErrBillingLettersExist
	}
	invoices := current.open()
	if len(invoices) == 0 {
		return parentbilling.CareLetters{}, parentbilling.ErrBillingRunEmpty
	}
	head, err := l.letterhead(ctx, reviewer)
	if err != nil {
		return parentbilling.CareLetters{}, err
	}
	if head, err = withNoticeTemplate(ctx, l.letterheads, reviewer.OrganizationID, head, invoices[0].DocumentKind, true); err != nil {
		return parentbilling.CareLetters{}, err
	}
	set, err := l.store.InsertCareRunLetters(ctx, reviewer.OrganizationID, current.run.ID, reviewer.AccountID, input)
	if err != nil {
		return parentbilling.CareLetters{}, err
	}
	current.letters = &set
	for _, invoice := range invoices {
		content := careLetterContent{run: current.run, invoice: invoice, input: input, letterhead: head}
		if err := l.storeLetter(ctx, reviewer, &current, content); err != nil {
			return parentbilling.CareLetters{}, err
		}
	}
	if err := l.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, current.run.ID, reviewer.AccountID, careEventLetters,
		map[string]any{"invoice_count": len(invoices), "invoice_ids": invoiceIDs(invoices)}); err != nil {
		return parentbilling.CareLetters{}, fmt.Errorf("parent billing: record care letters: %w", err)
	}
	return l.overview(ctx, reviewer, current)
}

// PreviewCareLetter renders the letter of the first open invoice with the
// input and stores nothing but the read.
func (l *CareLetters) PreviewCareLetter(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, input parentbilling.CareLetterInput) (parentbilling.BillingLetterFile, error) {
	input, err := l.prepare(reviewer, input)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	current, err := l.letterRun(ctx, reviewer, runID, false, false)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	invoices := current.open()
	if len(invoices) == 0 {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrBillingRunEmpty
	}
	head, err := l.letterhead(ctx, reviewer)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if head, err = withNoticeTemplate(ctx, l.letterheads, reviewer.OrganizationID, head, invoices[0].DocumentKind, true); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	file, err := l.render(careLetterContent{run: current.run, invoice: invoices[0], input: input, letterhead: head})
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if err := l.recordRead(ctx, reviewer, current.run.ID, map[string]any{"letter": "preview", "invoice_id": invoices[0].ID}); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	return file, nil
}

// CareLetter reads the stored letter of an invoice or its Storno and records
// the read.
func (l *CareLetters) CareLetter(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, invoiceID int64, kind string) (parentbilling.BillingLetterFile, error) {
	if err := validLetterKind(kind); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if runID <= 0 {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrCareRunNotFound
	}
	// The run is held like a send or a Storno: a Storno under way finishes
	// first, and the invoice it cancels is no longer handed out.
	run, err := l.store.CareRun(ctx, reviewer.OrganizationID, runID, true)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if run.Status != parentbilling.RunReleased {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrBillingRunNotReleased
	}
	if kind == parentbilling.CareLetterInvoice {
		if err := l.refuseCancelledUnsharedInvoice(ctx, reviewer.OrganizationID, run.ID, invoiceID); err != nil {
			return parentbilling.BillingLetterFile{}, err
		}
	}
	record, err := l.store.CareLetterRecord(ctx, reviewer.OrganizationID, run.ID, invoiceID, kind)
	if err != nil {
		return parentbilling.BillingLetterFile{}, fmt.Errorf("parent billing: read care letter: %w", err)
	}
	if record.File == nil {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrBillingLetterNotFound
	}
	if err := l.recordRead(ctx, reviewer, run.ID, map[string]any{"letter": "download", "invoice_id": invoiceID,
		"letter_id": record.ID}); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	return *record.File, nil
}

// refuseCancelledUnsharedInvoice refuses the letter of a cancelled invoice
// that never reached the family, by mail or on paper: it has no Storno
// letter, so handing it out now would put a void invoice in a family's hands.
// A cancelled invoice that went out stays readable with its Storno letter.
func (l *CareLetters) refuseCancelledUnsharedInvoice(ctx context.Context, organizationID, runID, invoiceID int64) error {
	cancellations, err := l.store.CareRunCancellations(ctx, organizationID, runID)
	if err != nil {
		return fmt.Errorf("parent billing: read care cancellations: %w", err)
	}
	for _, cancellation := range cancellations {
		if cancellation.InvoiceID != invoiceID {
			continue
		}
		if _, err := l.store.CareLetterRecord(ctx, organizationID, runID, invoiceID, parentbilling.CareLetterCancellation); err != nil {
			if errors.Is(err, parentbilling.ErrBillingLetterNotFound) {
				return parentbilling.ErrCareInvoiceCancelled
			}
			return fmt.Errorf("parent billing: read care letter: %w", err)
		}
	}
	return nil
}

// CancelCareInvoice issues the Storno of an issued invoice, numbered from
// the number range "care" like the invoices. It holds the monthly billing of
// the Träger and the run, so a release of the next run of the month waits and
// then bills the contract again. A Storno of an invoice whose letter went out
// or was downloaded for printing gets its letter at once. One never shared
// gets none, and the family never hears of either. The caller rolls back when
// this fails, which returns the number.
func (l *CareLetters) CancelCareInvoice(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, invoiceID int64, input parentbilling.CareCancellationInput) (parentbilling.CareLetters, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareLetters{}, err
	}
	reason, err := normalizeDocumentReason(input.Reason)
	if err != nil {
		return parentbilling.CareLetters{}, validation.Errors{"reason": err}
	}
	if runID <= 0 {
		return parentbilling.CareLetters{}, parentbilling.ErrCareRunNotFound
	}
	if err := l.store.LockCareBilling(ctx, reviewer.OrganizationID); err != nil {
		return parentbilling.CareLetters{}, fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	current, err := l.letterRun(ctx, reviewer, runID, true, false)
	if err != nil {
		return parentbilling.CareLetters{}, err
	}
	invoice, ok := current.invoice(invoiceID)
	if !ok {
		return parentbilling.CareLetters{}, parentbilling.ErrCareInvoiceNotFound
	}
	if _, cancelled := current.cancellations[invoice.ID]; cancelled {
		return parentbilling.CareLetters{}, parentbilling.ErrCareInvoiceCancelled
	}
	cancellation, err := l.issueCancellation(ctx, reviewer, current.run, invoice, reason)
	if err != nil {
		return parentbilling.CareLetters{}, err
	}
	current.cancellations[invoice.ID] = cancellation
	sent, printed, err := l.cancellationLetter(ctx, reviewer, &current, invoice, cancellation)
	if err != nil {
		return parentbilling.CareLetters{}, err
	}
	if err := l.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, current.run.ID, reviewer.AccountID, careEventCancel,
		map[string]any{"invoice_id": invoice.ID, "cancellation_id": cancellation.ID, "invoice_was_sent": sent, "invoice_was_printed": printed}); err != nil {
		return parentbilling.CareLetters{}, fmt.Errorf("parent billing: record care cancellation: %w", err)
	}
	return l.overview(ctx, reviewer, current)
}

// cancellationLetter stores the letter of a Storno whose invoice reached the
// family, by mail or on paper; a Storno of an invoice that never left the
// office gets none. It tells whether the invoice was sent and printed.
func (l *CareLetters) cancellationLetter(ctx context.Context, reviewer parentbilling.HolidayReviewer, current *careLetterRun,
	invoice parentbilling.CareInvoice, cancellation parentbilling.CareCancellation,
) (sent, printed bool, err error) {
	sent = current.wasSent(invoice.ID)
	record, _ := current.record(invoice.ID, parentbilling.CareLetterInvoice)
	printed = record.Printed
	if (!sent && !printed) || current.letters == nil {
		return sent, printed, nil
	}
	head, err := l.letterhead(ctx, reviewer)
	if err != nil {
		return sent, printed, err
	}
	if head, err = withNoticeTemplate(ctx, l.letterheads, reviewer.OrganizationID, head, invoice.DocumentKind, false); err != nil {
		return sent, printed, err
	}
	content := careLetterContent{run: current.run, invoice: invoice, cancellation: &cancellation,
		input: parentbilling.CareLetterInput{PaymentDueOn: current.letters.PaymentDueOn, Notes: current.letters.Notes}, letterhead: head}
	return sent, printed, l.storeLetter(ctx, reviewer, current, content)
}

// issueCancellation numbers the Storno from the counter of its issue year
// and stores it.
func (l *CareLetters) issueCancellation(ctx context.Context, reviewer parentbilling.HolidayReviewer, run parentbilling.CareRun,
	invoice parentbilling.CareInvoice, reason string,
) (parentbilling.CareCancellation, error) {
	pattern, err := numberPatternOf(ctx, l.store, reviewer.OrganizationID, parentbilling.ServiceCare)
	if err != nil {
		return parentbilling.CareCancellation{}, err
	}
	issuedOn := l.today()
	year := numberYear(pattern, issuedOn.Year())
	sequence, err := l.store.ReserveNumbers(ctx, reviewer.OrganizationID, parentbilling.ServiceCare, year, 1)
	if err != nil {
		return parentbilling.CareCancellation{}, fmt.Errorf("parent billing: reserve the number of the Storno: %w", err)
	}
	return l.store.InsertCareCancellation(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, year, sequence,
		parentbilling.CareCancellation{
			InvoiceID: invoice.ID, Number: formatInvoiceNumber(pattern, issuedOn.Year(), sequence), IssuedOn: issuedOn,
			Reason: reason, TotalCents: -invoice.TotalCents,
		})
}

// storeLetter renders and stores one letter and adds it to the run.
func (l *CareLetters) storeLetter(ctx context.Context, reviewer parentbilling.HolidayReviewer, current *careLetterRun, content careLetterContent) error {
	file, err := l.render(content)
	if err != nil {
		return err
	}
	kind := parentbilling.CareLetterInvoice
	if content.cancellation != nil {
		kind = parentbilling.CareLetterCancellation
	}
	id, err := l.store.InsertCareLetter(ctx, reviewer.OrganizationID, current.run.ID, content.invoice.ID, kind, file)
	if err != nil {
		return fmt.Errorf("parent billing: store care letter: %w", err)
	}
	current.records[careLetterKey{invoiceID: content.invoice.ID, kind: kind}] = parentbilling.CareLetterRecord{
		ID: id, InvoiceID: content.invoice.ID, Kind: kind,
	}
	return nil
}

func (l *CareLetters) render(content careLetterContent) (parentbilling.BillingLetterFile, error) {
	notice := content.invoice.DocumentKind == parentbilling.DocumentKindFeeNotice
	var letter Letter
	var filename string
	switch {
	case content.cancellation != nil && notice:
		letter, filename = careNoticeCancellationLetter(content), "aufhebungsbescheid-"+content.cancellation.Number
	case content.cancellation != nil:
		letter, filename = careCancellationLetter(content), "storno-"+content.cancellation.Number
	case notice:
		letter, filename = careNoticeLetter(content), "gebuehrenbescheid-"+content.invoice.Number
	default:
		letter, filename = careInvoiceLetter(content), "rechnung-"+content.invoice.Number
	}
	file, err := l.renderer.RenderLetter(letter, filename)
	if err != nil {
		return parentbilling.BillingLetterFile{}, fmt.Errorf("parent billing: render care letter: %w", err)
	}
	return file, nil
}

// withNoticeTemplate puts the Träger's template of the fee notice on the
// letterhead of a monthly fee notice (#4688). A monthly invoice keeps its
// built-in text and reads no template.
func withNoticeTemplate(ctx context.Context, templates parentbilling.BillingDocumentTemplates, organizationID int64,
	head parentbilling.Letterhead, documentKind string, required bool,
) (parentbilling.Letterhead, error) {
	if documentKind != parentbilling.DocumentKindFeeNotice {
		return head, nil
	}
	return withDocumentTemplate(ctx, templates, organizationID, head, documentKind, required)
}

// letterhead reads the layout of the Träger; a letter asks for payment, so
// the Träger needs a bank account.
func (l *CareLetters) letterhead(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.Letterhead, error) {
	head, err := l.letterheads.Letterhead(ctx, reviewer)
	if err != nil {
		return parentbilling.Letterhead{}, fmt.Errorf("parent billing: read letterhead: %w", err)
	}
	if head.BankAccount == nil {
		return parentbilling.Letterhead{}, parentbilling.ErrBankAccountMissing
	}
	return head, nil
}

func (l *CareLetters) prepare(reviewer parentbilling.HolidayReviewer, input parentbilling.CareLetterInput) (parentbilling.CareLetterInput, error) {
	if err := validReviewer(reviewer); err != nil {
		return input, err
	}
	normalized, err := normalizeLetterInput(parentbilling.BillingLetterInput{PaymentDueOn: input.PaymentDueOn, Notes: input.Notes}, l.today())
	return parentbilling.CareLetterInput{PaymentDueOn: normalized.PaymentDueOn, Notes: normalized.Notes}, err
}

// letterRun reads a released run with its invoices, their Storno, letters
// and sends; lock holds the run, withFiles reads the PDFs.
func (l *CareLetters) letterRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, lock, withFiles bool) (careLetterRun, error) {
	if err := validReviewer(reviewer); err != nil {
		return careLetterRun{}, err
	}
	if runID <= 0 {
		return careLetterRun{}, parentbilling.ErrCareRunNotFound
	}
	organizationID := reviewer.OrganizationID
	run, err := l.store.CareRun(ctx, organizationID, runID, lock)
	if err != nil {
		return careLetterRun{}, err
	}
	if run.Status != parentbilling.RunReleased {
		return careLetterRun{}, parentbilling.ErrBillingRunNotReleased
	}
	current := careLetterRun{run: run, cancellations: map[int64]parentbilling.CareCancellation{},
		records: map[careLetterKey]parentbilling.CareLetterRecord{}, sends: map[int64][]parentbilling.CareLetterSend{}}
	if current.letters, err = l.store.CareRunLetters(ctx, organizationID, run.ID); err != nil {
		return careLetterRun{}, fmt.Errorf("parent billing: read care letters: %w", err)
	}
	if current.invoices, err = l.store.CareRunInvoices(ctx, organizationID, run.ID); err != nil {
		return careLetterRun{}, err
	}
	cancellations, err := l.store.CareRunCancellations(ctx, organizationID, run.ID)
	if err != nil {
		return careLetterRun{}, fmt.Errorf("parent billing: read care cancellations: %w", err)
	}
	for _, cancellation := range cancellations {
		current.cancellations[cancellation.InvoiceID] = cancellation
	}
	if current.letters == nil {
		return current, nil
	}
	records, err := l.store.CareRunLetterRecords(ctx, organizationID, run.ID, withFiles)
	if err != nil {
		return careLetterRun{}, fmt.Errorf("parent billing: read care letters: %w", err)
	}
	for _, record := range records {
		current.records[careLetterKey{invoiceID: record.InvoiceID, kind: record.Kind}] = record
	}
	sends, err := l.store.CareRunMailings(ctx, organizationID, run.ID)
	if err != nil {
		return careLetterRun{}, fmt.Errorf("parent billing: read care mails: %w", err)
	}
	for _, send := range sends {
		current.sends[send.LetterID] = append(current.sends[send.LetterID], send)
	}
	return current, nil
}

// open lists the invoices of the run without a Storno.
func (c careLetterRun) open() []parentbilling.CareInvoice {
	open := make([]parentbilling.CareInvoice, 0, len(c.invoices))
	for _, invoice := range c.invoices {
		if _, cancelled := c.cancellations[invoice.ID]; !cancelled {
			open = append(open, invoice)
		}
	}
	return open
}

func (c careLetterRun) invoice(invoiceID int64) (parentbilling.CareInvoice, bool) {
	for _, invoice := range c.invoices {
		if invoice.ID == invoiceID {
			return invoice, true
		}
	}
	return parentbilling.CareInvoice{}, false
}

func (l *CareLetters) recordRead(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, details map[string]any) error {
	if err := l.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, runID, reviewer.AccountID, runEventRead, details); err != nil {
		return fmt.Errorf("parent billing: record care letter read: %w", err)
	}
	return nil
}

func validLetterKind(kind string) error {
	if kind != parentbilling.CareLetterInvoice && kind != parentbilling.CareLetterCancellation {
		return validation.Errors{"kind": errors.New("must be invoice or cancellation")}
	}
	return nil
}

func invoiceIDs(invoices []parentbilling.CareInvoice) []int64 {
	ids := make([]int64, len(invoices))
	for i, invoice := range invoices {
		ids[i] = invoice.ID
	}
	return ids
}
