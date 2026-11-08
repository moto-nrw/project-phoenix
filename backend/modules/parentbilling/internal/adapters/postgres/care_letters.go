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

// The letters, sends and Storno of the monthly invoices (#4684, ADR 0165).
// The request role reads and adds them and never changes them (triggers of
// 1.15.809); every statement names the Träger in its own filter.

const (
	// careRunLettersKey is the primary key of the letters of a run.
	careRunLettersKey = "care_run_letters_pkey"
	// careCancellationInvoiceConstraint cancels an invoice at most once.
	careCancellationInvoiceConstraint = "uq_care_invoice_cancellations_invoice"
	// careCancellationNumberConstraint keeps the numbers of the Storno unique.
	careCancellationNumberConstraint = "uq_care_invoice_cancellations_number"
)

type careRunLettersRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_run_letters"`

	OrganizationID     int64         `bun:"organization_id,pk"`
	RunID              int64         `bun:"run_id,pk"`
	PaymentDueOn       calendar.Date `bun:"payment_due_on,type:date"`
	Notes              string        `bun:"notes"`
	CreatedByAccountID *int64        `bun:"created_by_account_id"`
	CreatedAt          time.Time     `bun:"created_at,nullzero,default:current_timestamp"`
}

func (r careRunLettersRow) set() parentbilling.CareLetterSet {
	return parentbilling.CareLetterSet{PaymentDueOn: r.PaymentDueOn, Notes: r.Notes, CreatedAt: r.CreatedAt}
}

type careLetterRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_letters"`

	ID             int64     `bun:"id,pk,nullzero"`
	OrganizationID int64     `bun:"organization_id"`
	RunID          int64     `bun:"run_id"`
	InvoiceID      int64     `bun:"invoice_id"`
	Kind           string    `bun:"kind"`
	Filename       string    `bun:"filename"`
	PDF            []byte    `bun:"pdf"`
	SHA256         string    `bun:"sha256,scanonly"`
	Printed        bool      `bun:"printed,scanonly"`
	CreatedAt      time.Time `bun:"created_at,nullzero,default:current_timestamp"`
}

type careLetterMailingRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_letter_mailings"`

	ID                   int64     `bun:"id,pk,autoincrement"`
	OrganizationID       int64     `bun:"organization_id"`
	RunID                int64     `bun:"run_id"`
	LetterID             int64     `bun:"letter_id"`
	DeliveryID           int64     `bun:"delivery_id"`
	RecipientEmail       string    `bun:"recipient_email"`
	RequestedByAccountID *int64    `bun:"requested_by_account_id"`
	CreatedAt            time.Time `bun:"created_at,nullzero,default:current_timestamp"`
}

func (r careLetterMailingRow) send() parentbilling.CareLetterSend {
	return parentbilling.CareLetterSend{LetterID: r.LetterID, DeliveryID: r.DeliveryID, RecipientEmail: r.RecipientEmail, CreatedAt: r.CreatedAt}
}

type careCancellationRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_invoice_cancellations"`

	ID                 int64         `bun:"id,pk,autoincrement"`
	OrganizationID     int64         `bun:"organization_id"`
	RunID              int64         `bun:"run_id"`
	InvoiceID          int64         `bun:"invoice_id"`
	Number             string        `bun:"number"`
	NumberYear         int           `bun:"number_year"`
	NumberSequence     int64         `bun:"number_sequence"`
	IssuedOn           calendar.Date `bun:"issued_on,type:date"`
	Reason             string        `bun:"reason"`
	TotalCents         int64         `bun:"total_cents"`
	CreatedByAccountID *int64        `bun:"created_by_account_id"`
	Replaced           bool          `bun:"replaced,scanonly"`
}

func (r careCancellationRow) cancellation() parentbilling.CareCancellation {
	return parentbilling.CareCancellation{ID: r.ID, InvoiceID: r.InvoiceID, Number: r.Number, IssuedOn: r.IssuedOn,
		Reason: r.Reason, TotalCents: r.TotalCents, Replaced: r.Replaced}
}

// CareRunLetters reads what the Abrechnung wrote for the letters of a run;
// nil before they are created.
func (s *Store) CareRunLetters(ctx context.Context, organizationID, runID int64) (*parentbilling.CareLetterSet, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	row := &careRunLettersRow{}
	if err := db.NewSelect().Model(row).Where("organization_id = ?", organizationID).Where("run_id = ?", runID).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	set := row.set()
	return &set, nil
}

