package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The changes of single invoices in the review of a monthly run (#6197).
// The request role only appends entries; the trigger of 1.15.821 refuses an
// update and an entry of a released run for every role. A release copies
// what it applied onto the issued invoice (care_invoice_corrections), which
// never changes.

type careRunAmendmentRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_run_amendments"`

	ID                  int64                            `bun:"id,pk,autoincrement"`
	OrganizationID      int64                            `bun:"organization_id"`
	RunID               int64                            `bun:"run_id"`
	CareContractID      int64                            `bun:"care_contract_id"`
	Service             string                           `bun:"service"`
	Kind                string                           `bun:"kind"`
	Label               string                           `bun:"label"`
	AmountCents         int64                            `bun:"amount_cents"`
	PositionKey         string                           `bun:"position_key"`
	BeforeValue         parentbilling.CareAmendmentValue `bun:"before_value,type:jsonb"`
	AfterValue          parentbilling.CareAmendmentValue `bun:"after_value,type:jsonb"`
	Reason              string                           `bun:"reason"`
	RevokesAmendmentID  *int64                           `bun:"revokes_amendment_id"`
	RevokesCorrectionID *int64                           `bun:"revokes_correction_id"`
	AccountID           *int64                           `bun:"account_id"`
	AmendedAt           time.Time                        `bun:"amended_at,nullzero,default:current_timestamp"`
}

type careInvoiceCorrectionRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_invoice_corrections"`

	ID                int64                            `bun:"id,pk,autoincrement"`
	OrganizationID    int64                            `bun:"organization_id"`
	InvoiceID         int64                            `bun:"invoice_id"`
	Position          int                              `bun:"position"`
	Kind              string                           `bun:"kind"`
	Label             string                           `bun:"label"`
	AmountCents       int64                            `bun:"amount_cents"`
	PositionKey       string                           `bun:"position_key"`
	BeforeValue       parentbilling.CareAmendmentValue `bun:"before_value,type:jsonb"`
	AfterValue        parentbilling.CareAmendmentValue `bun:"after_value,type:jsonb"`
	Reason            string                           `bun:"reason"`
	AccountID         *int64                           `bun:"account_id"`
	AmendedAt         time.Time                        `bun:"amended_at"`
	CarriedFromNumber string                           `bun:"carried_from_number"`
	AmendmentID       *int64                           `bun:"amendment_id"`
}

// amendment is the correction as an issued invoice shows it.
func (row careInvoiceCorrectionRow) amendment() parentbilling.CareAmendment {
	return parentbilling.CareAmendment{
		ID: row.ID, Kind: row.Kind, Label: row.Label, AmountCents: row.AmountCents, PositionKey: row.PositionKey,
		Before: row.BeforeValue, After: row.AfterValue, Reason: row.Reason, AccountID: row.AccountID, AmendedAt: row.AmendedAt,
		Carried: row.CarriedFromNumber != "", CarriedFromNumber: row.CarriedFromNumber,
	}
}

// CareRunAmendments reads the entries of a run in the order they were made.
func (s *Store) CareRunAmendments(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareAmendmentEntry, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careRunAmendmentRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Where("run_id = ?", runID).
		OrderExpr("id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read the changes of the review: %w", err)
	}
	entries := make([]parentbilling.CareAmendmentEntry, len(rows))
	for i, row := range rows {
		entries[i] = parentbilling.CareAmendmentEntry{
			ID: row.ID, Invoice: parentbilling.CareInvoiceKey{ContractID: row.CareContractID, Service: row.Service},
			Kind: row.Kind, Label: row.Label, AmountCents: row.AmountCents, PositionKey: row.PositionKey,
			Before: row.BeforeValue, After: row.AfterValue, Reason: row.Reason, RevokesID: row.RevokesAmendmentID,
			RevokesCorrectionID: row.RevokesCorrectionID, AccountID: row.AccountID, AmendedAt: row.AmendedAt,
		}
	}
	return entries, nil
}

// InsertCareRunAmendment appends an entry to the review of a run. The
// trigger refuses an entry of a run that is no longer calculated.
func (s *Store) InsertCareRunAmendment(ctx context.Context, organizationID, runID int64, entry parentbilling.CareAmendmentEntry) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	row := &careRunAmendmentRow{
		OrganizationID: organizationID, RunID: runID, CareContractID: entry.Invoice.ContractID, Service: entry.Invoice.Service,
		Kind: entry.Kind, Label: entry.Label, AmountCents: entry.AmountCents, PositionKey: entry.PositionKey,
		BeforeValue: entry.Before, AfterValue: entry.After, Reason: entry.Reason, RevokesAmendmentID: entry.RevokesID,
		RevokesCorrectionID: entry.RevokesCorrectionID, AccountID: entry.AccountID,
	}
	if _, err := db.NewInsert().Model(row).Exec(ctx); err != nil {
		return fmt.Errorf("insert a change of the review: %w", err)
	}
	return nil
}

