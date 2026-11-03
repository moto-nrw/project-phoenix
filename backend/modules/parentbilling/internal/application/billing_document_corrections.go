package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// Storno and Gutschrift of an issued invoice (#4690, ADR 0119 addendum).
// Both are documents of their own with a number of the same number range
// as the invoices, in the run of their invoice: a cancellation negates
// every position of the invoice, a credit lowers one child's amount by one
// position. The run, then the invoices of its period are held while they
// are issued, so a cancellation, a credit, the letters of the run and the
// release of the next run of the period run one after another. The caller
// rolls the transaction back when this fails, which returns the number.

// Actions of the trail of a run for a cancellation and a credit.
const (
	runEventCancel = "cancel"
	runEventCredit = "credit"
)

// documentHistoryReader reads the documents that reference others.
type documentHistoryReader interface {
	ReferencingDocuments(ctx context.Context, organizationID int64, documentIDs []int64) ([]parentbilling.BillingDocument, error)
}

// correctionTarget is an issued invoice with its run and the documents
// that already reference it.
type correctionTarget struct {
	run     parentbilling.BillingRun
	invoice parentbilling.BillingDocument
	history []parentbilling.BillingDocument
}

func (t correctionTarget) has(kind string) bool {
	for _, document := range t.history {
		if document.Kind == kind {
			return true
		}
	}
	return false
}

// CancelBillingDocument issues the cancellation of an invoice. An invoice
// with a credit is not cancelled; the Abrechnung credits the rest instead,
// so no amount is returned twice.
func (b *BillingLetters) CancelBillingDocument(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, documentID int64, input parentbilling.BillingCancellationInput) (parentbilling.BillingDocument, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BillingDocument{}, err
	}
	reason, err := normalizeDocumentReason(input.Reason)
	if err != nil {
		return parentbilling.BillingDocument{}, validation.Errors{"reason": err}
	}
	target, err := b.correctable(ctx, reviewer, runID, documentID)
	if err != nil {
		return parentbilling.BillingDocument{}, err
	}
	if target.has(parentbilling.DocumentCredit) {
		return parentbilling.BillingDocument{}, parentbilling.ErrInvoiceCredited
	}
	return b.issueCorrection(ctx, reviewer, target, cancellationOf(target.invoice, reason), runEventCancel)
}

// CreditBillingDocument issues a credit for one child of an invoice: at
// least one cent and at most what the invoice and its earlier credits leave
// of the child's amount.
func (b *BillingLetters) CreditBillingDocument(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, documentID int64, input parentbilling.BillingCreditInput) (parentbilling.BillingDocument, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BillingDocument{}, err
	}
	input, err := normalizeCredit(input)
	if err != nil {
		return parentbilling.BillingDocument{}, err
	}
	target, err := b.correctable(ctx, reviewer, runID, documentID)
	if err != nil {
		return parentbilling.BillingDocument{}, err
	}
	child, ok := findChild(target.invoice, input.ChildID)
	if !ok {
		return parentbilling.BillingDocument{}, validation.Errors{"child_id": errors.New("is not a child of the invoice")}
	}
	if left := child.TotalCents + creditedCents(target.history, input.ChildID); input.AmountCents > left {
		return parentbilling.BillingDocument{}, validation.Errors{"amount_cents": fmt.Errorf("is more than the %d cent left", left)}
	}
	return b.issueCorrection(ctx, reviewer, target, creditOf(target.invoice, child, input), runEventCredit)
}

// correctable holds the released run and the invoices of its period and
// reads the invoice with the documents that reference it. A cancelled
// invoice is neither cancelled nor credited again.
func (b *BillingLetters) correctable(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, documentID int64) (correctionTarget, error) {
	if documentID <= 0 {
		return correctionTarget{}, parentbilling.ErrBillingDocumentNotFound
	}
	run, err := b.releasedRun(ctx, reviewer.OrganizationID, runID, true)
	if err != nil {
		return correctionTarget{}, err
	}
	if err := b.store.LockPeriodBilling(ctx, reviewer.OrganizationID, run.Service, run.HolidayPeriodID); err != nil {
		return correctionTarget{}, fmt.Errorf("parent billing: hold the invoices of the period: %w", err)
	}
	documents, err := b.store.RunDocuments(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return correctionTarget{}, fmt.Errorf("parent billing: read invoices: %w", err)
	}
	invoice, ok := findDocument(documents, documentID)
	if !ok || invoice.Kind != parentbilling.DocumentInvoice {
		return correctionTarget{}, parentbilling.ErrBillingDocumentNotFound
	}
	history, err := b.store.ReferencingDocuments(ctx, reviewer.OrganizationID, []int64{invoice.ID})
	if err != nil {
		return correctionTarget{}, fmt.Errorf("parent billing: read the history of the invoice: %w", err)
	}
	target := correctionTarget{run: run, invoice: invoice, history: history}
	if target.has(parentbilling.DocumentCancellation) {
		return correctionTarget{}, parentbilling.ErrInvoiceCancelled
	}
	return target, nil
}

