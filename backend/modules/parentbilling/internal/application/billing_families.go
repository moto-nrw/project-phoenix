package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// FamilyStore reads the issued documents of the families of a Träger in its
// Träger transaction and records the reads in the trails of their runs.
type FamilyStore interface {
	// FamilyDocuments reads the documents of the family of the document
	// familyID, or of every family when familyID is zero, the oldest first.
	FamilyDocuments(ctx context.Context, organizationID, familyID int64) ([]parentbilling.BillingFamilyDocument, error)
	// InsertRunEvents appends one entry to the trail of each holiday run and
	// of each monthly run, in one statement.
	InsertRunEvents(ctx context.Context, organizationID, accountID int64, action string, holiday, care map[int64]map[string]any) error
	// ReadListedFamilies records the read of the list in one statement: an
	// entry in the trail of each run (holiday, care) and in the trail of each
	// family row the list shows, which it answers: a row of a family with a
	// document (documentKeys), a row without a contract and a row with a
	// number (#6692).
	ReadListedFamilies(ctx context.Context, organizationID, accountID int64, holiday, care map[int64]map[string]any, documentKeys []string) ([]parentbilling.FamilyBilling, error)
	FamilyBillingOf(ctx context.Context, organizationID int64, familyEmail string) (*parentbilling.FamilyBilling, error)
	// FamilyHasDocuments reports whether a document of the Träger belongs
	// to the family.
	FamilyHasDocuments(ctx context.Context, organizationID int64, familyEmail string) (bool, error)
	// SetFamilyDebtorNumber sets the number of a family, with a row without
	// a contract for a family that has none.
	SetFamilyDebtorNumber(ctx context.Context, organizationID, accountID int64, familyEmail, number string, today calendar.Date) (parentbilling.FamilyBilling, error)
	InsertFamilyBillingEvents(ctx context.Context, organizationID, accountID int64, action string, families []parentbilling.FamilyBilling) error
}

// BillingFamilies serves parentbilling.BillingFamilies (#4691).
type BillingFamilies struct {
	store   FamilyStore
	formats parentbilling.DebtorNumberFormats
	// today is the Berlin day a number set in the list is needed from.
	today func() calendar.Date
}

var _ parentbilling.BillingFamilies = (*BillingFamilies)(nil)

// NewBillingFamilies builds the view of the families over the format of the
// Träger's debtor numbers (#6675); a nil today uses the Berlin day.
func NewBillingFamilies(store FamilyStore, formats parentbilling.DebtorNumberFormats, today func() calendar.Date) *BillingFamilies {
	if store == nil || formats == nil {
		panic("parent billing: the store and the debtor number format are required")
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &BillingFamilies{store: store, formats: formats, today: today}
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
	details := familiesOf(documents)
	keys := make([]string, len(details))
	for i, detail := range details {
		keys[i] = detail.FamilyEmail
	}
	holiday, care := readEvents(documents, map[string]any{"view": "families"})
	rows, err := f.store.ReadListedFamilies(ctx, reviewer.OrganizationID, reviewer.AccountID, holiday, care, keys)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the families and record the read: %w", err)
	}
	families := withFamilyRows(details, rows)
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
	if familyID == 0 {
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
	row, err := f.store.FamilyBillingOf(ctx, reviewer.OrganizationID, family.FamilyEmail)
	if err != nil {
		return parentbilling.BillingFamilyDetail{}, fmt.Errorf("parent billing: read the family's number: %w", err)
	}
	family.BillingFamily = withFamilyRow(family.BillingFamily, family.Documents, row)
	if row != nil {
		if err := f.store.InsertFamilyBillingEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingRead,
			[]parentbilling.FamilyBilling{*row}); err != nil {
			return parentbilling.BillingFamilyDetail{}, fmt.Errorf("parent billing: record the read of the family's number: %w", err)
		}
	}
	slices.Reverse(family.Documents)
	return family, nil
}

