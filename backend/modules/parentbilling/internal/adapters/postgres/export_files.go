package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The export protocol, the file numbers and the field mapping of the finance
// exports (#4693). Every statement runs in the Träger transaction and names
// the Träger in its own filter, also in every subquery: below row level
// security the filter alone keeps the Träger to its rows.

// exportFileNumberService is the counter of the file numbers of a Träger in
// number_range_counters, one across the formats (year 0).
const exportFileNumberService = "export"

// ExportFileNumbers reads the number the next file gets.
func (s *Store) ExportFileNumbers(ctx context.Context, organizationID int64) (parentbilling.ExportFileNumbers, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.ExportFileNumbers{}, err
	}
	var last int64
	err = db.NewRaw(`SELECT last_number FROM parent_billing.number_range_counters
		WHERE organization_id = ? AND service = ? AND year = 0`, organizationID, exportFileNumberService).Scan(ctx, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return parentbilling.ExportFileNumbers{Next: 1}, nil
	}
	if err != nil {
		return parentbilling.ExportFileNumbers{}, fmt.Errorf("read the export file numbers: %w", err)
	}
	return parentbilling.ExportFileNumbers{Next: last + 1, Started: true}, nil
}

// StartExportFileNumbers sets the number of the first file, once: a Träger
// whose counter exists (a start was set or a file was made) keeps it.
func (s *Store) StartExportFileNumbers(ctx context.Context, organizationID, start int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `INSERT INTO parent_billing.number_range_counters (organization_id, service, year, last_number)
		VALUES (?, ?, 0, ?) ON CONFLICT (organization_id, service, year) DO NOTHING`, organizationID, exportFileNumberService, start-1)
	if err != nil {
		return fmt.Errorf("start the export file numbers: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return parentbilling.ErrExportFileNumberStartLocked
	}
	return nil
}

// ReserveExportFileNumber takes the next file number of the Träger. The
// counter row stays locked until the transaction ends: a second download
// waits, and a rollback returns the number.
func (s *Store) ReserveExportFileNumber(ctx context.Context, organizationID int64) (int64, error) {
	return s.ReserveNumbers(ctx, organizationID, exportFileNumberService, 0, 1)
}

type exportMappingRow struct {
	bun.BaseModel `bun:"table:parent_billing.export_mappings"`

	OrganizationID     int64           `bun:"organization_id,pk"`
	Format             string          `bun:"format,pk"`
	Settings           json.RawMessage `bun:"settings,type:jsonb"`
	UpdatedByAccountID *int64          `bun:"updated_by_account_id"`
	UpdatedAt          time.Time       `bun:"updated_at,nullzero,default:current_timestamp"`
}

// ExportMapping reads the stored mapping of a format, nil without one.
func (s *Store) ExportMapping(ctx context.Context, organizationID int64, format string) (json.RawMessage, *time.Time, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, nil, err
	}
	var row exportMappingRow
	err = db.NewSelect().Model(&row).Where("organization_id = ?", organizationID).Where("format = ?", format).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read the export mapping: %w", err)
	}
	return row.Settings, &row.UpdatedAt, nil
}

// SaveExportMapping stores or replaces the mapping of a format.
func (s *Store) SaveExportMapping(ctx context.Context, organizationID, accountID int64, format string, settings json.RawMessage) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	row := &exportMappingRow{OrganizationID: organizationID, Format: format, Settings: settings, UpdatedByAccountID: accountRef(accountID)}
	if _, err := db.NewInsert().Model(row).On("CONFLICT (organization_id, format) DO UPDATE").
		Set("settings = EXCLUDED.settings").Set("updated_by_account_id = EXCLUDED.updated_by_account_id").
		Set("updated_at = CURRENT_TIMESTAMP").Exec(ctx); err != nil {
		return fmt.Errorf("save the export mapping: %w", err)
	}
	return nil
}

// DeleteExportMapping returns a format to its default mapping.
func (s *Store) DeleteExportMapping(ctx context.Context, organizationID int64, format string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewDelete().Model((*exportMappingRow)(nil)).Where("organization_id = ?", organizationID).
		Where("format = ?", format).Exec(ctx); err != nil {
		return fmt.Errorf("delete the export mapping: %w", err)
	}
	return nil
}

