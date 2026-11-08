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

// The debits of the SEPA direct debit file (#4697, ADR 0181). The release
// writes them from the invoices it issues, copying the sealed account of
// each mandate unopened; every later statement runs in the Träger
// transaction and names the Träger in its own filter.

// insertCareDebitsSQL writes the debits of monthly invoices just issued
// (?1) that are collected by direct debit with a mandate.
const insertCareDebitsSQL = `INSERT INTO parent_billing.sepa_debits (organization_id, mandate_id, care_contract_id, run_kind,
	run_id, care_invoice_id, invoice_number, amount_cents, due_on, sequence_type, creditor_id, mandate_reference,
	mandate_signed_on, account_holder_sealed, iban_sealed, bic)
SELECT invoice.organization_id, mandate.id, mandate.care_contract_id, 'care', invoice.run_id, invoice.id, invoice.number,
	invoice.total_cents, invoice.due_on, invoice.sequence_type, invoice.creditor_id, invoice.mandate_reference,
	invoice.mandate_signed_on, mandate.account_holder_sealed, mandate.iban_sealed, mandate.bic
FROM parent_billing.care_invoices AS invoice
JOIN parent_billing.sepa_mandates AS mandate
	ON mandate.organization_id = invoice.organization_id AND mandate.id = invoice.mandate_id
WHERE invoice.organization_id = ?0 AND invoice.id = ANY(?1) AND invoice.payment_method = 'direct_debit'
	AND invoice.total_cents > 0 AND invoice.due_on IS NOT NULL`

// insertHolidayDebitsSQL writes the debits of holiday invoices just issued.
const insertHolidayDebitsSQL = `INSERT INTO parent_billing.sepa_debits (organization_id, mandate_id, care_contract_id, run_kind,
	run_id, document_id, invoice_number, amount_cents, due_on, sequence_type, creditor_id, mandate_reference,
	mandate_signed_on, account_holder_sealed, iban_sealed, bic)
SELECT document.organization_id, mandate.id, mandate.care_contract_id, 'holiday', document.run_id, document.id, document.number,
	document.total_cents, document.due_on, document.sequence_type, document.creditor_id, document.mandate_reference,
	document.mandate_signed_on, mandate.account_holder_sealed, mandate.iban_sealed, mandate.bic
FROM parent_billing.documents AS document
JOIN parent_billing.sepa_mandates AS mandate
	ON mandate.organization_id = document.organization_id AND mandate.id = document.mandate_id
WHERE document.organization_id = ?0 AND document.id = ANY(?1) AND document.kind = 'invoice'
	AND document.payment_method = 'direct_debit' AND document.total_cents > 0 AND document.due_on IS NOT NULL`