// recordRead records in the trail of each run which of its documents the
// reader saw.
func (f *BillingFamilies) recordRead(ctx context.Context, reviewer parentbilling.HolidayReviewer, documents []parentbilling.BillingFamilyDocument, details map[string]any) error {
	holiday, care := readEvents(documents, details)
	if err := f.store.InsertRunEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, runEventRead, holiday, care); err != nil {
		return fmt.Errorf("parent billing: record family read: %w", err)
	}
	return nil
}

// readEvents are the entries of the trails of the holiday runs and of the
// monthly runs that name which of their documents the reader saw.
func readEvents(documents []parentbilling.BillingFamilyDocument, details map[string]any) (holiday, care map[int64]map[string]any) {
	byRun := map[bool]map[int64][]int64{false: {}, true: {}}
	for _, document := range documents {
		care := document.Service == parentbilling.ServiceCare
		byRun[care][document.RunID] = append(byRun[care][document.RunID], document.ID)
	}
	events := func(runs map[int64][]int64) map[int64]map[string]any {
		result := make(map[int64]map[string]any, len(runs))
		for runID, ids := range runs {
			event := map[string]any{"document_ids": ids}
			for key, value := range details {
				event[key] = value
			}
			result[runID] = event
		}
		return result
	}
	return events(byRun[false]), events(byRun[true])
}

// familiesOf groups documents, the oldest first, into families in the order
// of their first document. Each invoice gets who pays it, its state and
// what it still asks for after its cancellation and credits.
func familiesOf(documents []parentbilling.BillingFamilyDocument) []parentbilling.BillingFamilyDetail {
	cancelled := map[documentKey]bool{}
	credited := map[documentKey]int64{}
	for _, document := range documents {
		if document.ReferenceDocumentID == nil {
			continue
		}
		reference := documentKey{service: document.Service, id: *document.ReferenceDocumentID}
		switch document.Kind {
		case parentbilling.DocumentCancellation:
			cancelled[reference] = true
		case parentbilling.DocumentCredit:
			credited[reference] -= document.TotalCents
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
				BillingFamily: parentbilling.BillingFamily{ID: familyID(document), DebtorNumbers: []string{}, Payments: []parentbilling.BillingPayment{},
					PaymentMethods: []string{}, FamilyEmail: document.FamilyKey},
				Documents: []parentbilling.BillingFamilyDocument{},
			})
		}
		if document.Service == parentbilling.ServiceCare && document.PeriodName == "" {
			document.PeriodName = "Monatsbeitrag " + careMonthName(document.ServiceFrom)
		}
		if document.Kind == parentbilling.DocumentInvoice {
			key := documentKey{service: document.Service, id: document.ID}
			document = invoiceState(document, cancelled[key], credited[key])
		}
		families[at].Documents = append(families[at].Documents, document)
	}
	for i := range families {
		sumFamily(&families[i])
	}
	return families
}

// documentKey names a document across the holiday documents and the monthly
// invoices, which count their IDs apart.
type documentKey struct {
	service string
	id      int64
}

// familyID is the ID of the family a document opens: the ID of a holiday
// document, the negated ID of a monthly invoice (#4689). The store reads a
// family by either.
func familyID(document parentbilling.BillingFamilyDocument) int64 {
	if document.Service == parentbilling.ServiceCare {
		return -document.ID
	}
	return document.ID
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
		noteDocument(family, document)
	}
	slices.Sort(family.DebtorNumbers)
	slices.SortFunc(family.Payments, func(a, b parentbilling.BillingPayment) int {
		if order := strings.Compare(paymentOrder(a.Kind), paymentOrder(b.Kind)); order != 0 {
			return order
		}
		return strings.Compare(a.CostBearerName, b.CostBearerName)
	})
}