type exportFileRow struct {
	bun.BaseModel `bun:"table:parent_billing.export_files"`

	ID                   int64      `bun:"id,pk,autoincrement"`
	OrganizationID       int64      `bun:"organization_id"`
	FileNumber           int64      `bun:"file_number"`
	Format               string     `bun:"format"`
	RunKind              string     `bun:"run_kind"`
	RunID                int64      `bun:"run_id"`
	FileName             string     `bun:"file_name"`
	ContentSHA256        string     `bun:"content_sha256"`
	BookingSHA256        *string    `bun:"booking_sha256"`
	InvoiceCount         int        `bun:"invoice_count"`
	TotalCents           int64      `bun:"total_cents"`
	Reference            *string    `bun:"reference"`
	ExportedByAccountID  *int64     `bun:"exported_by_account_id"`
	ExportedAt           time.Time  `bun:"exported_at,nullzero,default:current_timestamp"`
	SubmittedAt          *time.Time `bun:"submitted_at"`
	SubmittedByAccountID *int64     `bun:"submitted_by_account_id"`
	DatevAll             bool       `bun:"datev_all"`
	ResolvedPositions    int        `bun:"resolved_positions"`
	// CostCenterFallbacks are the holiday positions the file booked on the
	// KOST1 of the holiday care (#6624).
	CostCenterFallbacks []parentbilling.CostCenterFallback `bun:"cost_center_fallbacks,type:jsonb"`
	// DatevBookingKeys are the bookings of a DATEV file (ADR 0218).
	DatevBookingKeys []string `bun:"datev_booking_keys,array"`
}

// exportFileListRow is a protocol entry with what the list shows besides:
// the run's month or period and the earliest file of the same content.
type exportFileListRow struct {
	exportFileRow `bun:",extend"`

	CareMonth   *calendar.Date `bun:"care_month,type:date,scanonly"`
	NoticeMonth *calendar.Date `bun:"notice_month,type:date,scanonly"`
	PeriodName  *string        `bun:"period_name,scanonly"`
	RepeatOf    *int64         `bun:"repeat_of,scanonly"`
	// The handover of the file (ADR 0218), all nil for a download.
	exportFileHandoverColumns
}

func (r exportFileRow) entry() parentbilling.ExportProtocolEntry {
	fallbacks := r.CostCenterFallbacks
	if fallbacks == nil {
		fallbacks = []parentbilling.CostCenterFallback{}
	}
	return parentbilling.ExportProtocolEntry{ID: r.ID, FileNumber: r.FileNumber, Format: r.Format, RunKind: r.RunKind, RunID: r.RunID,
		FileName: r.FileName, Fingerprint: r.ContentSHA256, InvoiceCount: r.InvoiceCount, TotalCents: r.TotalCents,
		Reference: textOf(r.Reference), ExportedBy: r.ExportedByAccountID, ExportedAt: r.ExportedAt, SubmittedAt: r.SubmittedAt,
		DatevAll: r.DatevAll, ResolvedPositions: r.ResolvedPositions, CostCenterFallbacks: fallbacks}
}

func (r exportFileListRow) entry() parentbilling.ExportProtocolEntry {
	entry := r.exportFileRow.entry()
	entry.RunLabel = parentbilling.ExportRunLabel(r.RunKind, r.CareMonth, r.NoticeMonth, textOf(r.PeriodName))
	if r.RepeatOf != nil {
		entry.RepeatOf = *r.RepeatOf
	}
	entry.Handover = r.handover()
	return entry
}

// costCenterFallbacksJSON is the list as the column holds it, [] for none.
func costCenterFallbacksJSON(fallbacks []parentbilling.CostCenterFallback) string {
	if len(fallbacks) == 0 {
		return "[]"
	}
	data, err := json.Marshal(fallbacks)
	if err != nil {
		// A list of strings and dates always marshals.
		panic(fmt.Sprintf("parent billing: marshal the cost center fallbacks: %v", err))
	}
	return string(data)
}