// issueCorrection numbers the document from the number range of the
// service, stores it and, once the run has its letters, its letter.
func (b *BillingLetters) issueCorrection(ctx context.Context, reviewer parentbilling.HolidayReviewer, target correctionTarget,
	document parentbilling.BillingDocument, action string,
) (parentbilling.BillingDocument, error) {
	organizationID, run := reviewer.OrganizationID, target.run
	pattern, err := numberPatternOf(ctx, b.store, organizationID, run.Service)
	if err != nil {
		return parentbilling.BillingDocument{}, err
	}
	issuedOn := b.today()
	year := numberYear(pattern, issuedOn.Year())
	sequence, err := b.store.ReserveNumbers(ctx, organizationID, run.Service, year, 1)
	if err != nil {
		return parentbilling.BillingDocument{}, fmt.Errorf("parent billing: reserve a number: %w", err)
	}
	document.Number, document.IssuedOn = formatInvoiceNumber(pattern, issuedOn.Year(), sequence), &issuedOn
	if document.ID, err = b.store.IssueDocument(ctx, organizationID, run, year, sequence, document); err != nil {
		return parentbilling.BillingDocument{}, err
	}
	if err := b.correctionLetter(ctx, reviewer, target, document); err != nil {
		return parentbilling.BillingDocument{}, err
	}
	if err := b.store.InsertRunEvent(ctx, organizationID, run.ID, reviewer.AccountID, action,
		map[string]any{"document_id": document.ID, "invoice_id": target.invoice.ID, "total_cents": document.TotalCents}); err != nil {
		return parentbilling.BillingDocument{}, fmt.Errorf("parent billing: record %s: %w", action, err)
	}
	document.History = []parentbilling.BillingDocumentLink{}
	return document, nil
}

// correctionLetter renders and stores the letter of a cancellation or a
// credit once the run has its letters. Before, the letters of the run
// render it with the invoices.
func (b *BillingLetters) correctionLetter(ctx context.Context, reviewer parentbilling.HolidayReviewer, target correctionTarget, document parentbilling.BillingDocument) error {
	letters, err := b.store.RunLetters(ctx, reviewer.OrganizationID, target.run.ID)
	if err != nil {
		return fmt.Errorf("parent billing: read letters: %w", err)
	}
	if letters == nil {
		return nil
	}
	head, err := b.letterheads.Letterhead(ctx, reviewer)
	if err != nil {
		return fmt.Errorf("parent billing: read letterhead: %w", err)
	}
	file, err := b.render(letterContent{
		run: target.run, document: document, letterhead: head, invoice: &target.invoice,
		leftCents: leftOf(target.invoice, append(append([]parentbilling.BillingDocument{}, target.history...), document)),
	})
	if err != nil {
		return err
	}
	if err := b.store.InsertDocumentLetter(ctx, reviewer.OrganizationID, target.run.ID, document.ID, file); err != nil {
		return fmt.Errorf("parent billing: store letter: %w", err)
	}
	return nil
}

// cancellationOf is the cancellation of an invoice: the copy of its
// recipient and every position negated, so it sums to minus the invoice.
func cancellationOf(invoice parentbilling.BillingDocument, reason string) parentbilling.BillingDocument {
	cancellation := correctionOf(invoice, parentbilling.DocumentCancellation, reason)
	for _, child := range invoice.Children {
		negated := parentbilling.BillingChild{
			ChildID: child.ChildID, FirstName: child.FirstName, LastName: child.LastName,
			Positions: make([]parentbilling.BillingPosition, len(child.Positions)), Corrections: []parentbilling.BillingCorrection{},
		}
		for i, position := range child.Positions {
			position.AmountCents = -position.AmountCents
			negated.Positions[i] = position
			negated.TotalCents += position.AmountCents
		}
		cancellation.Children = append(cancellation.Children, negated)
		cancellation.TotalCents += negated.TotalCents
	}
	return cancellation
}

// creditOf is the credit of one child of an invoice: one position with the
// reason as its note.
func creditOf(invoice parentbilling.BillingDocument, child parentbilling.BillingChild, input parentbilling.BillingCreditInput) parentbilling.BillingDocument {
	credit := correctionOf(invoice, parentbilling.DocumentCredit, input.Reason)
	credit.Children = []parentbilling.BillingChild{{
		ChildID: child.ChildID, FirstName: child.FirstName, LastName: child.LastName,
		Positions:   []parentbilling.BillingPosition{{Kind: parentbilling.LineCredit, AmountCents: -input.AmountCents, Note: input.Reason}},
		Corrections: []parentbilling.BillingCorrection{}, TotalCents: -input.AmountCents,
	}}
	credit.TotalCents = -input.AmountCents
	return credit
}

