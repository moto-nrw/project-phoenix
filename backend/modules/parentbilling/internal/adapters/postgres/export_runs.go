package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The released runs of a Träger as the finance exports read them (#4693):
// the list with the last Sollstellung and DATEV batch of each run, and the
// invoices a Sollstellung or a DATEV batch writes. Every subquery names the Träger.

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
	// The newest DATEV batch of the run, all NULL before the first.
	DatevID         *int64     `bun:"datev_id"`
	DatevNumber     *int64     `bun:"datev_number"`
	DatevName       *string    `bun:"datev_name"`
	DatevSHA        *string    `bun:"datev_sha"`
	DatevCount      *int       `bun:"datev_count"`
	DatevTotal      *int64     `bun:"datev_total"`
	DatevAccountID  *int64     `bun:"datev_account_id"`
	DatevExportedAt *time.Time `bun:"datev_exported_at"`
}

// lastFile is the newest file of one format of a run, nil before the first.
func lastFile(run parentbilling.ExportRun, format string, id, number *int64, name, sha *string, count *int, total, account *int64,
	at *time.Time,
) *parentbilling.ExportProtocolEntry {
	if id == nil || number == nil || at == nil {
		return nil
	}
	entry := &parentbilling.ExportProtocolEntry{ID: *id, FileNumber: *number, Format: format, RunKind: run.RunKind, RunID: run.RunID,
		RunLabel: run.Label, FileName: textOf(name), Fingerprint: textOf(sha), ExportedBy: account, ExportedAt: *at}
	if count != nil {
		entry.InvoiceCount = *count
	}
	if total != nil {
		entry.TotalCents = *total
	}
	return entry
}

func (r exportRunRow) run() parentbilling.ExportRun {
	run := parentbilling.ExportRun{RunKind: r.RunKind, RunID: r.RunID, IssuedOn: r.IssuedOn, InvoiceCount: r.InvoiceCount,
		TotalCents: r.TotalCents, Exports: r.Exports,
		Label: parentbilling.ExportRunLabel(r.RunKind, r.CareMonth, nil, textOf(r.PeriodName))}
	run.LastSollstellung = lastFile(run, parentbilling.ExportFormatSollstellung, r.LastID, r.LastNumber, r.LastName, r.LastSHA,
		r.LastCount, r.LastTotal, r.LastAccountID, r.LastExportedAt)
	run.LastDatev = lastFile(run, parentbilling.ExportFormatDatev, r.DatevID, r.DatevNumber, r.DatevName, r.DatevSHA,
		r.DatevCount, r.DatevTotal, r.DatevAccountID, r.DatevExportedAt)
	return run
}

// careRunDocuments counts and sums the rows a Sollstellung of a monthly run
// writes: its invoices and every Storno of them (careExportInvoicesQuery).
const careRunDocuments = `SELECT COUNT(*)::int AS invoice_count, COALESCE(SUM(line.total_cents), 0)::bigint AS total_cents
	FROM (
		SELECT invoice.total_cents FROM parent_billing.care_invoices AS invoice
		WHERE invoice.organization_id = ?0 AND invoice.run_id = run.id
		UNION ALL
		SELECT cancellation.total_cents FROM parent_billing.care_invoice_cancellations AS cancellation
		JOIN parent_billing.care_invoices AS invoice
			ON invoice.organization_id = cancellation.organization_id AND invoice.id = cancellation.invoice_id
		WHERE cancellation.organization_id = ?0 AND invoice.organization_id = ?0 AND invoice.run_id = run.id
	) AS line`

// holidayRunDocuments is the same for a holiday run: its invoices, Storno
// and credits (holidayExportInvoicesQuery).
const holidayRunDocuments = `SELECT COUNT(*)::int AS invoice_count, COALESCE(SUM(document.total_cents), 0)::bigint AS total_cents
	FROM parent_billing.documents AS document
	WHERE document.organization_id = ?0 AND document.run_id = run.id`