// InsertExportFile records a file and the invoices it holds; the database
// computes the fingerprint of the content. A number the Träger already used
// fails on uq_export_files_number.
func (s *Store) InsertExportFile(ctx context.Context, organizationID, accountID int64, record parentbilling.ExportFileRecord) (parentbilling.ExportProtocolEntry, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.ExportProtocolEntry{}, err
	}
	var row exportFileRow
	if err := db.NewRaw(`INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id, file_name,
		content_sha256, booking_sha256, invoice_count, total_cents, reference, exported_by_account_id, datev_all,
		resolved_positions, cost_center_fallbacks, datev_booking_keys)
		VALUES (?, ?, ?, ?, ?, ?, encode(sha256(?::bytea), 'hex'), encode(sha256(NULLIF(?::bytea, ''::bytea)), 'hex'), ?, ?, ?, ?, ?, ?, ?::jsonb, ?)
		RETURNING *`, organizationID, record.FileNumber, record.Format, record.RunKind, record.RunID, record.FileName, record.Content,
		record.BookingContent, exportDocumentCount(record), record.TotalCents,
		textRef(record.Reference), accountRef(accountID), record.DatevAll, record.ResolvedPositions,
		costCenterFallbacksJSON(record.CostCenterFallbacks), datevBookingKeys(record)).Scan(ctx, &row); err != nil {
		return parentbilling.ExportProtocolEntry{}, fmt.Errorf("insert the export file: %w", err)
	}
	if err := insertExportFileInvoices(ctx, db, organizationID, row.ID, record); err != nil {
		return parentbilling.ExportProtocolEntry{}, err
	}
	if err := insertDatevBookings(ctx, db, organizationID, row.ID, record); err != nil {
		return parentbilling.ExportProtocolEntry{}, err
	}
	return row.entry(), nil
}

// insertDatevBookedDocuments records the bookings of a DATEV file, one
// array per kind of booking, in one statement.
const insertDatevBookedDocuments = `INSERT INTO parent_billing.datev_booked_documents
		(organization_id, export_file_id, care_invoice_id, care_cancellation_id, document_id)
	SELECT ?0, ?1, booking.care_invoice_id, booking.care_cancellation_id, booking.document_id
	FROM (
		SELECT unnest(?2::bigint[]) AS care_invoice_id, NULL::bigint AS care_cancellation_id, NULL::bigint AS document_id
		UNION ALL SELECT NULL::bigint, unnest(?3::bigint[]), NULL::bigint
		UNION ALL SELECT NULL::bigint, NULL::bigint, unnest(?4::bigint[])
	) AS booking
	ON CONFLICT DO NOTHING`

// insertDatevBookings records the first DATEV file of each booking of a
// DATEV file (#6433). A booking an earlier or a concurrent file recorded
// keeps its row: the unique index makes the insert wait for the other
// transaction and then skip the row. A standard file must record every one
// of its bookings, else another download took them and the file is
// ErrDatevBookingsTaken; "all again" records only the new ones.
func insertDatevBookings(ctx context.Context, db bun.IDB, organizationID, fileID int64, record parentbilling.ExportFileRecord) error {
	if record.Format != parentbilling.ExportFormatDatev || len(record.DatevBookings) == 0 {
		return nil
	}
	invoices, cancellations, documents := datevBookingIDs(record.DatevBookings)
	result, err := db.ExecContext(ctx, insertDatevBookedDocuments, organizationID, fileID,
		pgdialect.Array(invoices), pgdialect.Array(cancellations), pgdialect.Array(documents))
	if err != nil {
		return fmt.Errorf("record the DATEV bookings of the file: %w", err)
	}
	recorded, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count the DATEV bookings of the file: %w", err)
	}
	if !record.DatevAll && recorded != int64(len(record.DatevBookings)) {
		return parentbilling.ErrDatevBookingsTaken
	}
	return nil
}

// datevBookingIDs splits the bookings by kind.
// datevBookingKeys names every booking of a DATEV file exactly; other
// formats hold none.
func datevBookingKeys(record parentbilling.ExportFileRecord) any {
	if record.Format != parentbilling.ExportFormatDatev {
		return nil
	}
	keys := make([]string, 0, len(record.DatevBookings))
	for _, booking := range record.DatevBookings {
		switch {
		case booking.CareCancellationID > 0:
			keys = append(keys, fmt.Sprintf("c:%d", booking.CareCancellationID))
		case booking.CareInvoiceID > 0:
			keys = append(keys, fmt.Sprintf("i:%d", booking.CareInvoiceID))
		case booking.DocumentID > 0:
			keys = append(keys, fmt.Sprintf("d:%d", booking.DocumentID))
		}
	}
	return pgdialect.Array(keys)
}

