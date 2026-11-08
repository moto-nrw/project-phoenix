package application

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The changes of single invoices in the review of a monthly run (#6197,
// ADR 0119 addendum 2026-11-08 "Änderungen je Rechnung"). The preview is
// computed as before; afterwards the entries of the run, preceded by the
// corrections the cancelled invoice of the month carries over, are applied
// to their invoice in the order they were made. Nothing of the preview is
// stored; the entries are the only state.

// CareAmendmentStore keeps the changes of the review and the corrections an
// issued invoice copied.
type CareAmendmentStore interface {
	// CareRunAmendments reads every entry of a run in the order it was made.
	CareRunAmendments(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareAmendmentEntry, error)
	InsertCareRunAmendment(ctx context.Context, organizationID, runID int64, entry parentbilling.CareAmendmentEntry) error
	// CarriedCareCorrections reads per invoice of the month the corrections
	// of its cancelled invoice no invoice replaced yet (ADR 0119 addendum
	// 2026-11-04): the next invoice proposes them.
	CarriedCareCorrections(ctx context.Context, organizationID int64, month calendar.Date) (map[parentbilling.CareInvoiceKey][]parentbilling.CareAmendment, error)
}

// Labels of the correction lines the letter and the review list show.
const (
	amendAmountLabel  = "Korrektur des Betrags"
	amendRemovePrefix = "Entfällt: "
)

// careInvoiceKeyOf names the invoice of the month a preview invoice is.
func careInvoiceKeyOf(invoice parentbilling.CareInvoice) parentbilling.CareInvoiceKey {
	service := invoice.Service
	if service == "" {
		service = parentbilling.ServiceCare
	}
	return parentbilling.CareInvoiceKey{ContractID: invoice.ContractID, Service: service}
}

// keyPositions names every position of an invoice for a removal: a module
// by its offering, contract version and first day, a Mensa month by child
// and month.
func keyPositions(invoice *parentbilling.CareInvoice) {
	for i := range invoice.Positions {
		position := &invoice.Positions[i]
		position.Key = "care:" + strconv.FormatInt(position.CareOfferingID, 10) + ":" +
			strconv.Itoa(position.VersionNumber) + ":" + position.ServiceFrom.String()
	}
	for i := range invoice.MealPositions {
		meal := &invoice.MealPositions[i]
		meal.Key = "meal:" + strconv.FormatInt(meal.StudentID, 10) + ":" + meal.MealMonth.String()
	}
}

// keyIssuedPositions gives the positions of issued invoices the keys the
// review named them by, so a removed position still shows as removed.
func keyIssuedPositions(invoices []parentbilling.CareInvoice) {
	for i := range invoices {
		keyPositions(&invoices[i])
	}
}

// removablePosition is what taking a position off costs the family: the
// module after the reductions of the chain, a Mensa month in full.
func removablePosition(invoice parentbilling.CareInvoice, key string) (string, int64, bool) {
	for _, position := range invoice.Positions {
		if position.Key == key {
			return position.ModuleName, position.PayableCents, true
		}
	}
	for _, meal := range invoice.MealPositions {
		if meal.Key == key {
			return mealPositionLabel(meal), meal.AmountCents, true
		}
	}
	return "", 0, false
}

// careAmendments are the entries of a run and the carried corrections, by
// invoice, with the revocations they received.
type careAmendments struct {
	byInvoice map[parentbilling.CareInvoiceKey][]parentbilling.CareAmendment
	carried   map[parentbilling.CareInvoiceKey][]parentbilling.CareAmendment
}

// readAmendments reads the entries of a run and the corrections the month
// carries, and marks each change its revocation took back.
func (c *CareBilling) readAmendments(ctx context.Context, organizationID int64, run parentbilling.CareRun) (careAmendments, error) {
	entries, err := c.store.CareRunAmendments(ctx, organizationID, run.ID)
	if err != nil {
		return careAmendments{}, err
	}
	carried, err := c.store.CarriedCareCorrections(ctx, organizationID, run.BillingMonth)
	if err != nil {
		return careAmendments{}, err
	}
	revokedRun, revokedCarried := map[int64]*parentbilling.CareRevocation{}, map[int64]*parentbilling.CareRevocation{}
	for _, entry := range entries {
		if entry.Kind != parentbilling.AmendRevoke {
			continue
		}
		revocation := &parentbilling.CareRevocation{Reason: entry.Reason, AccountID: entry.AccountID, RevokedAt: entry.AmendedAt}
		if entry.RevokesID != nil {
			revokedRun[*entry.RevokesID] = revocation
		} else if entry.RevokesCorrectionID != nil {
			revokedCarried[*entry.RevokesCorrectionID] = revocation
		}
	}
	result := careAmendments{byInvoice: map[parentbilling.CareInvoiceKey][]parentbilling.CareAmendment{}, carried: carried}
	for key, list := range carried {
		for i := range list {
			list[i].Revocation = revokedCarried[list[i].ID]
		}
		result.carried[key] = list
	}
	for _, entry := range entries {
		if entry.Kind == parentbilling.AmendRevoke {
			continue
		}
		result.byInvoice[entry.Invoice] = append(result.byInvoice[entry.Invoice], parentbilling.CareAmendment{
			ID: entry.ID, Kind: entry.Kind, Label: entry.Label, AmountCents: entry.AmountCents, PositionKey: entry.PositionKey,
			Before: entry.Before, After: entry.After, Reason: entry.Reason, AccountID: entry.AccountID, AmendedAt: entry.AmendedAt,
			Revocation: revokedRun[entry.ID],
		})
	}
	return result, nil
}

// apply applies the carried corrections and then the changes of the run to
// an invoice of the preview, in their order, and writes its history.
func (a careAmendments) apply(invoice *parentbilling.CareInvoice) {
	keyPositions(invoice)
	key := careInvoiceKeyOf(*invoice)
	history := slices.Concat(a.carried[key], a.byInvoice[key])
	removed := map[string]bool{}
	for i := range history {
		amendment := &history[i]
		if amendment.Revocation != nil {
			continue
		}
		applyAmendment(invoice, amendment, removed)
	}
	invoice.Amendments = history
	if invoice.Amendments == nil {
		invoice.Amendments = []parentbilling.CareAmendment{}
	}
}

// applyAmendment applies one change in force to the invoice, or marks the
// conflict that keeps it out.
func applyAmendment(invoice *parentbilling.CareInvoice, amendment *parentbilling.CareAmendment, removed map[string]bool) {
	switch amendment.Kind {
	case parentbilling.AmendAmount:
		// A change of the run corrects the total it saw; a carried one
		// proposes the difference the cancelled invoice had.
		if !amendment.Carried && (amendment.Before.TotalCents == nil || *amendment.Before.TotalCents != invoice.TotalCents) {
			amendment.Conflict = parentbilling.AmendConflictBasis
			return
		}
	case parentbilling.AmendRemovePosition:
		label, cents, ok := removablePosition(*invoice, amendment.PositionKey)
		if !ok || removed[amendment.PositionKey] {
			amendment.Conflict = parentbilling.AmendConflictPosition
			return
		}
		amendment.Label, amendment.AmountCents = amendRemovePrefix+label, -cents
	case parentbilling.AmendRecipient:
		applyRecipient(invoice, amendment)
		return
	case parentbilling.AmendAddress:
		applyAddress(invoice, amendment)
		return
	}
	if invoice.TotalCents+amendment.AmountCents < 0 {
		amendment.Conflict = parentbilling.AmendConflictNegative
		return
	}
	invoice.TotalCents += amendment.AmountCents
	if amendment.Kind == parentbilling.AmendRemovePosition {
		removed[amendment.PositionKey] = true
	}
}

// applyRecipient sets the name and mail address of a change. The changes
// of an invoice resolve to their end state: every change in force applies
// in its order, whatever recipient it was made on, so revoking an earlier
// change leaves the later one in force (#6197). The recipient it actually
// replaced is kept for the release, which stores it, so the send checks
// the very chain the issue applied.
func applyRecipient(invoice *parentbilling.CareInvoice, amendment *parentbilling.CareAmendment) {
	after := amendment.After.Recipient
	if after == nil {
		amendment.Conflict = parentbilling.AmendConflictBasis
		return
	}
	replaced := parentbilling.CareRecipientName{FirstName: invoice.Recipient.FirstName, LastName: invoice.Recipient.LastName,
		SecondName: invoice.Recipient.SecondName, Email: invoice.Recipient.Email}
	amendment.Replaced = parentbilling.CareAmendmentValue{Recipient: &replaced}
	invoice.Recipient.FirstName, invoice.Recipient.LastName = after.FirstName, after.LastName
	invoice.Recipient.SecondName, invoice.Recipient.Email = after.SecondName, after.Email
}

// applyAddress sets the postal address of a change, resolved to the end
// state like applyRecipient.
func applyAddress(invoice *parentbilling.CareInvoice, amendment *parentbilling.CareAmendment) {
	after := amendment.After.Address
	if after == nil {
		amendment.Conflict = parentbilling.AmendConflictBasis
		return
	}
	amendment.Replaced = parentbilling.CareAmendmentValue{Address: invoice.Recipient.Address}
	copied := *after
	invoice.Recipient.Address = &copied
}

// amendmentConflicts counts the changes in conflict on the invoices a
// release issues.
func amendmentConflicts(invoices []parentbilling.CareInvoice) int {
	count := 0
	for _, invoice := range invoices {
		for _, amendment := range invoice.Amendments {
			if amendment.Revocation == nil && amendment.Conflict != "" {
				count++
			}
		}
	}
	return count
}

// amendReviewed applies the changes to every invoice of a review, those the
// release issues and the excluded ones.
func (c *CareBilling) amendReviewed(ctx context.Context, organizationID int64, run parentbilling.CareRun, review *reviewedMonth) error {
	amendments, err := c.readAmendments(ctx, organizationID, run)
	if err != nil {
		return fmt.Errorf("parent billing: read the changes of the review: %w", err)
	}
	for i := range review.invoices {
		amendments.apply(&review.invoices[i])
	}
	for i := range review.excluded {
		amendments.apply(&review.excluded[i])
	}
	review.conflicts = amendmentConflicts(review.invoices)
	return nil
}
