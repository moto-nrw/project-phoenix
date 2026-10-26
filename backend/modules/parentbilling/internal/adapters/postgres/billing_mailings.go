package postgres

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The sends of the confirmation letters (#4670, ADR 0123). The request role
// may insert and read them, never change or delete them; the state of each
// mail stays with Delivery.

type documentMailingRow struct {
	bun.BaseModel `bun:"table:parent_billing.document_mailings"`

	ID                   int64     `bun:"id,pk,autoincrement"`
	OrganizationID       int64     `bun:"organization_id"`
	RunID                int64     `bun:"run_id"`
	DocumentID           int64     `bun:"document_id"`
	DeliveryID           int64     `bun:"delivery_id"`
	RecipientEmail       string    `bun:"recipient_email"`
	RequestedByAccountID *int64    `bun:"requested_by_account_id"`
	CreatedAt            time.Time `bun:"created_at,nullzero,default:current_timestamp"`
}

func (r documentMailingRow) mailing() parentbilling.BillingLetterSend {
	return parentbilling.BillingLetterSend{
		DocumentID: r.DocumentID, DeliveryID: r.DeliveryID, RecipientEmail: r.RecipientEmail, CreatedAt: r.CreatedAt,
	}
}

// RunMailings reads every send of the letters of a run, oldest first.
func (s *Store) RunMailings(ctx context.Context, organizationID, runID int64) ([]parentbilling.BillingLetterSend, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []documentMailingRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Where("run_id = ?", runID).
		OrderExpr("created_at, id").Scan(ctx); err != nil {
		return nil, err
	}
	mailings := make([]parentbilling.BillingLetterSend, len(rows))
	for i, row := range rows {
		mailings[i] = row.mailing()
	}
	return mailings, nil
}

// InsertMailing stores one send of a letter.
func (s *Store) InsertMailing(ctx context.Context, organizationID, runID, accountID int64, mailing parentbilling.BillingLetterSend) (parentbilling.BillingLetterSend, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.BillingLetterSend{}, err
	}
	row := &documentMailingRow{
		OrganizationID: organizationID, RunID: runID, DocumentID: mailing.DocumentID, DeliveryID: mailing.DeliveryID,
		RecipientEmail: mailing.RecipientEmail, RequestedByAccountID: &accountID,
	}
	if _, err := db.NewInsert().Model(row).Returning("id, created_at").Exec(ctx); err != nil {
		return parentbilling.BillingLetterSend{}, err
	}
	return row.mailing(), nil
}

// RunLetterFiles reads the stored letters of a run in the order of their
// invoice numbers.
func (s *Store) RunLetterFiles(ctx context.Context, organizationID, runID int64) ([]parentbilling.BillingDocumentLetter, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []documentLetterRow
	if err := db.NewSelect().Model(&rows).
		Join("JOIN parent_billing.documents AS document ON document.organization_id = document_letter_row.organization_id AND document.id = document_letter_row.document_id").
		Where("document_letter_row.organization_id = ?", organizationID).Where("document_letter_row.run_id = ?", runID).
		OrderExpr("document.number_sequence, document.id").Scan(ctx); err != nil {
		return nil, err
	}
	files := make([]parentbilling.BillingDocumentLetter, len(rows))
	for i, row := range rows {
		files[i] = parentbilling.BillingDocumentLetter{DocumentID: row.DocumentID, File: parentbilling.BillingLetterFile{
			Filename: row.Filename, ContentType: "application/pdf", Data: row.PDF,
		}}
	}
	return files, nil
}