func datevBookingIDs(bookings []parentbilling.DatevBookingRef) (invoices, cancellations, documents []int64) {
	invoices, cancellations, documents = []int64{}, []int64{}, []int64{}
	for _, booking := range bookings {
		switch {
		case booking.CareCancellationID > 0:
			cancellations = append(cancellations, booking.CareCancellationID)
		case booking.CareInvoiceID > 0:
			invoices = append(invoices, booking.CareInvoiceID)
		case booking.DocumentID > 0:
			documents = append(documents, booking.DocumentID)
		}
	}
	return invoices, cancellations, documents
}

// insertExportFileInvoices links the invoices, holiday documents,
// Jahresbescheide and the invoices of the SEPA debits of a file to it.
func insertExportFileInvoices(ctx context.Context, db bun.IDB, organizationID, fileID int64, record parentbilling.ExportFileRecord) error {
	if len(record.CareInvoiceIDs) > 0 {
		if _, err := db.ExecContext(ctx, `INSERT INTO parent_billing.export_file_invoices (organization_id, export_file_id, care_invoice_id)
			SELECT DISTINCT ?, ?, unnest(?::bigint[])`, organizationID, fileID, pgdialect.Array(record.CareInvoiceIDs)); err != nil {
			return fmt.Errorf("insert the invoices of the export file: %w", err)
		}
	}
	if len(record.DocumentIDs) > 0 {
		if _, err := db.ExecContext(ctx, `INSERT INTO parent_billing.export_file_invoices (organization_id, export_file_id, document_id)
			SELECT DISTINCT ?, ?, unnest(?::bigint[])`, organizationID, fileID, pgdialect.Array(record.DocumentIDs)); err != nil {
			return fmt.Errorf("insert the holiday documents of the export file: %w", err)
		}
	}
	if len(record.CareNoticeIDs) > 0 {
		if _, err := db.ExecContext(ctx, `INSERT INTO parent_billing.export_file_invoices (organization_id, export_file_id, care_notice_id)
			SELECT DISTINCT ?, ?, unnest(?::bigint[])`, organizationID, fileID, pgdialect.Array(record.CareNoticeIDs)); err != nil {
			return fmt.Errorf("insert the Jahresbescheide of the export file: %w", err)
		}
	}
	if len(record.SEPADebitIDs) > 0 {
		if _, err := db.ExecContext(ctx, `INSERT INTO parent_billing.export_file_invoices (organization_id, export_file_id,
			care_invoice_id, document_id)
			SELECT DISTINCT ?0, ?1, debit.care_invoice_id, debit.document_id FROM parent_billing.sepa_debits AS debit
			WHERE debit.organization_id = ?0 AND debit.id = ANY(?2) AND (debit.care_invoice_id IS NOT NULL OR debit.document_id IS NOT NULL)`,
			organizationID, fileID, pgdialect.Array(record.SEPADebitIDs)); err != nil {
			return fmt.Errorf("insert the invoices of the debit file: %w", err)
		}
	}
	return nil
}

