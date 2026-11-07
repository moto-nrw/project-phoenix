package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The mail, the state and the print of the letters of a monthly run (#4684,
// ADR 0165), the same steps as the holiday letters (ADR 0123): every send is
// one entry on Delivery's outbox in the Träger transaction that records it,
// with the PDF, and the first send also announces the letter in the inbox of
// the parents (ADR 0126). Before every send the address the invoice copied
// must still be the address of the party the contract bills; otherwise the
// Abrechnung cancels the invoice and the next run copies the party of today.

// careSendable is a letter the run may still send: every Storno letter and
// the letter of every invoice without a Storno.
type careSendable struct {
	invoice parentbilling.CareInvoice
	kind    string
	number  string
	record  parentbilling.CareLetterRecord
}

func (c careLetterRun) sendable() []careSendable {
	var letters []careSendable
	for _, invoice := range c.invoices {
		cancellation, cancelled := c.cancellations[invoice.ID]
		if record, ok := c.record(invoice.ID, parentbilling.CareLetterInvoice); ok && !cancelled {
			letters = append(letters, careSendable{invoice: invoice, kind: parentbilling.CareLetterInvoice, number: invoice.Number, record: record})
		}
		if record, ok := c.record(invoice.ID, parentbilling.CareLetterCancellation); ok && cancelled {
			letters = append(letters, careSendable{invoice: invoice, kind: parentbilling.CareLetterCancellation, number: cancellation.Number, record: record})
		}
	}
	return letters
}

// MailCareLetters mails every letter of the run that has an address and no
// mail yet. The run is held while it does, so a second series waits and then
// sends nothing twice.
func (l *CareLetters) MailCareLetters(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.BillingLetterSeries, error) {
	current, err := l.mailRun(ctx, reviewer, runID)
	if err != nil {
		return parentbilling.BillingLetterSeries{}, err
	}
	letters := current.sendable()
	recipients, err := l.currentRecipients(ctx, reviewer.OrganizationID, letters)
	if err != nil {
		return parentbilling.BillingLetterSeries{}, err
	}
	traeger, err := l.traegerName(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.BillingLetterSeries{}, err
	}
	var series parentbilling.BillingLetterSeries
	var queued []int64
	for _, letter := range letters {
		switch {
		case len(current.sends[letter.record.ID]) > 0:
			series.Already++
			continue
		case strings.TrimSpace(letter.invoice.Recipient.Email) == "":
			series.WithoutEmail++
			continue
		case !sameRecipient(letter.invoice, recipients):
			series.RecipientChanged++
			continue
		}
		if _, err := l.send(ctx, reviewer, &current, letter, traeger); err != nil {
			return parentbilling.BillingLetterSeries{}, err
		}
		series.Queued++
		queued = append(queued, letter.record.ID)
	}
	if err := l.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, current.run.ID, reviewer.AccountID, careEventMail,
		map[string]any{"queued": series.Queued, "already": series.Already, "without_email": series.WithoutEmail,
			"recipient_changed": series.RecipientChanged, "letter_ids": queued}); err != nil {
		return parentbilling.BillingLetterSeries{}, fmt.Errorf("parent billing: record care mails: %w", err)
	}
	return series, nil
}

// MailCareLetter mails one letter, first or again. A send while the last mail
// still waits or is delayed, to an address the provider holds back, or of an
// invoice with a Storno is refused, so a second click sends nothing twice.
func (l *CareLetters) MailCareLetter(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, invoiceID int64, kind string) (parentbilling.CareLetterMailing, error) {
	if err := validLetterKind(kind); err != nil {
		return parentbilling.CareLetterMailing{}, err
	}
	current, err := l.mailRun(ctx, reviewer, runID)
	if err != nil {
		return parentbilling.CareLetterMailing{}, err
	}
	letter, err := current.sendableLetter(invoiceID, kind)
	if err != nil {
		return parentbilling.CareLetterMailing{}, err
	}
	if strings.TrimSpace(letter.invoice.Recipient.Email) == "" {
		return parentbilling.CareLetterMailing{}, parentbilling.ErrRecipientEmailMissing
	}
	states, err := l.states(ctx, reviewer.OrganizationID, current.run.ID)
	if err != nil {
		return parentbilling.CareLetterMailing{}, err
	}
	last := []parentbilling.CareLetterMailing{careMailing(current, letter, states)}
	if err := l.refuseWhileWaiting(ctx, last); err != nil {
		return parentbilling.CareLetterMailing{}, err
	}
	recipients, err := l.currentRecipients(ctx, reviewer.OrganizationID, []careSendable{letter})
	if err != nil {
		return parentbilling.CareLetterMailing{}, err
	}
	if !sameRecipient(letter.invoice, recipients) {
		return parentbilling.CareLetterMailing{}, parentbilling.ErrCareRecipientChanged
	}
	traeger, err := l.traegerName(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareLetterMailing{}, err
	}
	send, err := l.send(ctx, reviewer, &current, letter, traeger)
	if err != nil {
		return parentbilling.CareLetterMailing{}, err
	}
	if err := l.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, current.run.ID, reviewer.AccountID, careEventMail,
		map[string]any{"queued": 1, "letter_ids": []int64{letter.record.ID}}); err != nil {
		return parentbilling.CareLetterMailing{}, fmt.Errorf("parent billing: record care mail: %w", err)
	}
	// The mail just waits on the outbox.
	states[send.DeliveryID] = parentbilling.BillingLetterMailState{DeliveryID: send.DeliveryID, Status: parentbilling.LetterMailQueued}
	return careMailing(current, letter, states), nil
}