// exportRunsQuery lists one page of the released runs with a document,
// newest first, with the count of every run and of those without a
// Sollstellung before the page is cut.
const exportRunsQuery = `SELECT released.run_kind, released.run_id, released.care_month, released.period_name, released.issued_on,
	released.invoice_count, released.total_cents,
	(SELECT COUNT(*) FROM parent_billing.export_files AS file
		WHERE file.organization_id = ?0 AND file.run_kind = released.run_kind AND file.run_id = released.run_id) AS exports,
	last.id AS last_id, last.file_number AS last_number, last.file_name AS last_name, last.content_sha256 AS last_sha,
	last.invoice_count AS last_count, last.total_cents AS last_total, last.exported_by_account_id AS last_account_id,
	last.exported_at AS last_exported_at,
	datev.id AS datev_id, datev.file_number AS datev_number, datev.file_name AS datev_name, datev.content_sha256 AS datev_sha,
	datev.invoice_count AS datev_count, datev.total_cents AS datev_total, datev.exported_by_account_id AS datev_account_id,
	datev.exported_at AS datev_exported_at,
	COUNT(*) OVER () AS total_runs, COUNT(*) FILTER (WHERE last.id IS NULL) OVER () AS not_exported
FROM (
	SELECT 'care' AS run_kind, run.id AS run_id, run.billing_month AS care_month, NULL::text AS period_name, run.issued_on,
		documents.invoice_count, documents.total_cents, run.released_at
	FROM parent_billing.care_runs AS run
	CROSS JOIN LATERAL (` + careRunDocuments + `) AS documents
	WHERE run.organization_id = ?0 AND run.status = 'released' AND documents.invoice_count > 0
	UNION ALL
	SELECT 'holiday', run.id, NULL::date, run.period_name, run.issued_on, documents.invoice_count, documents.total_cents,
		run.released_at
	FROM parent_billing.runs AS run
	CROSS JOIN LATERAL (` + holidayRunDocuments + `) AS documents
	WHERE run.organization_id = ?0 AND run.status = 'released'
) AS released
LEFT JOIN LATERAL (
	SELECT file.* FROM parent_billing.export_files AS file
	WHERE file.organization_id = ?0 AND file.run_kind = released.run_kind AND file.run_id = released.run_id
		AND file.format = 'sollstellung'
	ORDER BY file.file_number DESC LIMIT 1
) AS last ON TRUE
LEFT JOIN LATERAL (
	SELECT file.* FROM parent_billing.export_files AS file
	WHERE file.organization_id = ?0 AND file.run_kind = released.run_kind AND file.run_id = released.run_id
		AND file.format = 'datev'
	ORDER BY file.file_number DESC LIMIT 1
) AS datev ON TRUE
ORDER BY released.released_at DESC, released.run_id DESC, released.run_kind
OFFSET ?1 LIMIT ?2`

// exportRunPageRow is a run of a page with the counts of the whole list.
type exportRunPageRow struct {
	exportRunRow `bun:",extend"`
	TotalRuns    int `bun:"total_runs"`
	NotExported  int `bun:"not_exported"`
}

// ExportRuns reads limit released runs of the Träger after offset, newest
// first, with their exports, and counts every run and those without a
// Sollstellung.
func (s *Store) ExportRuns(ctx context.Context, organizationID int64, offset, limit int) (parentbilling.ExportRunPage, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.ExportRunPage{}, err
	}
	var rows []exportRunPageRow
	if err := db.NewRaw(exportRunsQuery, organizationID, offset, limit).Scan(ctx, &rows); err != nil {
		return parentbilling.ExportRunPage{}, fmt.Errorf("read the runs to export: %w", err)
	}
	page := parentbilling.ExportRunPage{Runs: make([]parentbilling.ExportRun, len(rows))}
	for i, row := range rows {
		page.Runs[i] = row.run()
		page.Total, page.NotExported = row.TotalRuns, row.NotExported
	}
	return page, nil
}