// correctionOf copies what a document of an invoice keeps of it: the
// registration, the recipient as the invoice has it and the service days.
func correctionOf(invoice parentbilling.BillingDocument, kind, reason string) parentbilling.BillingDocument {
	reference := invoice.ID
	return parentbilling.BillingDocument{
		Kind: kind, ReferenceDocumentID: &reference, ReferenceNumber: invoice.Number, Reason: reason,
		RegistrationID: invoice.RegistrationID, RecipientFirstName: invoice.RecipientFirstName,
		RecipientLastName: invoice.RecipientLastName, RecipientEmail: invoice.RecipientEmail,
		RecipientAddress: invoice.RecipientAddress, DebtorNumber: invoice.DebtorNumber,
		ServiceFrom: invoice.ServiceFrom, ServiceUntil: invoice.ServiceUntil, Children: []parentbilling.BillingChild{},
	}
}

func findChild(document parentbilling.BillingDocument, childID int64) (parentbilling.BillingChild, bool) {
	for _, child := range document.Children {
		if child.ChildID == childID {
			return child, true
		}
	}
	return parentbilling.BillingChild{}, false
}

// creditedCents sums the credits of one child among the documents of an
// invoice; the sum is negative.
func creditedCents(history []parentbilling.BillingDocument, childID int64) int64 {
	var credited int64
	for _, document := range history {
		if document.Kind != parentbilling.DocumentCredit {
			continue
		}
		for _, child := range document.Children {
			if child.ChildID == childID {
				credited += child.TotalCents
			}
		}
	}
	return credited
}

// leftOf is what the invoice asks for after its cancellation and credits
// among documents.
func leftOf(invoice parentbilling.BillingDocument, documents []parentbilling.BillingDocument) int64 {
	left := invoice.TotalCents
	for _, document := range documents {
		if document.Kind != parentbilling.DocumentInvoice && document.ReferenceDocumentID != nil && *document.ReferenceDocumentID == invoice.ID {
			left += document.TotalCents
		}
	}
	return left
}

// withDocumentHistory names on every issued invoice its cancellation, its
// credits and the invoice that replaces it; every document gets a list.
func withDocumentHistory(ctx context.Context, store documentHistoryReader, organizationID int64, documents []parentbilling.BillingDocument) ([]parentbilling.BillingDocument, error) {
	ids := make([]int64, 0, len(documents))
	for i := range documents {
		documents[i].History = []parentbilling.BillingDocumentLink{}
		if documents[i].Kind == parentbilling.DocumentInvoice {
			ids = append(ids, documents[i].ID)
		}
	}
	referencing, err := store.ReferencingDocuments(ctx, organizationID, ids)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the history of the invoices: %w", err)
	}
	byInvoice := map[int64][]parentbilling.BillingDocumentLink{}
	for _, document := range referencing {
		if document.ReferenceDocumentID == nil || document.IssuedOn == nil {
			continue
		}
		byInvoice[*document.ReferenceDocumentID] = append(byInvoice[*document.ReferenceDocumentID], parentbilling.BillingDocumentLink{
			ID: document.ID, RunID: document.RunID, Kind: document.Kind, Number: document.Number,
			IssuedOn: *document.IssuedOn, TotalCents: document.TotalCents, Reason: document.Reason,
		})
	}
	for i := range documents {
		if links, ok := byInvoice[documents[i].ID]; ok {
			documents[i].History = links
		}
	}
	return documents, nil
}

// normalizeDocumentReason trims the reason of a cancellation or a credit,
// which is required.
func normalizeDocumentReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	switch {
	case reason == "":
		return "", errors.New("is required")
	case len([]rune(reason)) > parentbilling.MaxDocumentReasonLength:
		return "", errors.New("is too long")
	}
	return reason, nil
}

// normalizeCredit checks a credit: a child, an amount of at least one cent
// and a reason.
func normalizeCredit(input parentbilling.BillingCreditInput) (parentbilling.BillingCreditInput, error) {
	errs := validation.Errors{}
	if input.ChildID <= 0 {
		errs["child_id"] = errors.New("is required")
	}
	if input.AmountCents <= 0 || input.AmountCents > parentbilling.MaxDiscountCents {
		errs["amount_cents"] = errors.New("must be an amount in cent, more than zero")
	}
	reason, err := normalizeDocumentReason(input.Reason)
	if err != nil {
		errs["reason"] = err
	}
	input.Reason = reason
	if len(errs) > 0 {
		return input, errs
	}
	return input, nil
}
