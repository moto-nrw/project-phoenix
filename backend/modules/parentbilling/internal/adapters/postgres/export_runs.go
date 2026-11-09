package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The released runs of a Träger as the finance exports read them (#4693):
// the list with the last export of each run, and the invoices a
// Sollstellung writes. Every subquery names the Träger.

type exportRunRow struct {
	RunKind      string         `bun:"run_kind"`
	RunID        int64          `bun:"run_id"`
	CareMonth    *calendar.Date `bun:"care_month,type:date"`
	PeriodName   *string        `bun:"period_name"`
	IssuedOn     calendar.Date  `bun:"issued_on,type:date"`
	InvoiceCount int            `bun:"invoice_count"`
	TotalCents   int64          `bun:"total_cents"`
	Exports      int            `bun:"exports"`
	// The newest Sollstellung of the run, all NULL before the first.
	LastID         *int64     `bun:"last_id"`
	LastNumber     *int64     `bun:"last_number"`
	LastName       *string    `bun:"last_name"`
	LastSHA        *string    `bun:"last_sha"`
	LastCount      *int       `bun:"last_count"`
	LastTotal      *int64     `bun:"last_total"`
	LastAccountID  *int64     `bun:"last_account_id"`
	LastExportedAt *time.Time `bun:"last_exported_at"`
}

func (r exportRunRow) run() parentbilling.ExportRun {
	run := parentbilling.ExportRun{RunKind: r.RunKind, RunID: r.RunID, IssuedOn: r.IssuedOn, InvoiceCount: r.InvoiceCount,
		TotalCents: r.TotalCents, Exports: r.Exports,
		Label: parentbilling.ExportRunLabel(r.RunKind, r.CareMonth, nil, textOf(r.PeriodName))}
	if r.LastID != nil && r.LastNumber != nil && r.LastExportedAt != nil {
		run.LastSollstellung = &parentbilling.ExportProtocolEntry{ID: *r.LastID, FileNumber: *r.LastNumber,
			Format: parentbilling.ExportFormatSollstellung, RunKind: r.RunKind, RunID: r.RunID, RunLabel: run.Label,
			FileName: textOf(r.LastName), Fingerprint: textOf(r.LastSHA), ExportedBy: r.LastAccountID, ExportedAt: *r.LastExportedAt}
		if r.LastCount != nil {
			run.LastSollstellung.InvoiceCount = *r.LastCount
		}
		if r.LastTotal != nil {
			run.LastSollstellung.TotalCents = *r.LastTotal
		}
	}
	return run
}

// exportRunsQuery lists the released runs with an invoice, newest first.
const exportRunsQuery = `SELECT released.run_kind, released.run_id, released.care_month, released.period_name, released.issued_on,
	released.invoice_count, released.total_cents,
	(SELECT COUNT(*) FROM parent_billing.export_files AS file
		WHERE file.organization_id = ?0 AND file.run_kind = released.run_kind AND file.run_id = released.run_id) AS exports,
	last.id AS last_id, last.file_number AS last_number, last.file_name AS last_name, last.content_sha256 AS last_sha,
	last.invoice_count AS last_count, last.total_cents AS last_total, last.exported_by_account_id AS last_account_id,
	last.exported_at AS last_exported_at
FROM (
	SELECT 'care' AS run_kind, run.id AS run_id, run.billing_month AS care_month, NULL::text AS period_name, run.issued_on,
		run.document_count AS invoice_count, run.total_cents, run.released_at
	FROM parent_billing.care_runs AS run
	WHERE run.organization_id = ?0 AND run.status = 'released' AND run.document_count > 0
	UNION ALL
	SELECT 'holiday', run.id, NULL::date, run.period_name, run.issued_on, run.document_count, run.total_cents, run.released_at
	FROM parent_billing.runs AS run
	WHERE run.organization_id = ?0 AND run.status = 'released'
) AS released
LEFT JOIN LATERAL (
	SELECT file.* FROM parent_billing.export_files AS file
	WHERE file.organization_id = ?0 AND file.run_kind = released.run_kind AND file.run_id = released.run_id
		AND file.format = 'sollstellung'
	ORDER BY file.file_number DESC LIMIT 1
) AS last ON TRUE
ORDER BY released.released_at DESC, released.run_id DESC
LIMIT ?1`

// MaxExportRuns bounds the runs the export page lists.
const MaxExportRuns = 200

// ExportRuns lists the released runs of the Träger with their exports.
func (s *Store) ExportRuns(ctx context.Context, organizationID int64) ([]parentbilling.ExportRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []exportRunRow
	if err := db.NewRaw(exportRunsQuery, organizationID, MaxExportRuns).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read the runs to export: %w", err)
	}
	runs := make([]parentbilling.ExportRun, len(rows))
	for i, row := range rows {
		runs[i] = row.run()
	}
	return runs, nil
}