// CarriedCareCorrections reads per invoice of the month the corrections of
// its cancelled invoice that no invoice replaced yet. Only the invoices are
// filtered by the Träger; the corrections follow them by invoice ID under
// their composite key.
func (s *Store) CarriedCareCorrections(ctx context.Context, organizationID int64, month calendar.Date) (map[parentbilling.CareInvoiceKey][]parentbilling.CareAmendment, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var cancelled []struct {
		ID             int64  `bun:"id"`
		Number         string `bun:"number"`
		CareContractID int64  `bun:"care_contract_id"`
		Service        string `bun:"service"`
	}
	if err := db.NewRaw(`SELECT invoice.id, invoice.number, invoice.care_contract_id, invoice.service
		FROM parent_billing.care_invoices AS invoice
		WHERE invoice.organization_id = ? AND invoice.billing_month = ?
			AND EXISTS (SELECT 1 FROM parent_billing.care_invoice_cancellations AS cancellation
				WHERE cancellation.organization_id = invoice.organization_id AND cancellation.invoice_id = invoice.id)
			AND NOT EXISTS (SELECT 1 FROM parent_billing.care_invoices AS reissue
				WHERE reissue.organization_id = invoice.organization_id AND reissue.replaces_invoice_id = invoice.id)
		ORDER BY invoice.id`, organizationID, month).Scan(ctx, &cancelled); err != nil {
		return nil, fmt.Errorf("read the cancelled invoices of the month: %w", err)
	}
	carried := map[parentbilling.CareInvoiceKey][]parentbilling.CareAmendment{}
	if len(cancelled) == 0 {
		return carried, nil
	}
	ids := make([]int64, len(cancelled))
	for i, invoice := range cancelled {
		ids[i] = invoice.ID
	}
	corrections, err := careCorrectionsOf(ctx, db, organizationID, ids)
	if err != nil {
		return nil, err
	}
	for _, invoice := range cancelled {
		key := parentbilling.CareInvoiceKey{ContractID: invoice.CareContractID, Service: invoice.Service}
		for _, amendment := range corrections[invoice.ID] {
			amendment.Carried, amendment.CarriedFromNumber = true, invoice.Number
			carried[key] = append(carried[key], amendment)
		}
	}
	return carried, nil
}

// careInvoiceCorrectionsColumn reads the corrections of each issued invoice
// under its own Träger, by position, as one JSON array beside the invoice
// row, in the shape of parentbilling.CareAmendment.
const careInvoiceCorrectionsColumn = `COALESCE((SELECT jsonb_agg(jsonb_build_object(
		'id', correction.id, 'kind', correction.kind, 'label', correction.label, 'amount_cents', correction.amount_cents,
		'position_key', correction.position_key, 'before', correction.before_value, 'after', correction.after_value,
		'reason', correction.reason, 'account_id', correction.account_id, 'amended_at', correction.amended_at,
		'carried', correction.carried_from_number <> '', 'carried_from_number', correction.carried_from_number)
		ORDER BY correction.position)
	FROM parent_billing.care_invoice_corrections AS correction
	WHERE correction.organization_id = care_invoice_row.organization_id AND correction.invoice_id = care_invoice_row.id), '[]'::jsonb) AS corrections`

// insertCareCorrections copies the changes a release applied onto its
// invoices; rows run parallel to invoices.
func insertCareCorrections(ctx context.Context, db bun.IDB, organizationID int64, rows []careInvoiceRow, invoices []parentbilling.CareInvoice) error {
	var corrections []careInvoiceCorrectionRow
	for i, invoice := range invoices {
		position := 0
		for _, amendment := range invoice.Amendments {
			if !amendment.Applied() {
				continue
			}
			position++
			row := careInvoiceCorrectionRow{
				OrganizationID: organizationID, InvoiceID: rows[i].ID, Position: position, Kind: amendment.Kind,
				Label: amendment.Label, AmountCents: amendment.AmountCents, PositionKey: amendment.PositionKey,
				BeforeValue: amendment.Before, AfterValue: amendment.After, Reason: amendment.Reason,
				AccountID: amendment.AccountID, AmendedAt: amendment.AmendedAt, CarriedFromNumber: amendment.CarriedFromNumber,
			}
			if !amendment.Carried {
				id := amendment.ID
				row.AmendmentID = &id
			}
			corrections = append(corrections, row)
		}
	}
	if len(corrections) == 0 {
		return nil
	}
	if _, err := db.NewInsert().Model(&corrections).Exec(ctx); err != nil {
		return fmt.Errorf("insert the corrections of the care invoices: %w", err)
	}
	return nil
}

// careCorrectionsOf reads the corrections of issued invoices by invoice.
// The invoices were read under the Träger's filter; their IDs name it.
func careCorrectionsOf(ctx context.Context, db bun.IDB, organizationID int64, invoiceIDs []int64) (map[int64][]parentbilling.CareAmendment, error) {
	var rows []careInvoiceCorrectionRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).
		Where("invoice_id = ANY(?)", pgdialect.Array(invoiceIDs)).OrderExpr("invoice_id, position").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read the corrections of the care invoices: %w", err)
	}
	byInvoice := map[int64][]parentbilling.CareAmendment{}
	for _, row := range rows {
		byInvoice[row.InvoiceID] = append(byInvoice[row.InvoiceID], row.amendment())
	}
	return byInvoice, nil
}