// noteDocument adds one document to the date, the debtor numbers and the
// sums of its family.
func noteDocument(family *parentbilling.BillingFamilyDetail, document parentbilling.BillingFamilyDocument) {
	if family.LastIssuedOn.IsZero() || document.IssuedOn.After(family.LastIssuedOn) {
		family.LastIssuedOn = document.IssuedOn
	}
	if document.DebtorNumber != "" && !slices.Contains(family.DebtorNumbers, document.DebtorNumber) {
		family.DebtorNumbers = append(family.DebtorNumbers, document.DebtorNumber)
	}
	switch document.Kind {
	case parentbilling.DocumentInvoice:
		noteInvoice(family, document)
	case parentbilling.DocumentCancellation:
		family.CancelledCents -= document.TotalCents
	case parentbilling.DocumentCredit:
		family.CreditedCents -= document.TotalCents
	}
}

// noteInvoice adds an invoice to the counts and sums of its family and, unless
// it is cancelled, its payer to the ways the family pays.
func noteInvoice(family *parentbilling.BillingFamilyDetail, invoice parentbilling.BillingFamilyDocument) {
	family.InvoiceCount++
	family.IssuedCents += invoice.TotalCents
	family.OpenCents += invoice.OpenCents
	if invoice.OpenCents > 0 {
		family.OpenInvoiceCount++
	}
	if invoice.Status != parentbilling.InvoiceCancelled && !slices.Contains(family.Payments, *invoice.Payment) {
		family.Payments = append(family.Payments, *invoice.Payment)
	}
	if invoice.Status != parentbilling.InvoiceCancelled && invoice.PaymentMethod != "" && !slices.Contains(family.PaymentMethods, invoice.PaymentMethod) {
		family.PaymentMethods = append(family.PaymentMethods, invoice.PaymentMethod)
		slices.SortFunc(family.PaymentMethods, func(a, b string) int {
			return slices.Index(parentbilling.PaymentMethods, a) - slices.Index(parentbilling.PaymentMethods, b)
		})
	}
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

// withFamilyRows adds to the families of the documents their own number and
// whether they are known to the holiday care only (#6692), and lists the
// families without a document: one the holiday care knows, or one with a
// number of its own.
func withFamilyRows(details []parentbilling.BillingFamilyDetail, rows []parentbilling.FamilyBilling) []parentbilling.BillingFamily {
	byKey := make(map[string]*parentbilling.FamilyBilling, len(rows))
	for i := range rows {
		byKey[rows[i].FamilyEmail] = &rows[i]
	}
	families := make([]parentbilling.BillingFamily, 0, len(details)+len(rows))
	listed := make(map[string]bool, len(details))
	for _, detail := range details {
		families = append(families, withFamilyRow(detail.BillingFamily, detail.Documents, byKey[detail.FamilyEmail]))
		listed[detail.FamilyEmail] = true
	}
	for _, row := range rows {
		if listed[row.FamilyEmail] || (row.ContractID != 0 && row.DebtorNumber == "") {
			continue
		}
		first, last := row.HolderFirstName, row.HolderLastName
		if first == "" && last == "" && row.RecipientKind == parentbilling.RecipientPayer {
			first, last = row.PayerFirstName, row.PayerLastName
		}
		families = append(families, parentbilling.BillingFamily{
			RecipientFirstName: first, RecipientLastName: last, RecipientEmail: row.FamilyEmail,
			DebtorNumbers: []string{}, Payments: []parentbilling.BillingPayment{}, PaymentMethods: []string{},
			FamilyEmail: row.FamilyEmail, FamilyDebtorNumber: row.DebtorNumber, HolidayOnly: row.ContractID == 0,
		})
	}
	return families
}

// withFamilyRow sets the family's own number and whether no care contract
// and no monthly invoice name it.
func withFamilyRow(family parentbilling.BillingFamily, documents []parentbilling.BillingFamilyDocument, row *parentbilling.FamilyBilling) parentbilling.BillingFamily {
	care := slices.ContainsFunc(documents, func(document parentbilling.BillingFamilyDocument) bool {
		return document.Service == parentbilling.ServiceCare
	})
	family.HolidayOnly = !care && (row == nil || row.ContractID == 0)
	if row != nil {
		family.FamilyDebtorNumber = row.DebtorNumber
	}
	return family
}