type exportInvoiceRow struct {
	CareInvoiceID       *int64         `bun:"care_invoice_id"`
	CareCancellationID  *int64         `bun:"care_cancellation_id"`
	DocumentID          *int64         `bun:"document_id"`
	ReferenceDocumentID *int64         `bun:"reference_document_id"`
	DatevBooked         bool           `bun:"datev_booked"`
	Number              string         `bun:"number"`
	Kind                string         `bun:"kind"`
	IssuedOn            calendar.Date  `bun:"issued_on,type:date"`
	DueOn               *calendar.Date `bun:"due_on,type:date"`
	TotalCents          int64          `bun:"total_cents"`
	DebtorNumber        *string        `bun:"debtor_number"`
	RecipientName       string         `bun:"recipient_name"`
	Street              *string        `bun:"street"`
	PostalCode          *string        `bun:"postal_code"`
	City                *string        `bun:"city"`
	ChildName           *string        `bun:"child_name"`
	SchoolName          *string        `bun:"school_name"`
	ServiceFrom         calendar.Date  `bun:"service_from,type:date"`
	ServiceUntil        calendar.Date  `bun:"service_until,type:date"`
	PaymentMethod       *string        `bun:"payment_method"`
	MandateReference    *string        `bun:"mandate_reference"`
}

func (r exportInvoiceRow) invoice() parentbilling.ExportInvoice {
	invoice := parentbilling.ExportInvoice{Number: r.Number, Kind: r.Kind, IssuedOn: r.IssuedOn, DueOn: r.DueOn,
		TotalCents: r.TotalCents, DebtorNumber: textOf(r.DebtorNumber), RecipientName: r.RecipientName, Street: textOf(r.Street),
		PostalCode: textOf(r.PostalCode), City: textOf(r.City), ChildName: textOf(r.ChildName), SchoolName: textOf(r.SchoolName),
		ServiceFrom: r.ServiceFrom, ServiceUntil: r.ServiceUntil, PaymentMethod: textOf(r.PaymentMethod),
		MandateReference: textOf(r.MandateReference), DatevBooked: r.DatevBooked}
	for _, id := range []struct {
		from *int64
		to   *int64
	}{{r.CareInvoiceID, &invoice.CareInvoiceID}, {r.CareCancellationID, &invoice.CareCancellationID},
		{r.DocumentID, &invoice.DocumentID}, {r.ReferenceDocumentID, &invoice.ReferenceDocumentID}} {
		if id.from != nil {
			*id.to = *id.from
		}
	}
	return invoice
}

// Whether a booking is in a DATEV file of the Träger (#6433). The Träger
// filter keeps a row of another Träger out below row level security.
const (
	datevBookedCareInvoice = `EXISTS (SELECT 1 FROM parent_billing.datev_booked_documents AS booked
		WHERE booked.organization_id = ?0 AND booked.care_invoice_id = invoice.id)`
	datevBookedCareCancellation = `EXISTS (SELECT 1 FROM parent_billing.datev_booked_documents AS booked
		WHERE booked.organization_id = ?0 AND booked.care_cancellation_id = cancellation.id)`
	datevBookedDocument = `EXISTS (SELECT 1 FROM parent_billing.datev_booked_documents AS booked
		WHERE booked.organization_id = ?0 AND booked.document_id = document.id)`
)

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
const careExportInvoicesQuery = `SELECT care_invoice_id, care_cancellation_id, document_id, reference_document_id, datev_booked,
	number, kind, issued_on, due_on, total_cents, debtor_number,
	recipient_name, street, postal_code, city, child_name, school_name, service_from, service_until, payment_method,
	mandate_reference
FROM (
	SELECT invoice.id AS care_invoice_id, NULL::bigint AS care_cancellation_id, NULL::bigint AS document_id,
		NULL::bigint AS reference_document_id, ` + datevBookedCareInvoice + ` AS datev_booked,
		invoice.number, 'invoice' AS kind, invoice.issued_on,
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
	SELECT invoice.id, cancellation.id, NULL::bigint, NULL::bigint, ` + datevBookedCareCancellation + `,
		cancellation.number, 'cancellation', cancellation.issued_on, NULL::date,
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
const holidayExportInvoicesQuery = `SELECT NULL::bigint AS care_invoice_id, NULL::bigint AS care_cancellation_id,
	document.id AS document_id, document.reference_document_id, ` + datevBookedDocument + ` AS datev_booked,
	document.number, document.kind,
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
	parts, err := exportBookingParts(ctx, db, organizationID, runKind, runID)
	if err != nil {
		return parentbilling.ExportRunContent{}, err
	}
	content := parentbilling.ExportRunContent{Run: head.run(), Invoices: make([]parentbilling.ExportInvoice, len(rows))}
	content.Run.InvoiceCount, content.Run.TotalCents = len(rows), 0
	for i, row := range rows {
		content.Invoices[i] = row.invoice()
		if runKind == parentbilling.DebitRunCare {
			// A Storno relieves the positions of its invoice.
			content.Invoices[i].Parts = parts[row.invoice().CareInvoiceID]
		} else {
			content.Invoices[i].Parts = parts[row.invoice().DocumentID]
		}
		content.Run.TotalCents += row.TotalCents
	}
	return content, nil
}