// insertSEPADebits writes the debits of the invoices just issued, in the
// statement after their insert, so they hold the account of the mandate at
// the release (ADR 0181).
func insertSEPADebits(ctx context.Context, db bun.IDB, organizationID int64, query string, invoiceIDs []int64) error {
	if len(invoiceIDs) == 0 {
		return nil
	}
	if _, err := db.NewRaw(query, organizationID, pgdialect.Array(invoiceIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("insert SEPA debits: %w", err)
	}
	return nil
}

// careInvoiceIDs are the IDs of monthly invoices just inserted.
func careInvoiceIDs(rows []careInvoiceRow) []int64 {
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return ids
}

type sepaDebitRow struct {
	bun.BaseModel `bun:"table:parent_billing.sepa_debits,alias:debit"`

	ID                     int64          `bun:"id"`
	MandateID              int64          `bun:"mandate_id"`
	CareContractID         int64          `bun:"care_contract_id"`
	RunKind                string         `bun:"run_kind"`
	RunID                  int64          `bun:"run_id"`
	InvoiceNumber          string         `bun:"invoice_number"`
	AmountCents            int64          `bun:"amount_cents"`
	DueOn                  calendar.Date  `bun:"due_on,type:date"`
	SequenceType           string         `bun:"sequence_type"`
	CreditorID             string         `bun:"creditor_id"`
	MandateReference       string         `bun:"mandate_reference"`
	MandateSignedOn        calendar.Date  `bun:"mandate_signed_on,type:date"`
	BIC                    string         `bun:"bic"`
	Status                 string         `bun:"status"`
	MessageID              *string        `bun:"message_id"`
	SubmittedAt            *time.Time     `bun:"submitted_at"`
	MandateRevoked         bool           `bun:"mandate_revoked"`
	MandateKind            string         `bun:"mandate_kind"`
	MandateSigned          calendar.Date  `bun:"mandate_signed,type:date"`
	MandateSequence        string         `bun:"mandate_sequence"`
	MandateLastCollectedOn *calendar.Date `bun:"mandate_last_collected_on,type:date"`
	MandateCreatedOn       calendar.Date  `bun:"mandate_created_on,type:date"`
	CreditorNow            string         `bun:"creditor_now"`
	RecipientFirstName     string         `bun:"recipient_first_name"`
	RecipientLastName      string         `bun:"recipient_last_name"`
	IBANMasked             string         `bun:"iban_masked"`
	Cancelled              bool           `bun:"cancelled"`
	CreditedCents          int64          `bun:"credited_cents"`
	HolderSealed           []byte         `bun:"account_holder_sealed"`
	IBANSealed             []byte         `bun:"iban_sealed"`
}

func (r sepaDebitRow) sealed() parentbilling.SealedSEPADebit {
	name := r.RecipientFirstName
	if r.RecipientLastName != "" {
		if name != "" {
			name += " "
		}
		name += r.RecipientLastName
	}
	return parentbilling.SealedSEPADebit{
		Debit: parentbilling.SEPADebit{ID: r.ID, MandateID: r.MandateID, ContractID: r.CareContractID, RunKind: r.RunKind,
			RunID: r.RunID, InvoiceNumber: r.InvoiceNumber, RecipientName: name, AmountCents: r.AmountCents, DueOn: r.DueOn,
			SequenceType: r.SequenceType, MandateReference: r.MandateReference, IBANMasked: r.IBANMasked, Status: r.Status,
			SubmittedAt: r.SubmittedAt, MessageID: textOf(r.MessageID)},
		CreditorID: r.CreditorID, MandateSignedOn: r.MandateSignedOn, BIC: r.BIC, MandateRevoked: r.MandateRevoked,
		MandateKind: r.MandateKind, MandateSigned: r.MandateSigned, MandateSequence: r.MandateSequence,
		MandateLastCollectedOn: r.MandateLastCollectedOn, MandateCreatedOn: r.MandateCreatedOn,
		CreditorNow: r.CreditorNow, Cancelled: r.Cancelled, CreditedCents: r.CreditedCents,
		Holder: r.HolderSealed, IBAN: r.IBANSealed,
	}
}

// sepaDebitsSQL reads debits with the state of their mandate and invoice in
// one statement; %s adds the sealed account (or nothing), the filter and
// the lock. Every part names the Träger (?0).
const sepaDebitsSQL = `SELECT debit.id, debit.mandate_id, debit.care_contract_id, debit.run_kind, debit.run_id, debit.invoice_number,
	debit.amount_cents, debit.due_on, debit.sequence_type, debit.creditor_id, debit.mandate_reference, debit.mandate_signed_on,
	debit.bic, debit.status, debit.message_id, debit.submitted_at%s,
	mandate.revoked_on IS NOT NULL AS mandate_revoked, mandate.kind AS mandate_kind, mandate.signed_on AS mandate_signed,
	mandate.sequence AS mandate_sequence, mandate.last_collected_on AS mandate_last_collected_on,
	(mandate.created_at AT TIME ZONE 'Europe/Berlin')::date AS mandate_created_on,
	COALESCE((SELECT creditor.creditor_id FROM parent_billing.sepa_creditors AS creditor
		WHERE creditor.organization_id = ?0), '') AS creditor_now,
	COALESCE(care.recipient_first_name, holiday.recipient_first_name, '') AS recipient_first_name,
	COALESCE(care.recipient_last_name, holiday.recipient_last_name, '') AS recipient_last_name,
	COALESCE(care.debtor_iban_masked, holiday.debtor_iban_masked, '') AS iban_masked,
	(EXISTS (SELECT 1 FROM parent_billing.care_invoice_cancellations AS cancellation
			WHERE cancellation.organization_id = ?0 AND cancellation.invoice_id = debit.care_invoice_id)
		OR EXISTS (SELECT 1 FROM parent_billing.documents AS correction
			WHERE correction.organization_id = ?0 AND correction.reference_document_id = debit.document_id
				AND correction.kind = 'cancellation')) AS cancelled,
	COALESCE((SELECT SUM(correction.total_cents) FROM parent_billing.documents AS correction
		WHERE correction.organization_id = ?0 AND correction.reference_document_id = debit.document_id
			AND correction.kind = 'credit'), 0) AS credited_cents
FROM parent_billing.sepa_debits AS debit
JOIN parent_billing.sepa_mandates AS mandate ON mandate.organization_id = ?0 AND mandate.id = debit.mandate_id
LEFT JOIN parent_billing.care_invoices AS care ON care.organization_id = ?0 AND care.id = debit.care_invoice_id
LEFT JOIN parent_billing.documents AS holiday ON holiday.organization_id = ?0 AND holiday.id = debit.document_id
WHERE debit.organization_id = ?0 AND %s
ORDER BY %s%s`

func (s *Store) selectSEPADebits(ctx context.Context, organizationID int64, sealed bool, filter, order string, lock bool,
	args ...any,
) ([]parentbilling.SealedSEPADebit, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	account, locking := "", ""
	if sealed {
		account = ", debit.account_holder_sealed, debit.iban_sealed"
	}
	if lock {
		locking = " FOR UPDATE OF debit"
	}
	var rows []sepaDebitRow
	query := fmt.Sprintf(sepaDebitsSQL, account, filter, order, locking)
	if err := db.NewRaw(query, append([]any{organizationID}, args...)...).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read SEPA debits: %w", err)
	}
	debits := make([]parentbilling.SealedSEPADebit, len(rows))
	for i, row := range rows {
		debits[i] = row.sealed()
	}
	return debits, nil
}