type exportInvoiceRow struct {
	CareInvoiceID    *int64         `bun:"care_invoice_id"`
	DocumentID       *int64         `bun:"document_id"`
	Number           string         `bun:"number"`
	Kind             string         `bun:"kind"`
	IssuedOn         calendar.Date  `bun:"issued_on,type:date"`
	DueOn            *calendar.Date `bun:"due_on,type:date"`
	TotalCents       int64          `bun:"total_cents"`
	DebtorNumber     *string        `bun:"debtor_number"`
	RecipientName    string         `bun:"recipient_name"`
	Street           *string        `bun:"street"`
	PostalCode       *string        `bun:"postal_code"`
	City             *string        `bun:"city"`
	ChildName        *string        `bun:"child_name"`
	SchoolName       *string        `bun:"school_name"`
	ServiceFrom      calendar.Date  `bun:"service_from,type:date"`
	ServiceUntil     calendar.Date  `bun:"service_until,type:date"`
	PaymentMethod    *string        `bun:"payment_method"`
	MandateReference *string        `bun:"mandate_reference"`
}

func (r exportInvoiceRow) invoice() parentbilling.ExportInvoice {
	invoice := parentbilling.ExportInvoice{Number: r.Number, Kind: r.Kind, IssuedOn: r.IssuedOn, DueOn: r.DueOn,
		TotalCents: r.TotalCents, DebtorNumber: textOf(r.DebtorNumber), RecipientName: r.RecipientName, Street: textOf(r.Street),
		PostalCode: textOf(r.PostalCode), City: textOf(r.City), ChildName: textOf(r.ChildName), SchoolName: textOf(r.SchoolName),
		ServiceFrom: r.ServiceFrom, ServiceUntil: r.ServiceUntil, PaymentMethod: textOf(r.PaymentMethod),
		MandateReference: textOf(r.MandateReference)}
	if r.CareInvoiceID != nil {
		invoice.CareInvoiceID = *r.CareInvoiceID
	}
	if r.DocumentID != nil {
		invoice.DocumentID = *r.DocumentID
	}
	return invoice
}

// careInvoicePeriod is the service period an issued monthly invoice bills:
// from its first care or Mensa position to its last, the billing month for an
// invoice without positions. A Mensa month billed later (MK-001) widens it.
const careInvoicePeriod = `SELECT
	COALESCE(LEAST(
		(SELECT MIN(position.service_from) FROM parent_billing.care_invoice_positions AS position
			WHERE position.organization_id = ?0 AND position.invoice_id = invoice.id),
		(SELECT MIN(meal.meal_month) FROM parent_billing.care_meal_positions AS meal
			WHERE meal.organization_id = ?0 AND meal.invoice_id = invoice.id)), invoice.billing_month) AS service_from,
	COALESCE(GREATEST(
		(SELECT MAX(position.service_until) FROM parent_billing.care_invoice_positions AS position
			WHERE position.organization_id = ?0 AND position.invoice_id = invoice.id),
		(SELECT MAX((meal.meal_month + INTERVAL '1 month' - INTERVAL '1 day')::date) FROM parent_billing.care_meal_positions AS meal
			WHERE meal.organization_id = ?0 AND meal.invoice_id = invoice.id)),
		(invoice.billing_month + INTERVAL '1 month' - INTERVAL '1 day')::date) AS service_until`

