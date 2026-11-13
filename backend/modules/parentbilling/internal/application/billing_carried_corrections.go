package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// The corrections a reissued invoice carries over (#5663, ADR 0119 addendum).
// A calculated run stores no drafts, so a proposal is no row: the preview
// reads the correction positions of the cancelled invoice it replaces, leaves
// out those the run dismissed and applies the others before the corrections
// of the run. Its credits are only named: they are documents of their own.

// replacedInvoices is what a preview takes from the cancelled invoices it
// replaces, by registration.
type replacedInvoices struct {
	corrections map[int64][]parentbilling.BillingCarriedCorrection
	credits     map[int64][]parentbilling.BillingDocumentLink
}

// replacedInvoices reads the proposals and the credits of the cancelled
// invoices. It reads nothing when the run replaces none.
func (b *BillingRuns) replacedInvoices(ctx context.Context, organizationID, runID int64, replaces map[int64]parentbilling.BillingDocumentLink) (replacedInvoices, error) {
	if len(replaces) == 0 {
		return replacedInvoices{}, nil
	}
	invoiceIDs := make([]int64, 0, len(replaces))
	registrationOf := make(map[int64]int64, len(replaces))
	for registrationID, invoice := range replaces {
		invoiceIDs = append(invoiceIDs, invoice.ID)
		registrationOf[invoice.ID] = registrationID
	}
	positions, err := b.store.InvoiceCorrections(ctx, organizationID, invoiceIDs)
	if err != nil {
		return replacedInvoices{}, fmt.Errorf("parent billing: read the corrections of the cancelled invoices: %w", err)
	}
	dismissed, err := b.store.RunCorrectionDismissals(ctx, organizationID, runID)
	if err != nil {
		return replacedInvoices{}, fmt.Errorf("parent billing: read the removed proposals: %w", err)
	}
	referencing, err := b.store.ReferencingDocuments(ctx, organizationID, invoiceIDs)
	if err != nil {
		return replacedInvoices{}, fmt.Errorf("parent billing: read the credits of the cancelled invoices: %w", err)
	}
	result := replacedInvoices{
		corrections: map[int64][]parentbilling.BillingCarriedCorrection{},
		credits:     map[int64][]parentbilling.BillingDocumentLink{},
	}
	for _, position := range positions {
		if dismissed[position.PositionID] {
			continue
		}
		registrationID := registrationOf[position.SourceDocumentID]
		position.SourceNumber = replaces[registrationID].Number
		result.corrections[registrationID] = append(result.corrections[registrationID], position)
	}
	for _, document := range referencing {
		if document.Kind != parentbilling.DocumentCredit || document.ReferenceDocumentID == nil || document.IssuedOn == nil {
			continue
		}
		registrationID := registrationOf[*document.ReferenceDocumentID]
		result.credits[registrationID] = append(result.credits[registrationID], parentbilling.BillingDocumentLink{
			ID: document.ID, RunID: document.RunID, Kind: document.Kind, Number: document.Number,
			IssuedOn: *document.IssuedOn, TotalCents: document.TotalCents, Reason: document.Reason,
		})
	}
	return result, nil
}

// withCarriedCorrections applies the proposals of one child after its frozen
// lines. Each keeps its amount, but lowers the child at most to zero: the
// contribution can be lower now than on the cancelled invoice, and an invoice
// is never negative. A proposal that stops at zero stays visible without a
// position.
func withCarriedCorrections(child parentbilling.BillingChild, carried []parentbilling.BillingCarriedCorrection) parentbilling.BillingChild {
	child.CarriedCorrections = make([]parentbilling.BillingCarriedCorrection, 0, len(carried))
	for _, correction := range carried {
		correction.TotalBeforeCents = child.TotalCents
		correction.AmountCents = max(correction.SourceAmountCents, -child.TotalCents)
		if correction.AmountCents != 0 {
			child.Positions = append(child.Positions, parentbilling.BillingPosition{
				Kind: parentbilling.LineCorrection, AmountCents: correction.AmountCents, Note: correction.Reason,
			})
			child.TotalCents += correction.AmountCents
		}
		child.CarriedCorrections = append(child.CarriedCorrections, correction)
	}
	return child
}

// carriedCorrection finds a proposal in the preview with its family and child.
func (p preview) carriedCorrection(positionID int64) (int64, parentbilling.BillingChild, bool) {
	for _, document := range p.documents {
		for _, child := range document.Children {
			for _, correction := range child.CarriedCorrections {
				if correction.PositionID == positionID {
					return document.RegistrationID, child, true
				}
			}
		}
	}
	return 0, parentbilling.BillingChild{}, false
}

// RemoveCarriedCorrection stops the preview from proposing a correction, so
// the release issues the amount without it. It refuses when the corrections
// of the run would then leave the child below zero.
func (b *BillingRuns) RemoveCarriedCorrection(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, positionID int64) (parentbilling.BillingRunDetail, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	run, err := b.lockCalculated(ctx, reviewer.OrganizationID, runID)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	registrationID, child, err := b.dismissCarried(ctx, reviewer, run, positionID)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	if child.TotalCents < 0 {
		return parentbilling.BillingRunDetail{}, validation.Errors{"total_cents": errors.New("would leave the child below zero")}
	}
	if err := b.recordCarriedChange(ctx, reviewer, run.ID, registrationID, child.ChildID, positionID, "remove"); err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	return b.detail(ctx, reviewer, run)
}