// exportFileListQuery reads the protocol entries of a Träger with the
// label of their run and the earliest earlier file of the same content.
// PAIN.008 uses its stable collection/message ID; CreDtTm changes the byte hash.
// DATEV compares bookings and accounting settings with fixed file metadata;
// a Sollstellung its rows with the file number left out (ADR 0218), an
// older one without that hash its content.
const exportFileListQuery = `SELECT file.*,
	(SELECT run.billing_month FROM parent_billing.care_runs AS run
		WHERE run.organization_id = file.organization_id AND run.id = file.run_id AND file.run_kind = 'care') AS care_month,
	(SELECT run.period_name FROM parent_billing.runs AS run
		WHERE run.organization_id = file.organization_id AND run.id = file.run_id AND file.run_kind = 'holiday') AS period_name,
	CASE WHEN file.run_kind = 'notice' THEN make_date((file.run_id / 100)::int, (file.run_id % 100)::int, 1) END AS notice_month,
	(SELECT MIN(earlier.file_number) FROM parent_billing.export_files AS earlier
		WHERE earlier.organization_id = file.organization_id AND earlier.run_kind = file.run_kind AND earlier.run_id = file.run_id
			AND earlier.format = file.format
			AND CASE WHEN file.format = 'pain008' AND NULLIF(file.reference, '') IS NOT NULL
				THEN earlier.reference = file.reference
				WHEN file.format = 'datev' AND file.booking_sha256 IS NOT NULL
				THEN earlier.booking_sha256 = file.booking_sha256
				WHEN file.format = 'sollstellung' AND file.booking_sha256 IS NOT NULL
				THEN COALESCE(earlier.booking_sha256, earlier.content_sha256) = file.booking_sha256
				ELSE earlier.content_sha256 = file.content_sha256 END
			AND earlier.file_number < file.file_number) AS repeat_of,
	CASE WHEN handover.state = 'pending' AND handover.started_at < CURRENT_TIMESTAMP - INTERVAL '` + handoverUnknownAfter + `'
		THEN 'unknown' WHEN handover.state = 'pending' THEN 'in_progress' ELSE handover.state END AS handover_state,
	NULLIF(handover.failure_reason, '') AS handover_reason, handover.target_host AS handover_host,
	handover.target_directory AS handover_directory, NULLIF(handover.recipient_name, '') AS handover_recipient,
	handover.started_at AS handover_started_at,
	handover.settled_at AS handover_settled_at,
	COUNT(*) OVER () AS total_files
FROM parent_billing.export_files AS file
LEFT JOIN parent_billing.export_file_transfers AS handover
	ON handover.organization_id = file.organization_id AND handover.export_file_id = file.id AND handover.organization_id = ?0
WHERE file.organization_id = ?0 AND (?1 = '' OR (file.run_kind = ?1 AND file.run_id = ?2))
ORDER BY file.exported_at DESC, file.id DESC
OFFSET ?3 LIMIT ?4`

// exportFilePageRow is an entry of a page with the count of the protocol.
type exportFilePageRow struct {
	exportFileListRow `bun:",extend"`
	TotalFiles        int `bun:"total_files"`
}

// ExportFiles reads limit entries of the protocol of the Träger after
// offset, newest first, of one run when runKind is set, and counts every
// entry.
func (s *Store) ExportFiles(ctx context.Context, organizationID int64, runKind string, runID int64, offset, limit int,
) (parentbilling.ExportProtocolPage, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.ExportProtocolPage{}, err
	}
	var rows []exportFilePageRow
	if err := db.NewRaw(exportFileListQuery, organizationID, runKind, runID, offset, limit).Scan(ctx, &rows); err != nil {
		return parentbilling.ExportProtocolPage{}, fmt.Errorf("read the export protocol: %w", err)
	}
	page := parentbilling.ExportProtocolPage{Files: make([]parentbilling.ExportProtocolEntry, len(rows))}
	for i, row := range rows {
		page.Files[i] = row.entry()
		page.Total = row.TotalFiles
	}
	return page, nil
}

// MarkExportFilesSubmitted notes on every protocol entry of the PAIN.008
// file messageID of a run that the office confirmed its submission.
func (s *Store) MarkExportFilesSubmitted(ctx context.Context, organizationID, accountID int64, runKind string, runID int64, messageID string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE parent_billing.export_files
		SET submitted_at = CURRENT_TIMESTAMP, submitted_by_account_id = ?
		WHERE organization_id = ? AND format = ? AND run_kind = ? AND run_id = ? AND reference = ? AND submitted_at IS NULL`,
		accountRef(accountID), organizationID, parentbilling.ExportFormatPain008, runKind, runID, messageID); err != nil {
		return fmt.Errorf("mark the export files submitted: %w", err)
	}
	return nil
}

// exportDocumentCount is the number of documents a file holds: the rows of a
// Sollstellung (an invoice and its Storno count twice), else its links.
func exportDocumentCount(record parentbilling.ExportFileRecord) int {
	if record.DocumentCount > 0 {
		return record.DocumentCount
	}
	return len(record.CareInvoiceIDs) + len(record.DocumentIDs) + len(record.CareNoticeIDs) + len(record.SEPADebitIDs)
}