// InsertCareRunLetters stores what the Abrechnung wrote; a second time is
// ErrBillingLettersExist.
func (s *Store) InsertCareRunLetters(ctx context.Context, organizationID, runID, accountID int64, input parentbilling.CareLetterInput) (parentbilling.CareLetterSet, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareLetterSet{}, err
	}
	row := &careRunLettersRow{OrganizationID: organizationID, RunID: runID, PaymentDueOn: input.PaymentDueOn, Notes: input.Notes,
		CreatedByAccountID: &accountID}
	if _, err := db.NewInsert().Model(row).Returning("created_at").Exec(ctx); err != nil {
		if constraintOf(err) == careRunLettersKey {
			return parentbilling.CareLetterSet{}, parentbilling.ErrBillingLettersExist
		}
		return parentbilling.CareLetterSet{}, fmt.Errorf("insert care run letters: %w", err)
	}
	return row.set(), nil
}

// InsertCareLetter stores the PDF of one letter and returns its ID; the
// database adds its SHA-256.
func (s *Store) InsertCareLetter(ctx context.Context, organizationID, runID, invoiceID int64, kind string, file parentbilling.BillingLetterFile) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	row := &careLetterRow{OrganizationID: organizationID, RunID: runID, InvoiceID: invoiceID, Kind: kind, Filename: file.Filename, PDF: file.Data}
	if _, err := db.NewInsert().Model(row).Returning("id").Exec(ctx); err != nil {
		return 0, fmt.Errorf("insert care letter: %w", err)
	}
	return row.ID, nil
}

// CareRunLetterRecords reads the letters of a run by letter ID, with their
// PDF when withFiles is set.
func (s *Store) CareRunLetterRecords(ctx context.Context, organizationID, runID int64, withFiles bool) ([]parentbilling.CareLetterRecord, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careLetterRow
	query := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Where("run_id = ?", runID).OrderExpr("id")
	if withFiles {
		query = query.Column("care_letter_row.*")
	} else {
		query = query.Column("id", "organization_id", "run_id", "invoice_id", "kind", "filename")
	}
	query = query.ColumnExpr(`EXISTS (SELECT 1 FROM parent_billing.care_run_events AS event
		WHERE event.organization_id = care_letter_row.organization_id AND event.run_id = care_letter_row.run_id
		AND event.action = 'read' AND (
			(event.details->>'letter' = 'print' AND event.details->'letter_ids' @> jsonb_build_array(care_letter_row.id))
			OR (event.details->>'letter' = 'download' AND event.details->'letter_id' = to_jsonb(care_letter_row.id)))) AS printed`)
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("read care letters: %w", err)
	}
	records := make([]parentbilling.CareLetterRecord, len(rows))
	for i, row := range rows {
		records[i] = parentbilling.CareLetterRecord{ID: row.ID, InvoiceID: row.InvoiceID, Kind: row.Kind, Printed: row.Printed}
		if withFiles {
			records[i].File = &parentbilling.BillingLetterFile{Filename: row.Filename, ContentType: "application/pdf", Data: row.PDF}
		}
	}
	return records, nil
}

// CareRunCancellations reads the Storno of the invoices of a run.
func (s *Store) CareRunCancellations(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareCancellation, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careCancellationRow
	if err := db.NewSelect().Model(&rows).Column("id", "invoice_id", "number", "issued_on", "reason", "total_cents").
		ColumnExpr(`EXISTS (SELECT 1 FROM parent_billing.care_invoices AS replacement
			WHERE replacement.organization_id = care_cancellation_row.organization_id
			AND replacement.replaces_invoice_id = care_cancellation_row.invoice_id) AS replaced`).
		Where("care_cancellation_row.organization_id = ?", organizationID).Where("care_cancellation_row.run_id = ?", runID).
		OrderExpr("care_cancellation_row.id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read care cancellations: %w", err)
	}
	cancellations := make([]parentbilling.CareCancellation, len(rows))
	for i, row := range rows {
		cancellations[i] = row.cancellation()
	}
	return cancellations, nil
}

