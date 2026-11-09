package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The history of one issued invoice (#4693): its issue with the count of
// positions, every send of its letter (mail and parents inbox), its
// Storno, the confirmed submission of its direct debit and every export
// file that holds it. Every statement names the Träger in each subquery.

type invoiceStepRow struct {
	Kind       string    `bun:"kind"`
	At         time.Time `bun:"at"`
	AccountID  *int64    `bun:"account_id"`
	Positions  int       `bun:"positions"`
	Note       string    `bun:"note"`
	RunID      int64     `bun:"run_id"`
	DeliveryID int64     `bun:"delivery_id"`
}

const careInvoiceStepsQuery = `SELECT 'issued' AS kind, run.released_at AS at,
	(SELECT event.actor_account_id FROM parent_billing.care_run_events AS event
		WHERE event.organization_id = ?0 AND event.run_id = invoice.run_id AND event.action = 'release'
		ORDER BY event.occurred_at DESC LIMIT 1) AS account_id,
	((SELECT COUNT(*) FROM parent_billing.care_invoice_positions AS position
		WHERE position.organization_id = ?0 AND position.invoice_id = invoice.id)
		+ (SELECT COUNT(*) FROM parent_billing.care_meal_positions AS meal
			WHERE meal.organization_id = ?0 AND meal.invoice_id = invoice.id))::int AS positions, '' AS note,
	invoice.run_id, 0::bigint AS delivery_id
FROM parent_billing.care_invoices AS invoice
JOIN parent_billing.care_runs AS run ON run.organization_id = invoice.organization_id AND run.id = invoice.run_id
WHERE invoice.organization_id = ?0 AND run.organization_id = ?0 AND invoice.id = ?1
UNION ALL
SELECT 'mail_requested', mailing.created_at, mailing.requested_by_account_id, 0, letter.kind, mailing.run_id, mailing.delivery_id
FROM parent_billing.care_letter_mailings AS mailing
JOIN parent_billing.care_letters AS letter ON letter.organization_id = mailing.organization_id AND letter.id = mailing.letter_id
WHERE mailing.organization_id = ?0 AND letter.organization_id = ?0 AND letter.invoice_id = ?1
UNION ALL
SELECT 'cancelled', cancellation.issued_at, cancellation.created_by_account_id, 0, cancellation.number, 0, 0
FROM parent_billing.care_invoice_cancellations AS cancellation
WHERE cancellation.organization_id = ?0 AND cancellation.invoice_id = ?1
UNION ALL
SELECT 'submitted', debit.submitted_at, debit.submitted_by_account_id, 0, debit.message_id, 0, 0
FROM parent_billing.sepa_debits AS debit
WHERE debit.organization_id = ?0 AND debit.care_invoice_id = ?1 AND debit.status = 'submitted'`

const holidayDocumentStepsQuery = `SELECT 'issued' AS kind, run.released_at AS at,
	(SELECT event.actor_account_id FROM parent_billing.run_events AS event
		WHERE event.organization_id = ?0 AND event.run_id = document.run_id AND event.action = 'release'
		ORDER BY event.occurred_at DESC LIMIT 1) AS account_id,
	(SELECT COUNT(*) FROM parent_billing.document_positions AS position
		WHERE position.organization_id = ?0 AND position.document_id = document.id)::int AS positions, '' AS note, document.run_id, 0::bigint AS delivery_id
FROM parent_billing.documents AS document
JOIN parent_billing.runs AS run ON run.organization_id = document.organization_id AND run.id = document.run_id
WHERE document.organization_id = ?0 AND run.organization_id = ?0 AND document.id = ?1
UNION ALL
SELECT 'mail_requested', mailing.created_at, mailing.requested_by_account_id, 0, 'invoice', mailing.run_id, mailing.delivery_id
FROM parent_billing.document_mailings AS mailing
WHERE mailing.organization_id = ?0 AND mailing.document_id = ?1
UNION ALL
SELECT 'cancelled', later.issued_at, NULL::bigint, 0, later.number, 0, 0
FROM parent_billing.documents AS later
WHERE later.organization_id = ?0 AND later.reference_document_id = ?1 AND later.kind = 'cancellation'
UNION ALL
SELECT 'submitted', debit.submitted_at, debit.submitted_by_account_id, 0, debit.message_id, 0, 0
FROM parent_billing.sepa_debits AS debit
WHERE debit.organization_id = ?0 AND debit.document_id = ?1 AND debit.status = 'submitted'`

// exportFilesOfCareInvoiceQuery reads every protocol entry that holds a
// monthly invoice, with the label of its run.
const exportFilesOfCareInvoiceQuery = `SELECT file.*,
	(SELECT run.billing_month FROM parent_billing.care_runs AS run
		WHERE run.organization_id = ?0 AND run.id = file.run_id AND file.run_kind = 'care') AS care_month,
	(SELECT run.period_name FROM parent_billing.runs AS run
		WHERE run.organization_id = ?0 AND run.id = file.run_id AND file.run_kind = 'holiday') AS period_name,
	CASE WHEN file.run_kind = 'notice' THEN make_date((file.run_id / 100)::int, (file.run_id % 100)::int, 1) END AS notice_month,
	(SELECT MIN(earlier.file_number) FROM parent_billing.export_files AS earlier
		WHERE earlier.organization_id = ?0 AND earlier.run_kind = file.run_kind AND earlier.run_id = file.run_id
			AND earlier.format = file.format
			AND CASE WHEN file.format = 'pain008' AND NULLIF(file.reference, '') IS NOT NULL
				THEN earlier.reference = file.reference
				ELSE earlier.content_sha256 = file.content_sha256 END
			AND earlier.file_number < file.file_number) AS repeat_of
FROM parent_billing.export_files AS file
WHERE file.organization_id = ?0 AND EXISTS (
	SELECT 1 FROM parent_billing.export_file_invoices AS held
	WHERE held.organization_id = ?0 AND held.export_file_id = file.id AND held.care_invoice_id = ?1)
ORDER BY file.exported_at, file.id`

// exportFilesOfHolidayDocumentQuery is the same for a holiday document.
const exportFilesOfHolidayDocumentQuery = `SELECT file.*,
	(SELECT run.billing_month FROM parent_billing.care_runs AS run
		WHERE run.organization_id = ?0 AND run.id = file.run_id AND file.run_kind = 'care') AS care_month,
	(SELECT run.period_name FROM parent_billing.runs AS run
		WHERE run.organization_id = ?0 AND run.id = file.run_id AND file.run_kind = 'holiday') AS period_name,
	CASE WHEN file.run_kind = 'notice' THEN make_date((file.run_id / 100)::int, (file.run_id % 100)::int, 1) END AS notice_month,
	(SELECT MIN(earlier.file_number) FROM parent_billing.export_files AS earlier
		WHERE earlier.organization_id = ?0 AND earlier.run_kind = file.run_kind AND earlier.run_id = file.run_id
			AND earlier.format = file.format
			AND CASE WHEN file.format = 'pain008' AND NULLIF(file.reference, '') IS NOT NULL
				THEN earlier.reference = file.reference
				ELSE earlier.content_sha256 = file.content_sha256 END
			AND earlier.file_number < file.file_number) AS repeat_of
FROM parent_billing.export_files AS file
WHERE file.organization_id = ?0 AND EXISTS (
	SELECT 1 FROM parent_billing.export_file_invoices AS held
	WHERE held.organization_id = ?0 AND held.export_file_id = file.id AND held.document_id = ?1)
ORDER BY file.exported_at, file.id`

// InvoiceHistory reads the number and the steps of an invoice of the
// Träger, oldest first, and the export files that hold it.
func (s *Store) InvoiceHistory(ctx context.Context, organizationID int64, kind string, id int64) (string, []parentbilling.InvoiceHistoryEntry, []parentbilling.ExportProtocolEntry, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return "", nil, nil, err
	}
	var number string
	var steps []invoiceStepRow
	var files []exportFileListRow
	switch kind {
	case parentbilling.HistoryCareInvoice:
		err = db.NewRaw(`SELECT number FROM parent_billing.care_invoices WHERE organization_id = ? AND id = ?`, organizationID, id).
			Scan(ctx, &number)
		if err == nil {
			err = db.NewRaw(careInvoiceStepsQuery, organizationID, id).Scan(ctx, &steps)
		}
		if err == nil {
			err = db.NewRaw(exportFilesOfCareInvoiceQuery, organizationID, id).Scan(ctx, &files)
		}
	case parentbilling.HistoryHolidayDocument:
		err = db.NewRaw(`SELECT number FROM parent_billing.documents WHERE organization_id = ? AND id = ? AND kind = 'invoice'`,
			organizationID, id).Scan(ctx, &number)
		if err == nil {
			err = db.NewRaw(holidayDocumentStepsQuery, organizationID, id).Scan(ctx, &steps)
		}
		if err == nil {
			err = db.NewRaw(exportFilesOfHolidayDocumentQuery, organizationID, id).Scan(ctx, &files)
		}
	default:
		return "", nil, nil, parentbilling.ErrExportInvoiceNotFound
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, nil, parentbilling.ErrExportInvoiceNotFound
	}
	if err != nil {
		return "", nil, nil, fmt.Errorf("read the history of the invoice: %w", err)
	}
	entries := make([]parentbilling.InvoiceHistoryEntry, len(steps))
	for i, step := range steps {
		entries[i] = parentbilling.InvoiceHistoryEntry{Kind: step.Kind, At: step.At, AccountID: step.AccountID,
			Positions: step.Positions, Note: step.Note, RunID: step.RunID, DeliveryID: step.DeliveryID}
	}
	exports := make([]parentbilling.ExportProtocolEntry, len(files))
	for i, file := range files {
		exports[i] = file.entry()
	}
	return number, entries, exports, nil
}
