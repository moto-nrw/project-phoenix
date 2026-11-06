package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// FamilyStore reads the issued documents of the families of a Träger in its
// Träger transaction and records the reads in the trails of their runs.
type FamilyStore interface {
	// FamilyDocuments reads the documents of the family of the document
	// familyID, or of every family when familyID is zero, the oldest first.
	FamilyDocuments(ctx context.Context, organizationID, familyID int64) ([]parentbilling.BillingFamilyDocument, error)
	// InsertRunEvents appends one entry to the trail of each run.
	InsertRunEvents(ctx context.Context, organizationID, accountID int64, action string, details map[int64]map[string]any) error
}

// BillingFamilies serves parentbilling.BillingFamilies (#4691).
type BillingFamilies struct {
	store FamilyStore
}

var _ parentbilling.BillingFamilies = (*BillingFamilies)(nil)

// NewBillingFamilies builds the view of the families.
func NewBillingFamilies(store FamilyStore) *BillingFamilies {
	if store == nil {
		panic("parent billing: the store is required")
	}
	return &BillingFamilies{store: store}
}

// ListBillingFamilies lists the families of the Träger by name and records
// the read in the trail of every run whose documents the list sums.
func (f *BillingFamilies) ListBillingFamilies(ctx context.Context, reviewer parentbilling.HolidayReviewer) ([]parentbilling.BillingFamily, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	documents, err := f.store.FamilyDocuments(ctx, reviewer.OrganizationID, 0)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read family documents: %w", err)
	}
	if err := f.recordRead(ctx, reviewer, documents, map[string]any{"view": "families"}); err != nil {
		return nil, err
	}
	details := familiesOf(documents)
	families := make([]parentbilling.BillingFamily, len(details))
	for i, detail := range details {
		families[i] = detail.BillingFamily
	}
	slices.SortStableFunc(families, func(a, b parentbilling.BillingFamily) int {
		for _, order := range []int{
			strings.Compare(strings.ToLower(a.RecipientLastName), strings.ToLower(b.RecipientLastName)),
			strings.Compare(strings.ToLower(a.RecipientFirstName), strings.ToLower(b.RecipientFirstName)),
		} {
			if order != 0 {
				return order
			}
		}
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	return families, nil
}

// BillingFamily reads one family with its documents, the newest first, and
// records the read in the trail of each of their runs.
func (f *BillingFamilies) BillingFamily(ctx context.Context, reviewer parentbilling.HolidayReviewer, familyID int64) (parentbilling.BillingFamilyDetail, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BillingFamilyDetail{}, err
	}
	if familyID <= 0 {
		return parentbilling.BillingFamilyDetail{}, parentbilling.ErrFamilyNotFound
	}
	documents, err := f.store.FamilyDocuments(ctx, reviewer.OrganizationID, familyID)
	if err != nil {
		return parentbilling.BillingFamilyDetail{}, fmt.Errorf("parent billing: read family documents: %w", err)
	}
	families := familiesOf(documents)
	if len(families) != 1 {
		return parentbilling.BillingFamilyDetail{}, parentbilling.ErrFamilyNotFound
	}
	if err := f.recordRead(ctx, reviewer, documents, map[string]any{"view": "family", "family_id": families[0].ID}); err != nil {
		return parentbilling.BillingFamilyDetail{}, err
	}
	family := families[0]
	slices.Reverse(family.Documents)
	return family, nil
}

// recordRead records in the trail of each run which of its documents the
// reader saw.
func (f *BillingFamilies) recordRead(ctx context.Context, reviewer parentbilling.HolidayReviewer, documents []parentbilling.BillingFamilyDocument, details map[string]any) error {
	byRun := map[int64][]int64{}
	for _, document := range documents {
		byRun[document.RunID] = append(byRun[document.RunID], document.ID)
	}
	events := make(map[int64]map[string]any, len(byRun))
	for runID, ids := range byRun {
		event := map[string]any{"document_ids": ids}
		for key, value := range details {
			event[key] = value
		}
		events[runID] = event
	}
	if err := f.store.InsertRunEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, runEventRead, events); err != nil {
		return fmt.Errorf("parent billing: record family read: %w", err)
	}
	return nil
}