// Where the positions of the documents of a run book (#4821): care and Mensa
// positions by invoice, holiday positions by document. The Träger filter
// stands on every table, also below row level security.
const (
	careBookingPartsQuery = `SELECT part.document_id, part.kind, part.school_id, part.care_offering_id, part.day, part.amount_cents,
	part.revenue_account, part.cost_center, part.cost_center_2, part.accounts_frozen
FROM (
	SELECT position.invoice_id AS document_id, 'care' AS kind, invoice.school_id, position.care_offering_id,
		position.service_from AS day, position.amount_cents, position.revenue_account, position.cost_center,
		position.cost_center_2, position.accounts_frozen, 1 AS part_order, position.position
	FROM parent_billing.care_invoice_positions AS position
	JOIN parent_billing.care_invoices AS invoice
		ON invoice.organization_id = position.organization_id AND invoice.id = position.invoice_id
	WHERE position.organization_id = ?0 AND invoice.organization_id = ?0 AND invoice.run_id = ?1
	UNION ALL
	SELECT meal.invoice_id, 'meal', meal.school_id, 0, meal.meal_month, meal.amount_cents, meal.revenue_account,
		meal.cost_center, meal.cost_center_2, meal.accounts_frozen, 2, meal.position
	FROM parent_billing.care_meal_positions AS meal
	JOIN parent_billing.care_invoices AS invoice
		ON invoice.organization_id = meal.organization_id AND invoice.id = meal.invoice_id
	WHERE meal.organization_id = ?0 AND invoice.organization_id = ?0 AND invoice.run_id = ?1
) AS part
ORDER BY part.document_id, part.part_order, part.position`
	holidayBookingPartsQuery = `SELECT position.document_id, 'holiday' AS kind, 0 AS school_id, 0 AS care_offering_id,
	document.service_from AS day, position.amount_cents, position.revenue_account, position.cost_center,
	position.cost_center_2, position.accounts_frozen
FROM parent_billing.document_positions AS position
JOIN parent_billing.documents AS document
	ON document.organization_id = position.organization_id AND document.id = position.document_id
WHERE position.organization_id = ?0 AND document.organization_id = ?0 AND document.run_id = ?1
ORDER BY position.document_id, position.position`
)

type exportBookingPartRow struct {
	DocumentID     int64         `bun:"document_id"`
	Kind           string        `bun:"kind"`
	SchoolID       int64         `bun:"school_id"`
	CareOfferingID int64         `bun:"care_offering_id"`
	Day            calendar.Date `bun:"day,type:date"`
	AmountCents    int64         `bun:"amount_cents"`
	positionAccountsColumns
}