// refuseWhileWaiting refuses another mail while the last one still waits, is
// delayed, or the provider still holds the address back. The block as it is
// now decides, not what became of the last mail.
func (l *CareLetters) refuseWhileWaiting(ctx context.Context, last []parentbilling.CareLetterMailing) error {
	switch {
	case last[0].Status == parentbilling.LetterMailQueued:
		return parentbilling.ErrLetterMailPending
	case last[0].MailOutcome == parentbilling.LetterMailOutcomeDelayed:
		return parentbilling.ErrLetterMailDelayed
	}
	l.readProviderBlocks(ctx, last)
	switch last[0].ProviderBlock {
	case parentbilling.LetterProviderBlockActive:
		return parentbilling.ErrLetterMailBlocked
	case parentbilling.LetterProviderBlockUnreadable:
		return parentbilling.ErrLetterMailBlockUnreadable
	}
	return nil
}

// PrintCareLetters combines the letters the run may still send, by invoice
// number with each Storno after its invoice; only CareLetterPrintWithoutEmail
// keeps the families without an address.
func (l *CareLetters) PrintCareLetters(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, only string) (parentbilling.BillingLetterFile, error) {
	if only != "" && only != parentbilling.CareLetterPrintWithoutEmail {
		return parentbilling.BillingLetterFile{}, validLetterKind("")
	}
	current, err := l.letterRun(ctx, reviewer, runID, false, true)
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	if current.letters == nil {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrBillingLettersMissing
	}
	var pdfs [][]byte
	var printed []int64
	for _, letter := range current.sendable() {
		if only == parentbilling.CareLetterPrintWithoutEmail && strings.TrimSpace(letter.invoice.Recipient.Email) != "" {
			continue
		}
		if letter.record.File == nil {
			return parentbilling.BillingLetterFile{}, parentbilling.ErrBillingLetterNotFound
		}
		pdfs, printed = append(pdfs, letter.record.File.Data), append(printed, letter.record.ID)
	}
	if len(pdfs) == 0 {
		return parentbilling.BillingLetterFile{}, parentbilling.ErrBillingLetterNotFound
	}
	name := "Rechnungen " + carePeriodName(current.run.BillingMonth)
	file, err := l.combiner.CombineLetters(pdfs, name, name)
	if err != nil {
		return parentbilling.BillingLetterFile{}, fmt.Errorf("parent billing: combine care letters: %w", err)
	}
	if err := l.recordRead(ctx, reviewer, current.run.ID, map[string]any{"letter": "print", "only": only, "letter_ids": printed}); err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	return file, nil
}

// mailRun holds a released run whose letters exist.
func (l *CareLetters) mailRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (careLetterRun, error) {
	current, err := l.letterRun(ctx, reviewer, runID, true, true)
	if err != nil {
		return careLetterRun{}, err
	}
	if current.letters == nil {
		return careLetterRun{}, parentbilling.ErrBillingLettersMissing
	}
	return current, nil
}

func (c careLetterRun) sendableLetter(invoiceID int64, kind string) (careSendable, error) {
	if _, cancelled := c.cancellations[invoiceID]; cancelled && kind == parentbilling.CareLetterInvoice {
		return careSendable{}, parentbilling.ErrCareInvoiceCancelled
	}
	for _, letter := range c.sendable() {
		if letter.invoice.ID == invoiceID && letter.kind == kind {
			return letter, nil
		}
	}
	return careSendable{}, parentbilling.ErrBillingLetterNotFound
}