// releasedRunSQL tells whether a run of the Träger is released.
const releasedRunSQL = `SELECT EXISTS (SELECT 1 FROM parent_billing.%s WHERE organization_id = ? AND id = ? AND status = 'released')`

// SEPADebitsOfRun reads the debits of a released run with the sealed
// account when sealed is set, locked for the confirmation when lock is.
// A run of another Träger, none or one not released is
// ErrSEPADebitRunNotFound.
func (s *Store) SEPADebitsOfRun(ctx context.Context, organizationID int64, runKind string, runID int64, sealed, lock bool,
) ([]parentbilling.SealedSEPADebit, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	table := "care_runs"
	if runKind == parentbilling.DebitRunHoliday {
		table = "runs"
	}
	var released bool
	if err := db.NewRaw(fmt.Sprintf(releasedRunSQL, table), organizationID, runID).Scan(ctx, &released); err != nil {
		return nil, fmt.Errorf("read the run of the SEPA debits: %w", err)
	}
	if !released {
		return nil, parentbilling.ErrSEPADebitRunNotFound
	}
	return s.selectSEPADebits(ctx, organizationID, sealed, "debit.run_kind = ?1 AND debit.run_id = ?2",
		"debit.invoice_number, debit.id", lock, runKind, runID)
}

// SEPADebitsOfMandate reads the debits of a mandate of the Träger, newest
// first, without the account.
func (s *Store) SEPADebitsOfMandate(ctx context.Context, organizationID, mandateID int64) ([]parentbilling.SealedSEPADebit, error) {
	return s.selectSEPADebits(ctx, organizationID, false, "debit.mandate_id = ?1",
		"debit.due_on DESC, debit.id DESC", false, mandateID)
}

// submitSEPADebitsSQL marks pending debits of the Träger submitted.
const submitSEPADebitsSQL = `UPDATE parent_billing.sepa_debits SET status = 'submitted', message_id = ?1,
	submitted_at = CURRENT_TIMESTAMP, submitted_by_account_id = ?2
WHERE organization_id = ?0 AND id = ANY(?3) AND status = 'pending'`

// collectSEPAMandatesSQL sets the last collection of each mandate of the
// debits (?2: mandate IDs, ?3: their collection days) and turns a recurring
// mandate's next sequence to RCUR; a revoked mandate never changes.
const collectSEPAMandatesSQL = `UPDATE parent_billing.sepa_mandates AS mandate
SET sequence = CASE WHEN mandate.kind = 'recurring' THEN 'recurring' ELSE mandate.sequence END,
	last_collected_on = GREATEST(COALESCE(mandate.last_collected_on, collected.collected_on), collected.collected_on),
	updated_at = CURRENT_TIMESTAMP, updated_by_account_id = ?1
FROM unnest(?2::bigint[], ?3::date[]) AS collected(mandate_id, collected_on)
WHERE mandate.organization_id = ?0 AND mandate.id = collected.mandate_id AND mandate.revoked_on IS NULL`

// recurPendingDebitsSQL turns the pending FRST debits of the collected
// mandates in other runs to RCUR: their first collection is done.
const recurPendingDebitsSQL = `UPDATE parent_billing.sepa_debits SET sequence_type = 'RCUR'
WHERE organization_id = ?0 AND status = 'pending' AND sequence_type = 'FRST' AND mandate_id = ANY(?1)`

// SubmitSEPADebits marks the debits submitted with the message ID of their
// file, collects their mandates on the given days and turns the other
// pending FRST debits of those mandates to RCUR, in the caller's
// transaction. A debit no longer pending is ErrSEPADebitsChanged.
func (s *Store) SubmitSEPADebits(ctx context.Context, organizationID, accountID int64, messageID string, debitIDs []int64,
	collected map[int64]calendar.Date,
) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	result, err := db.NewRaw(submitSEPADebitsSQL, organizationID, messageID, accountID, pgdialect.Array(debitIDs)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("submit SEPA debits: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != int64(len(debitIDs)) {
		return parentbilling.ErrSEPADebitsChanged
	}
	mandates := make([]int64, 0, len(collected))
	days := make([]string, 0, len(collected))
	for id, day := range collected {
		mandates = append(mandates, id)
		days = append(days, day.String())
	}
	if _, err := db.NewRaw(collectSEPAMandatesSQL, organizationID, accountID, pgdialect.Array(mandates),
		pgdialect.Array(days)).Exec(ctx); err != nil {
		return fmt.Errorf("collect SEPA mandates: %w", err)
	}
	if _, err := db.NewRaw(recurPendingDebitsSQL, organizationID, pgdialect.Array(mandates)).Exec(ctx); err != nil {
		return fmt.Errorf("turn pending SEPA debits to RCUR: %w", err)
	}
	return nil
}
