package postgres

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// The cancellations, credits and invoices issued again (#4690). They are
// documents like the invoices, with a number of the same number range, and
// never change.

// notCancelledSQL keeps an invoice that has no cancellation; the query names
// it "document".
const notCancelledSQL = `NOT EXISTS (SELECT 1 FROM parent_billing.documents AS cancellation
	WHERE cancellation.organization_id = document.organization_id AND cancellation.kind = 'cancellation'
		AND cancellation.reference_document_id = document.id)`

// IssueDocument stores one numbered document with its positions, the one
// with the sequence of the counter of year, and returns its ID. A second
// cancellation of an invoice is ErrInvoiceCancelled.
func (s *Store) IssueDocument(ctx context.Context, organizationID int64, run parentbilling.BillingRun, year int, sequence int64, document parentbilling.BillingDocument) (int64, error) {
	ids, err := s.insertDocuments(ctx, organizationID, run, year, sequence, []parentbilling.BillingDocument{document})
	if err != nil {
		return 0, err
	}
	return ids[0], nil
}

// ReferencingDocuments reads the documents that reference one of the given
// documents: their cancellations, credits and the invoices that replace
// them, in the order of their numbers.
func (s *Store) ReferencingDocuments(ctx context.Context, organizationID int64, documentIDs []int64) ([]parentbilling.BillingDocument, error) {
	if len(documentIDs) == 0 {
		return []parentbilling.BillingDocument{}, nil
	}
	return s.selectDocuments(ctx, organizationID, func(query *bun.SelectQuery) *bun.SelectQuery {
		return query.Where("document_row.reference_document_id IN (?)", bun.List(documentIDs))
	})
}

// cancelledInvoiceRow is a cancelled invoice of a registration, or of one
// of its children, that no invoice replaces yet.
type cancelledInvoiceRow struct {
	ID     int64  `bun:"id"`
	Number string `bun:"number"`
	invoiceKeyRow
}

// CancelledInvoices names, per registration of the period and child of an
// invoice per child (#7007), the cancelled invoice of the service that no
// invoice replaces yet; the next release issues its replacement.
func (s *Store) CancelledInvoices(ctx context.Context, organizationID int64, service string, periodID int64) (map[parentbilling.HolidayInvoiceKey]parentbilling.BillingDocumentLink, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []cancelledInvoiceRow
	if err := db.NewSelect().TableExpr("parent_billing.documents AS document").
		Column("document.id", "document.holiday_registration_id", "document.holiday_registration_child_id", "document.number").
		Where("document.organization_id = ?", organizationID).Where("document.service = ?", service).
		Where("document.holiday_period_id = ?", periodID).Where("document.kind = ?", parentbilling.DocumentInvoice).
		Where("NOT ("+notCancelledSQL+")").
		Where(`NOT EXISTS (SELECT 1 FROM parent_billing.documents AS replacement
			WHERE replacement.organization_id = document.organization_id AND replacement.kind = 'invoice'
				AND replacement.reference_document_id = document.id)`).
		OrderExpr("document.id").Scan(ctx, &rows); err != nil {
		return nil, err
	}
	cancelled := make(map[parentbilling.HolidayInvoiceKey]parentbilling.BillingDocumentLink, len(rows))
	for _, row := range rows {
		cancelled[row.key()] = parentbilling.BillingDocumentLink{
			ID: row.ID, Kind: parentbilling.DocumentInvoice, Number: row.Number,
		}
	}
	return cancelled, nil
}