// InsertCareCancellation stores the numbered Storno of an invoice of the
// run. A number another document of the Träger carries is
// ErrInvoiceNumberTaken, a second Storno ErrCareInvoiceCancelled, an invoice
// outside the run ErrCareInvoiceNotFound.
func (s *Store) InsertCareCancellation(ctx context.Context, organizationID, runID, accountID int64, year int, sequence int64,
	cancellation parentbilling.CareCancellation,
) (parentbilling.CareCancellation, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareCancellation{}, err
	}
	numbers := []string{cancellation.Number}
	for _, other := range []string{holidayNumbersTable, careNumbersTable} {
		if err := claimInvoiceNumbers(ctx, db, organizationID, numbers, other); err != nil {
			return parentbilling.CareCancellation{}, err
		}
	}
	var inRun bool
	if err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM parent_billing.care_invoices
		WHERE organization_id = ? AND run_id = ? AND id = ?)`, organizationID, runID, cancellation.InvoiceID).Scan(ctx, &inRun); err != nil {
		return parentbilling.CareCancellation{}, fmt.Errorf("check care invoice: %w", err)
	}
	if !inRun {
		return parentbilling.CareCancellation{}, parentbilling.ErrCareInvoiceNotFound
	}
	row := &careCancellationRow{
		OrganizationID: organizationID, RunID: runID, InvoiceID: cancellation.InvoiceID, Number: cancellation.Number,
		NumberYear: year, NumberSequence: sequence, IssuedOn: cancellation.IssuedOn, Reason: cancellation.Reason,
		TotalCents: cancellation.TotalCents, CreatedByAccountID: &accountID,
	}
	if _, err := db.NewInsert().Model(row).Returning("id").Exec(ctx); err != nil {
		switch constraintOf(err) {
		case careCancellationInvoiceConstraint:
			return parentbilling.CareCancellation{}, parentbilling.ErrCareInvoiceCancelled
		case careCancellationNumberConstraint:
			return parentbilling.CareCancellation{}, parentbilling.ErrInvoiceNumberTaken
		}
		return parentbilling.CareCancellation{}, fmt.Errorf("insert care cancellation: %w", err)
	}
	return row.cancellation(), nil
}

// CareRunMailings reads every send of the letters of a run in the order
// they were stored.
func (s *Store) CareRunMailings(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareLetterSend, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careLetterMailingRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Where("run_id = ?", runID).
		OrderExpr("id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read care mails: %w", err)
	}
	sends := make([]parentbilling.CareLetterSend, len(rows))
	for i, row := range rows {
		sends[i] = row.send()
	}
	return sends, nil
}

// InsertCareMailing stores one send of a letter.
func (s *Store) InsertCareMailing(ctx context.Context, organizationID, runID, accountID int64, send parentbilling.CareLetterSend) (parentbilling.CareLetterSend, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareLetterSend{}, err
	}
	row := &careLetterMailingRow{OrganizationID: organizationID, RunID: runID, LetterID: send.LetterID, DeliveryID: send.DeliveryID,
		RecipientEmail: send.RecipientEmail, RequestedByAccountID: &accountID}
	if _, err := db.NewInsert().Model(row).Returning("id, created_at").Exec(ctx); err != nil {
		return parentbilling.CareLetterSend{}, fmt.Errorf("insert care mail: %w", err)
	}
	return row.send(), nil
}

// cancelledInvoicesToReplace names per contract of the invoices the
// cancelled invoice of the month a new invoice replaces: the latest
// cancelled invoice of the contract that no invoice replaced yet. A contract
// without one gets its first invoice of the month.
func cancelledInvoicesToReplace(ctx context.Context, db bun.IDB, organizationID int64, month calendar.Date,
	invoices []parentbilling.CareInvoice,
) (map[int64]*int64, error) {
	contracts := make([]int64, 0, len(invoices))
	for _, invoice := range invoices {
		contracts = append(contracts, invoice.ContractID)
	}
	var rows []struct {
		ContractID int64 `bun:"care_contract_id"`
		InvoiceID  int64 `bun:"id"`
	}
	if err := db.NewRaw(`SELECT DISTINCT ON (invoice.care_contract_id) invoice.care_contract_id, invoice.id
		FROM parent_billing.care_invoices AS invoice
		JOIN parent_billing.care_invoice_cancellations AS cancellation
			ON cancellation.organization_id = invoice.organization_id AND cancellation.invoice_id = invoice.id
		WHERE invoice.organization_id = ? AND invoice.billing_month = ? AND invoice.care_contract_id = ANY(?)
			AND NOT EXISTS (SELECT 1 FROM parent_billing.care_invoices AS reissue
				WHERE reissue.organization_id = invoice.organization_id AND reissue.replaces_invoice_id = invoice.id)
		ORDER BY invoice.care_contract_id, invoice.id DESC`,
		organizationID, month, pgdialect.Array(contracts)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read cancelled care invoices: %w", err)
	}
	replaced := make(map[int64]*int64, len(rows))
	for _, row := range rows {
		id := row.InvoiceID
		replaced[row.ContractID] = &id
	}
	return replaced, nil
}

// CareLetterRecord loads one PDF rather than every PDF of the monthly run.
func (s *Store) CareLetterRecord(ctx context.Context, organizationID, runID, invoiceID int64, kind string) (parentbilling.CareLetterRecord, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareLetterRecord{}, err
	}
	var row careLetterRow
	if err := db.NewSelect().Model(&row).Column("id", "invoice_id", "kind", "filename", "pdf").Where("organization_id = ?", organizationID).
		Where("run_id = ?", runID).Where("invoice_id = ?", invoiceID).Where("kind = ?", kind).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.CareLetterRecord{}, parentbilling.ErrBillingLetterNotFound
		}
		return parentbilling.CareLetterRecord{}, fmt.Errorf("read care letter: %w", err)
	}
	return parentbilling.CareLetterRecord{ID: row.ID, InvoiceID: row.InvoiceID, Kind: row.Kind,
		File: &parentbilling.BillingLetterFile{Filename: row.Filename, ContentType: "application/pdf", Data: row.PDF}}, nil
}