// familiesOf groups documents, the oldest first, into families in the order
// of their first document. Each invoice gets who pays it, its state and
// what it still asks for after its cancellation and credits.
func familiesOf(documents []parentbilling.BillingFamilyDocument) []parentbilling.BillingFamilyDetail {
	cancelled := map[int64]bool{}
	credited := map[int64]int64{}
	for _, document := range documents {
		if document.ReferenceDocumentID == nil {
			continue
		}
		switch document.Kind {
		case parentbilling.DocumentCancellation:
			cancelled[*document.ReferenceDocumentID] = true
		case parentbilling.DocumentCredit:
			credited[*document.ReferenceDocumentID] -= document.TotalCents
		}
	}
	var families []parentbilling.BillingFamilyDetail
	index := map[string]int{}
	for _, document := range documents {
		at, ok := index[document.FamilyKey]
		if !ok {
			at = len(families)
			index[document.FamilyKey] = at
			families = append(families, parentbilling.BillingFamilyDetail{
				BillingFamily: parentbilling.BillingFamily{ID: document.ID, DebtorNumbers: []string{}, Payments: []parentbilling.BillingPayment{}},
				Documents:     []parentbilling.BillingFamilyDocument{},
			})
		}
		if document.Kind == parentbilling.DocumentInvoice {
			document = invoiceState(document, cancelled[document.ID], credited[document.ID])
		}
		families[at].Documents = append(families[at].Documents, document)
	}
	for i := range families {
		sumFamily(&families[i])
	}
	return families
}

// invoiceState sets who pays an invoice, its state and what it still asks
// for: nothing once it is cancelled, otherwise its amount less its credits.
func invoiceState(invoice parentbilling.BillingFamilyDocument, cancelled bool, creditedCents int64) parentbilling.BillingFamilyDocument {
	payment := paymentOf(invoice)
	invoice.Payment = &payment
	switch {
	case cancelled:
		invoice.Status = parentbilling.InvoiceCancelled
	case invoice.TotalCents <= 0:
		invoice.Status = parentbilling.InvoiceNothingDue
	case creditedCents >= invoice.TotalCents:
		invoice.Status = parentbilling.InvoiceCredited
	case creditedCents > 0:
		invoice.Status, invoice.OpenCents = parentbilling.InvoicePartlyCredited, invoice.TotalCents-creditedCents
	default:
		invoice.Status, invoice.OpenCents = parentbilling.InvoiceOpen, invoice.TotalCents
	}
	return invoice
}

// paymentOf is who pays an invoice: a cost bearer its positions name, no
// one when the exemption freed it, the family otherwise.
func paymentOf(invoice parentbilling.BillingFamilyDocument) parentbilling.BillingPayment {
	switch {
	case len(invoice.CostBearerNames) > 0:
		return parentbilling.BillingPayment{Kind: parentbilling.PaymentCostBearer, CostBearerName: strings.Join(invoice.CostBearerNames, ", ")}
	case invoice.Exempt && invoice.TotalCents <= 0:
		return parentbilling.BillingPayment{Kind: parentbilling.PaymentExempt}
	}
	return parentbilling.BillingPayment{Kind: parentbilling.PaymentFamily}
}

// sumFamily sets the recipient of the latest document, the debtor numbers,
// the ways the family's invoices are paid, the counts and the sums.
func sumFamily(family *parentbilling.BillingFamilyDetail) {
	latest := family.Documents[len(family.Documents)-1]
	family.RecipientFirstName, family.RecipientLastName, family.RecipientEmail =
		latest.RecipientFirstName, latest.RecipientLastName, latest.RecipientEmail
	family.DocumentCount = len(family.Documents)
	for _, document := range family.Documents {
		if document.IssuedOn.After(family.LastIssuedOn) || family.LastIssuedOn.IsZero() {
			family.LastIssuedOn = document.IssuedOn
		}
		if document.DebtorNumber != "" && !slices.Contains(family.DebtorNumbers, document.DebtorNumber) {
			family.DebtorNumbers = append(family.DebtorNumbers, document.DebtorNumber)
		}
		switch document.Kind {
		case parentbilling.DocumentInvoice:
			family.InvoiceCount++
			family.IssuedCents += document.TotalCents
			family.OpenCents += document.OpenCents
			if document.OpenCents > 0 {
				family.OpenInvoiceCount++
			}
			if document.Status != parentbilling.InvoiceCancelled && !slices.Contains(family.Payments, *document.Payment) {
				family.Payments = append(family.Payments, *document.Payment)
			}
		case parentbilling.DocumentCancellation:
			family.CancelledCents -= document.TotalCents
		case parentbilling.DocumentCredit:
			family.CreditedCents -= document.TotalCents
		}
	}
	slices.Sort(family.DebtorNumbers)
	slices.SortFunc(family.Payments, func(a, b parentbilling.BillingPayment) int {
		if order := strings.Compare(paymentOrder(a.Kind), paymentOrder(b.Kind)); order != 0 {
			return order
		}
		return strings.Compare(a.CostBearerName, b.CostBearerName)
	})
}

// paymentOrder lists the family first, then the cost bearers, then the
// exemption.
func paymentOrder(kind string) string {
	switch kind {
	case parentbilling.PaymentFamily:
		return "1"
	case parentbilling.PaymentCostBearer:
		return "2"
	}
	return "3"
}