// exportBookingParts reads the positions of the documents of a run by
// document: a monthly invoice by its ID, a holiday document by its own.
func exportBookingParts(ctx context.Context, db bun.IDB, organizationID int64, runKind string, runID int64,
) (map[int64][]parentbilling.ExportBookingPart, error) {
	var rows []exportBookingPartRow
	var err error
	if runKind == parentbilling.DebitRunHoliday {
		err = db.NewRaw(holidayBookingPartsQuery, organizationID, runID).Scan(ctx, &rows)
	} else {
		err = db.NewRaw(careBookingPartsQuery, organizationID, runID).Scan(ctx, &rows)
	}
	if err != nil {
		return nil, fmt.Errorf("read where the positions of the run book: %w", err)
	}
	parts := map[int64][]parentbilling.ExportBookingPart{}
	for _, row := range rows {
		parts[row.DocumentID] = append(parts[row.DocumentID], parentbilling.ExportBookingPart{Kind: row.Kind, SchoolID: row.SchoolID,
			CareOfferingID: row.CareOfferingID, Day: row.Day, AmountCents: row.AmountCents, Accounts: row.accounts()})
	}
	return parts, nil
}

// datevRunBookingsQuery reads the documents of monthly runs ?1 and holiday
// runs ?2 with what the DATEV selection needs (#6433), in one statement.
const datevRunBookingsQuery = `SELECT 'care' AS run_kind, invoice.run_id, invoice.id AS care_invoice_id,
	NULL::bigint AS care_cancellation_id, NULL::bigint AS document_id, NULL::bigint AS reference_document_id,
	'invoice' AS kind, invoice.issued_on, invoice.total_cents, invoice.debtor_number,
	` + datevBookedCareInvoice + ` AS datev_booked
FROM parent_billing.care_invoices AS invoice
WHERE invoice.organization_id = ?0 AND invoice.run_id = ANY(?1)
UNION ALL
SELECT 'care', invoice.run_id, invoice.id, cancellation.id, NULL::bigint, NULL::bigint, 'cancellation', cancellation.issued_on,
	cancellation.total_cents, invoice.debtor_number, ` + datevBookedCareCancellation + `
FROM parent_billing.care_invoice_cancellations AS cancellation
JOIN parent_billing.care_invoices AS invoice
	ON invoice.organization_id = cancellation.organization_id AND invoice.id = cancellation.invoice_id
WHERE cancellation.organization_id = ?0 AND invoice.organization_id = ?0 AND invoice.run_id = ANY(?1)
UNION ALL
SELECT 'holiday', document.run_id, NULL::bigint, NULL::bigint, document.id, document.reference_document_id, document.kind,
	document.issued_on, document.total_cents, document.debtor_number, ` + datevBookedDocument + `
FROM parent_billing.documents AS document
WHERE document.organization_id = ?0 AND document.run_id = ANY(?2)`

type datevRunBookingRow struct {
	exportInvoiceRow `bun:",extend"`
	RunKind          string `bun:"run_kind"`
	RunID            int64  `bun:"run_id"`
}

// DatevRunBookings reads the documents of the runs for the count of their
// new DATEV bookings, by run.
func (s *Store) DatevRunBookings(ctx context.Context, organizationID int64, runs []parentbilling.ExportRunRef,
) (map[parentbilling.ExportRunRef][]parentbilling.ExportInvoice, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	care, holiday := []int64{}, []int64{}
	for _, run := range runs {
		switch run.RunKind {
		case parentbilling.DebitRunCare:
			care = append(care, run.RunID)
		case parentbilling.DebitRunHoliday:
			holiday = append(holiday, run.RunID)
		}
	}
	var rows []datevRunBookingRow
	if err := db.NewRaw(datevRunBookingsQuery, organizationID, pgdialect.Array(care), pgdialect.Array(holiday)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read the DATEV bookings of the runs: %w", err)
	}
	byRun := make(map[parentbilling.ExportRunRef][]parentbilling.ExportInvoice, len(runs))
	for _, row := range rows {
		ref := parentbilling.ExportRunRef{RunKind: row.RunKind, RunID: row.RunID}
		byRun[ref] = append(byRun[ref], row.invoice())
	}
	return byRun, nil
}