// send puts the mail of one letter on the outbox and stores the send in the
// same Träger transaction. The key counts the sends of the letter, so each
// send is one outbox entry; only the first send pushes to the inbox.
func (l *CareLetters) send(ctx context.Context, reviewer parentbilling.HolidayReviewer, current *careLetterRun, letter careSendable, traeger string) (parentbilling.CareLetterSend, error) {
	recipient := strings.TrimSpace(letter.invoice.Recipient.Email)
	earlier := len(current.sends[letter.record.ID])
	mailKind := parentbilling.MailDocumentCareInvoice
	if letter.kind == parentbilling.CareLetterCancellation {
		mailKind = parentbilling.MailDocumentCareCancellation
	}
	deliveryID, err := l.carePost.EnqueueCareLetterMail(ctx, parentbilling.BillingLetterMail{
		OrganizationID: reviewer.OrganizationID, RunID: current.run.ID, DocumentID: letter.record.ID, Recipient: recipient,
		IdempotencyKey: fmt.Sprintf("parent-billing-care-letter:%d:%d", letter.record.ID, earlier+1),
		GuardianName:   fullName(letter.invoice.Recipient.FirstName, letter.invoice.Recipient.LastName),
		TraegerName:    traeger, PeriodName: carePeriodName(current.run.BillingMonth), Number: letter.number,
		DocumentKind: mailKind, Filename: letter.record.File.Filename, PDF: letter.record.File.Data,
	})
	if err != nil {
		return parentbilling.CareLetterSend{}, fmt.Errorf("parent billing: enqueue care letter mail: %w", err)
	}
	send, err := l.store.InsertCareMailing(ctx, reviewer.OrganizationID, current.run.ID, reviewer.AccountID,
		parentbilling.CareLetterSend{LetterID: letter.record.ID, DeliveryID: deliveryID, RecipientEmail: recipient})
	if err != nil {
		return parentbilling.CareLetterSend{}, fmt.Errorf("parent billing: record care letter mail: %w", err)
	}
	current.sends[letter.record.ID] = append(current.sends[letter.record.ID], send)
	if earlier > 0 {
		return send, nil
	}
	if err := l.post.EnqueueDocumentNotice(ctx, parentbilling.BillingDocumentNotice{
		OrganizationID: reviewer.OrganizationID, DocumentID: letter.record.ID, RecipientEmail: recipient,
	}); err != nil {
		return parentbilling.CareLetterSend{}, fmt.Errorf("parent billing: enqueue document notice: %w", err)
	}
	return send, nil
}

// currentRecipients reads the party each contract of the letters bills now.
func (l *CareLetters) currentRecipients(ctx context.Context, organizationID int64, letters []careSendable) (map[int64]parentbilling.CareRecipient, error) {
	seen := map[int64]bool{}
	var contracts []int64
	for _, letter := range letters {
		if !seen[letter.invoice.ContractID] {
			seen[letter.invoice.ContractID] = true
			contracts = append(contracts, letter.invoice.ContractID)
		}
	}
	if len(contracts) == 0 {
		return map[int64]parentbilling.CareRecipient{}, nil
	}
	recipients, err := l.recipients.Recipients(ctx, organizationID, contracts)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the recipients of the contracts: %w", err)
	}
	return recipients, nil
}

// sameRecipient tells whether the address the invoice copied is still the
// address the contract bills, without case like the login.
func sameRecipient(invoice parentbilling.CareInvoice, recipients map[int64]parentbilling.CareRecipient) bool {
	now, ok := recipients[invoice.ContractID]
	return ok && strings.EqualFold(strings.TrimSpace(now.Email), strings.TrimSpace(invoice.Recipient.Email))
}

// overview builds the state of every invoice and Storno of the run and
// records the read with the invoices it shows.
func (l *CareLetters) overview(ctx context.Context, reviewer parentbilling.HolidayReviewer, current careLetterRun) (parentbilling.CareLetters, error) {
	states := map[int64]parentbilling.BillingLetterMailState{}
	if current.letters != nil {
		var err error
		if states, err = l.states(ctx, reviewer.OrganizationID, current.run.ID); err != nil {
			return parentbilling.CareLetters{}, err
		}
	}
	documents := make([]parentbilling.CareLetterMailing, 0, len(current.invoices))
	for _, invoice := range current.invoices {
		entry := careMailing(current, careSendable{invoice: invoice, kind: parentbilling.CareLetterInvoice, number: invoice.Number}, states)
		documents = append(documents, entry)
		if _, cancelled := current.cancellations[invoice.ID]; cancelled {
			documents = append(documents, careMailing(current, careSendable{invoice: invoice, kind: parentbilling.CareLetterCancellation}, states))
		}
	}
	l.readProviderBlocks(ctx, documents)
	overview := parentbilling.CareLetters{Letters: current.letters, Documents: documents, Summary: careSummary(documents)}
	if err := l.recordRead(ctx, reviewer, current.run.ID, map[string]any{"letters": "status", "invoice_ids": invoiceIDs(current.invoices)}); err != nil {
		return parentbilling.CareLetters{}, err
	}
	return overview, nil
}