// ChangeCarriedCorrection replaces a proposal with a correction of the run:
// the new amount of the child with its reason. The new amount has to differ
// from the amount without the proposal; that is what removing it does.
func (b *BillingRuns) ChangeCarriedCorrection(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, positionID int64, change parentbilling.BillingCarriedCorrectionChange) (parentbilling.BillingRunDetail, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	reason := strings.TrimSpace(change.Reason)
	if errs := correctionAmountErrors(change.TotalCents, reason); len(errs) > 0 {
		return parentbilling.BillingRunDetail{}, errs
	}
	run, err := b.lockCalculated(ctx, reviewer.OrganizationID, runID)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	registrationID, child, err := b.dismissCarried(ctx, reviewer, run, positionID)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	if change.TotalCents == child.TotalCents {
		return parentbilling.BillingRunDetail{}, validation.Errors{"total_cents": errors.New("is the amount without the correction")}
	}
	correction := parentbilling.BillingCorrection{
		ChildID: child.ChildID, AmountCents: change.TotalCents - child.TotalCents, TotalBeforeCents: child.TotalCents,
		TotalAfterCents: change.TotalCents, Reason: reason,
	}
	if err := b.store.InsertRunCorrection(ctx, reviewer.OrganizationID, run, registrationID, reviewer.AccountID, correction); err != nil {
		return parentbilling.BillingRunDetail{}, fmt.Errorf("parent billing: store correction: %w", err)
	}
	if err := b.recordCarriedChange(ctx, reviewer, run.ID, registrationID, child.ChildID, positionID, "change"); err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	return b.detail(ctx, reviewer, run)
}

// dismissCarried stops the run from proposing a correction and returns the
// family and the child as the preview then has them. The caller rolls back
// when it refuses what it finds.
func (b *BillingRuns) dismissCarried(ctx context.Context, reviewer parentbilling.HolidayReviewer, run parentbilling.BillingRun, positionID int64) (int64, parentbilling.BillingChild, error) {
	current, err := b.preview(ctx, reviewer, run)
	if err != nil {
		return 0, parentbilling.BillingChild{}, err
	}
	registrationID, child, ok := current.carriedCorrection(positionID)
	if !ok {
		return 0, parentbilling.BillingChild{}, parentbilling.ErrCarriedCorrectionNotFound
	}
	if err := b.store.InsertRunCorrectionDismissal(ctx, reviewer.OrganizationID, run.ID, positionID, reviewer.AccountID); err != nil {
		return 0, parentbilling.BillingChild{}, fmt.Errorf("parent billing: remove the proposal: %w", err)
	}
	current, err = b.preview(ctx, reviewer, run)
	if err != nil {
		return 0, parentbilling.BillingChild{}, err
	}
	after, ok := current.child(registrationID, child.ChildID)
	if !ok {
		return 0, parentbilling.BillingChild{}, parentbilling.ErrBillingFamilyNotFound
	}
	return registrationID, after, nil
}

func (b *BillingRuns) recordCarriedChange(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, registrationID, childID, positionID int64, change string) error {
	if err := b.store.InsertRunEvent(ctx, reviewer.OrganizationID, runID, reviewer.AccountID, runEventCorrect, map[string]any{
		"registration_id": registrationID, "child_id": childID, "carried_position_id": positionID, "carried_change": change,
	}); err != nil {
		return fmt.Errorf("parent billing: record the change of the proposal: %w", err)
	}
	return nil
}

// recordCarriedCorrections adds what a release takes over to the corrections
// of the run, so the issued invoice keeps its history and origin. The release
// took these amounts from the draft; a proposal that stopped at zero has no
// position and no history.
func (b *BillingRuns) recordCarriedCorrections(ctx context.Context, reviewer parentbilling.HolidayReviewer, run parentbilling.BillingRun, documents []parentbilling.BillingDocument) (int, error) {
	count := 0
	for _, document := range documents {
		for _, child := range document.Children {
			for _, carried := range child.CarriedCorrections {
				if carried.AmountCents == 0 {
					continue
				}
				if err := b.store.InsertRunCorrection(ctx, reviewer.OrganizationID, run, document.RegistrationID, reviewer.AccountID, parentbilling.BillingCorrection{
					ChildID: child.ChildID, AmountCents: carried.AmountCents, TotalBeforeCents: carried.TotalBeforeCents,
					TotalAfterCents: carried.TotalBeforeCents + carried.AmountCents, Reason: carried.Reason, CarriedFromNumber: carried.SourceNumber,
				}); err != nil {
					return 0, fmt.Errorf("parent billing: store the carried correction: %w", err)
				}
				count++
			}
		}
	}
	return count, nil
}