// careExportInvoicesQuery reads the invoices of a monthly run and the Storno
// of each, in number order.
const careExportInvoicesQuery = `SELECT care_invoice_id, document_id, number, kind, issued_on, due_on, total_cents, debtor_number,
	recipient_name, street, postal_code, city, child_name, school_name, service_from, service_until, payment_method,
	mandate_reference
FROM (
	SELECT invoice.id AS care_invoice_id, NULL::bigint AS document_id, invoice.number, 'invoice' AS kind, invoice.issued_on,
		invoice.due_on, invoice.total_cents, invoice.debtor_number,
		BTRIM(invoice.recipient_first_name || ' ' || invoice.recipient_last_name) || CASE WHEN COALESCE(invoice.recipient_second_name, '') <> '' THEN ' und ' || invoice.recipient_second_name ELSE '' END AS recipient_name,
		invoice.recipient_address_street AS street, invoice.recipient_address_postal_code AS postal_code,
		invoice.recipient_address_city AS city, BTRIM(invoice.child_first_name || ' ' || invoice.child_last_name) AS child_name,
		invoice.school_name, period.service_from, period.service_until,
		invoice.payment_method, invoice.mandate_reference, invoice.number_year, invoice.number_sequence
	FROM parent_billing.care_invoices AS invoice
	CROSS JOIN LATERAL (` + careInvoicePeriod + `) AS period
	WHERE invoice.organization_id = ?0 AND invoice.run_id = ?1
	UNION ALL
	SELECT invoice.id, NULL::bigint, cancellation.number, 'cancellation', cancellation.issued_on, NULL::date,
		cancellation.total_cents, invoice.debtor_number,
		BTRIM(invoice.recipient_first_name || ' ' || invoice.recipient_last_name) || CASE WHEN COALESCE(invoice.recipient_second_name, '') <> '' THEN ' und ' || invoice.recipient_second_name ELSE '' END,
		invoice.recipient_address_street, invoice.recipient_address_postal_code, invoice.recipient_address_city,
		BTRIM(invoice.child_first_name || ' ' || invoice.child_last_name), invoice.school_name, period.service_from,
		period.service_until, invoice.payment_method, invoice.mandate_reference,
		cancellation.number_year, cancellation.number_sequence
	FROM parent_billing.care_invoice_cancellations AS cancellation
	JOIN parent_billing.care_invoices AS invoice
		ON invoice.organization_id = cancellation.organization_id AND invoice.id = cancellation.invoice_id
	CROSS JOIN LATERAL (` + careInvoicePeriod + `) AS period
	WHERE cancellation.organization_id = ?0 AND invoice.organization_id = ?0 AND invoice.run_id = ?1
) AS lines
ORDER BY number_year, number_sequence, kind DESC`

// holidayExportInvoicesQuery reads the invoices, Storno and credits of a
// holiday run with the children of their positions.
const holidayExportInvoicesQuery = `SELECT NULL::bigint AS care_invoice_id, document.id AS document_id, document.number, document.kind,
	document.issued_on, document.due_on, document.total_cents, document.debtor_number,
	BTRIM(document.recipient_first_name || ' ' || document.recipient_last_name) || CASE WHEN COALESCE(document.recipient_second_name, '') <> '' THEN ' und ' || document.recipient_second_name ELSE '' END AS recipient_name,
	document.recipient_address_street AS street, document.recipient_address_postal_code AS postal_code,
	document.recipient_address_city AS city,
	(SELECT string_agg(DISTINCT BTRIM(position.child_first_name || ' ' || position.child_last_name), ', ')
		FROM parent_billing.document_positions AS position
		WHERE position.organization_id = ?0 AND position.document_id = document.id) AS child_name,
	NULL::text AS school_name, document.service_from, document.service_until, document.payment_method,
	document.mandate_reference
FROM parent_billing.documents AS document
WHERE document.organization_id = ?0 AND document.run_id = ?1
ORDER BY document.number_year, document.number_sequence`

// ExportRunContent reads a released run and the invoices a Sollstellung of
// it writes; a run of another Träger, none or an open one is
// ErrExportRunNotFound.
func (s *Store) ExportRunContent(ctx context.Context, organizationID int64, runKind string, runID int64) (parentbilling.ExportRunContent, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.ExportRunContent{}, err
	}
	var head exportRunRow
	var rows []exportInvoiceRow
	switch runKind {
	case parentbilling.DebitRunCare:
		err = db.NewRaw(`SELECT 'care' AS run_kind, id AS run_id, billing_month AS care_month, issued_on, document_count AS invoice_count,
			total_cents FROM parent_billing.care_runs WHERE organization_id = ? AND id = ? AND status = 'released'`,
			organizationID, runID).Scan(ctx, &head)
		if err == nil {
			err = db.NewRaw(careExportInvoicesQuery, organizationID, runID).Scan(ctx, &rows)
		}
	case parentbilling.DebitRunHoliday:
		err = db.NewRaw(`SELECT 'holiday' AS run_kind, id AS run_id, period_name, issued_on, document_count AS invoice_count, total_cents
			FROM parent_billing.runs WHERE organization_id = ? AND id = ? AND status = 'released'`, organizationID, runID).Scan(ctx, &head)
		if err == nil {
			err = db.NewRaw(holidayExportInvoicesQuery, organizationID, runID).Scan(ctx, &rows)
		}
	default:
		return parentbilling.ExportRunContent{}, parentbilling.ErrExportRunNotFound
	}
	if errors.Is(err, sql.ErrNoRows) {
		return parentbilling.ExportRunContent{}, parentbilling.ErrExportRunNotFound
	}
	if err != nil {
		return parentbilling.ExportRunContent{}, fmt.Errorf("read the run to export: %w", err)
	}
	content := parentbilling.ExportRunContent{Run: head.run(), Invoices: make([]parentbilling.ExportInvoice, len(rows))}
	for i, row := range rows {
		content.Invoices[i] = row.invoice()
	}
	return content, nil
}