// careMailing is an invoice or Storno of the run with its letter and the
// state of its last send. A send whose outbox entry Delivery no longer keeps
// has an unknown result.
func careMailing(current careLetterRun, letter careSendable, states map[int64]parentbilling.BillingLetterMailState) parentbilling.CareLetterMailing {
	invoice := letter.invoice
	entry := parentbilling.CareLetterMailing{
		InvoiceID: invoice.ID, Kind: letter.kind, Number: invoice.Number, InvoiceNumber: invoice.Number,
		ChildFirstName: invoice.ChildFirstName, ChildLastName: invoice.ChildLastName, SchoolName: invoice.SchoolName,
		RecipientFirstName: invoice.Recipient.FirstName, RecipientLastName: invoice.Recipient.LastName,
		RecipientEmail: invoice.Recipient.Email, TotalCents: invoice.TotalCents, Status: parentbilling.LetterMailNotSent,
	}
	if invoice.IssuedOn != nil {
		entry.IssuedOn = *invoice.IssuedOn
	}
	if cancellation, cancelled := current.cancellations[invoice.ID]; cancelled {
		entry.Cancelled, entry.CancellationNumber = true, cancellation.Number
		if letter.kind == parentbilling.CareLetterCancellation {
			entry.Number, entry.IssuedOn, entry.TotalCents, entry.Reason = cancellation.Number, cancellation.IssuedOn, cancellation.TotalCents, cancellation.Reason
		}
	}
	record, ok := current.record(invoice.ID, letter.kind)
	if !ok {
		return entry
	}
	entry.HasLetter = true
	sends := current.sends[record.ID]
	entry.Sends = len(sends)
	if len(sends) == 0 {
		return entry
	}
	last := sends[len(sends)-1]
	requested := last.CreatedAt
	entry.RequestedAt = &requested
	state, known := states[last.DeliveryID]
	if !known {
		entry.Status = parentbilling.LetterMailUnknown
		return entry
	}
	entry.Status, entry.Attempts, entry.SentAt, entry.MailOutcome = state.Status, state.Attempts, state.SentAt, state.Outcome
	return entry
}

// careSummary counts the letters by the state of their last mail. A cancelled
// invoice that never went out is void and not counted; one that went out
// keeps its state.
func careSummary(documents []parentbilling.CareLetterMailing) parentbilling.CareLetterSummary {
	var summary parentbilling.CareLetterSummary
	for _, document := range documents {
		if !document.HasLetter || (document.Kind == parentbilling.CareLetterInvoice && document.Cancelled && document.Sends == 0) {
			continue
		}
		summary.Letters++
		switch document.Status {
		case parentbilling.LetterMailSent:
			summary.Sent++
		case parentbilling.LetterMailQueued:
			summary.Queued++
		case parentbilling.LetterMailFailed, parentbilling.LetterMailUnknown:
			summary.Failed++
		default:
			summary.NotSent++
			if strings.TrimSpace(document.RecipientEmail) == "" {
				summary.WithoutEmail++
			}
		}
	}
	return summary
}

func (l *CareLetters) states(ctx context.Context, organizationID, runID int64) (map[int64]parentbilling.BillingLetterMailState, error) {
	states, err := l.carePost.CareLetterMailStates(ctx, organizationID, runID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read care mail states: %w", err)
	}
	byID := make(map[int64]parentbilling.BillingLetterMailState, len(states))
	for _, state := range states {
		byID[state.DeliveryID] = state
	}
	return byID, nil
}

// readProviderBlocks asks once per address, and only about letters whose
// last mail left a block at the provider.
func (l *CareLetters) readProviderBlocks(ctx context.Context, documents []parentbilling.CareLetterMailing) {
	var addresses []string
	for _, document := range documents {
		if parentbilling.LetterMailOutcomeHeldBack(document.MailOutcome) {
			addresses = append(addresses, document.RecipientEmail)
		}
	}
	if len(addresses) == 0 {
		return
	}
	blocks := l.post.LetterMailBlocks(ctx, addresses)
	for i := range documents {
		if parentbilling.LetterMailOutcomeHeldBack(documents[i].MailOutcome) {
			documents[i].ProviderBlock = blocks[strings.ToLower(strings.TrimSpace(documents[i].RecipientEmail))]
		}
	}
}

func (l *CareLetters) traegerName(ctx context.Context, organizationID int64) (string, error) {
	text, err := l.post.SenderText(ctx, organizationID)
	if err != nil {
		return "", fmt.Errorf("parent billing: read letterhead: %w", err)
	}
	return firstLine(textLines(text)), nil
}
