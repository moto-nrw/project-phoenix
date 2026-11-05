package postgres

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The corrections a reissued invoice carries over (#5663). A proposal is no
// row of its own: it is a correction position of the cancelled invoice. A
// removed or changed proposal leaves a dismissal, which is only added.

type correctionDismissalRow struct {
	bun.BaseModel `bun:"table:parent_billing.run_correction_dismissals"`

	OrganizationID     int64  `bun:"organization_id,pk"`
	RunID              int64  `bun:"run_id,pk"`
	DocumentPositionID int64  `bun:"document_position_id,pk"`
	CreatedByAccountID *int64 `bun:"created_by_account_id"`
}

// InvoiceCorrections reads the correction positions of the given invoices in
// their order.
func (s *Store) InvoiceCorrections(ctx context.Context, organizationID int64, invoiceIDs []int64) ([]parentbilling.BillingCarriedCorrection, error) {
	if len(invoiceIDs) == 0 {
		return nil, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []documentPositionRow
	if err := db.NewSelect().Model(&rows).
		Where("organization_id = ?", organizationID).Where("document_id IN (?)", bun.List(invoiceIDs)).
		Where("kind = ?", parentbilling.LineCorrection).
		OrderExpr("document_id, position").Scan(ctx); err != nil {
		return nil, err
	}
	corrections := make([]parentbilling.BillingCarriedCorrection, len(rows))
	for i, row := range rows {
		corrections[i] = parentbilling.BillingCarriedCorrection{
			PositionID: row.ID, SourceAmountCents: row.AmountCents, AmountCents: row.AmountCents, Reason: row.Note,
			ChildID: row.HolidayRegistrationChildID, SourceDocumentID: row.DocumentID,
		}
	}
	return corrections, nil
}

// RunCorrectionDismissals names the proposals the run no longer makes.
func (s *Store) RunCorrectionDismissals(ctx context.Context, organizationID, runID int64) (map[int64]bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var ids []int64
	if err := db.NewSelect().Model((*correctionDismissalRow)(nil)).Column("document_position_id").
		Where("organization_id = ?", organizationID).Where("run_id = ?", runID).Scan(ctx, &ids); err != nil {
		return nil, err
	}
	dismissed := make(map[int64]bool, len(ids))
	for _, id := range ids {
		dismissed[id] = true
	}
	return dismissed, nil
}

// InsertRunCorrectionDismissal stops the run from proposing a correction.
func (s *Store) InsertRunCorrectionDismissal(ctx context.Context, organizationID, runID, positionID, accountID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	_, err = db.NewInsert().Model(&correctionDismissalRow{
		OrganizationID: organizationID, RunID: runID, DocumentPositionID: positionID, CreatedByAccountID: &accountID,
	}).Exec(ctx)
	return err
}
