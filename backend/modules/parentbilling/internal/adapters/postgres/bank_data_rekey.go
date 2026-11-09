package postgres

import (
	"bytes"
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// BankDataRekeyStore joins the CLI maintenance Träger transaction, never HTTP.
// It registers no query logger: ciphertext and errors are not diagnostics.
type BankDataRekeyStore struct{}

func NewBankDataRekeyStore() *BankDataRekeyStore { return &BankDataRekeyStore{} }

type bankRekeyRow struct {
	ID        int64
	Reference string
	IBAN      []byte `bun:"iban_sealed"`
	Holder    []byte `bun:"account_holder_sealed"`
}

// RewriteBankData joins one bounded maintenance Träger transaction. It changes only
// sealed columns; all business fields, timestamps and audit entries stay put.
func (s *BankDataRekeyStore) RewriteBankData(ctx context.Context, organizationID int64, source string, afterID int64, limit int,
	rewrite func(string, []byte, []byte) ([]byte, []byte, error),
) (parentbilling.BankDataRekeyResult, error) {
	tx, err := bankRekeyTransaction(ctx, organizationID)
	if err != nil {
		return parentbilling.BankDataRekeyResult{}, errors.New("bank data rekey requires a privileged maintenance Träger transaction")
	}
	rows, err := lockedBankRekeyRows(ctx, tx, organizationID, source, afterID, limit)
	result := parentbilling.BankDataRekeyResult{}
	if err == nil {
		result, err = rewriteBankRekeyRows(ctx, tx, organizationID, source, rows, rewrite)
	}
	if err != nil {
		tenant.MarkRollback(ctx)
		return parentbilling.BankDataRekeyResult{}, errors.New("bank data rekey batch failed; transaction must roll back")
	}
	return result, nil
}

// Only the original superuser maintenance login may bypass the immutable-row
// triggers. Both role and replication setting die with this transaction.
func bankRekeyTransaction(ctx context.Context, organizationID int64) (bun.IDB, error) {
	tx, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "RESET ROLE; SET LOCAL lock_timeout = '5s'; SET LOCAL session_replication_role = replica"); err != nil {
		tenant.MarkRollback(ctx)
		return nil, err
	}
	return tx, nil
}

// No skipped locks: the cursor must never pass a concurrent writer. All
// sources keep their own organization filter even under RLS bypass.
// For consents, IBAN carries the single sealed text through the rewriter.
func lockedBankRekeyRows(ctx context.Context, tx bun.IDB, organizationID int64, source string, afterID int64, limit int) ([]bankRekeyRow, error) {
	var rows []bankRekeyRow
	switch source {
	case "mandates":
		err := tx.NewRaw(`SELECT id, reference, iban_sealed, account_holder_sealed
   FROM parent_billing.sepa_mandates WHERE organization_id = ? AND id > ? ORDER BY id LIMIT ? FOR UPDATE`, organizationID, afterID, limit).Scan(ctx, &rows)
		return rows, err
	case "debits":
		err := tx.NewRaw(`SELECT id, mandate_reference AS reference, iban_sealed, account_holder_sealed
   FROM parent_billing.sepa_debits WHERE organization_id = ? AND id > ? ORDER BY id LIMIT ? FOR UPDATE`, organizationID, afterID, limit).Scan(ctx, &rows)
		return rows, err
	case "consents":
		err := tx.NewRaw(`SELECT consent.id, mandate.reference, consent.text_sealed AS iban_sealed
   FROM parent_billing.sepa_mandate_consents AS consent
   JOIN parent_billing.sepa_mandates AS mandate ON mandate.organization_id = consent.organization_id AND mandate.id = consent.mandate_id
   WHERE consent.organization_id = ? AND consent.id > ? ORDER BY consent.id LIMIT ? FOR UPDATE OF consent`, organizationID, afterID, limit).Scan(ctx, &rows)
		return rows, err
	case "debit_files":
		// The stored PAIN.008 file (ADR 0187) is one sealed value, bound to its
		// message ID; rows without a stored download have nothing to rewrap.
		err := tx.NewRaw(`SELECT id, message_id AS reference, file_sealed AS iban_sealed, NULL::bytea AS account_holder_sealed
   FROM parent_billing.sepa_debit_files WHERE organization_id = ? AND id > ? AND file_sealed IS NOT NULL ORDER BY id LIMIT ? FOR UPDATE`,
			organizationID, afterID, limit).Scan(ctx, &rows)
		return rows, err
	default:
		return nil, errors.New("unsupported bank data source")
	}
}

func rewriteBankRekeyRows(ctx context.Context, tx bun.IDB, organizationID int64, source string, rows []bankRekeyRow,
	rewrite func(string, []byte, []byte) ([]byte, []byte, error),
) (parentbilling.BankDataRekeyResult, error) {
	result := parentbilling.BankDataRekeyResult{}
	for _, row := range rows {
		iban, holder, err := rewrite(row.Reference, row.IBAN, row.Holder)
		if err != nil {
			return result, err
		}
		result.Scanned++
		result.LastID = row.ID
		if bytes.Equal(iban, row.IBAN) && bytes.Equal(holder, row.Holder) {
			continue
		}
		if err := updateBankRekeyRow(ctx, tx, organizationID, source, row.ID, iban, holder); err != nil {
			return result, err
		}
		result.Changed++
	}
	return result, nil
}

func updateBankRekeyRow(ctx context.Context, tx bun.IDB, organizationID int64, source string, id int64, iban, holder []byte) error {
	switch source {
	case "mandates":
		_, err := tx.NewRaw(`UPDATE parent_billing.sepa_mandates SET iban_sealed = ?, account_holder_sealed = ?
   WHERE organization_id = ? AND id = ?`, iban, holder, organizationID, id).Exec(ctx)
		return err
	case "debits":
		_, err := tx.NewRaw(`UPDATE parent_billing.sepa_debits SET iban_sealed = ?, account_holder_sealed = ?
   WHERE organization_id = ? AND id = ?`, iban, holder, organizationID, id).Exec(ctx)
		return err
	case "consents":
		_, err := tx.NewRaw(`UPDATE parent_billing.sepa_mandate_consents SET text_sealed = ?
   WHERE organization_id = ? AND id = ?`, iban, organizationID, id).Exec(ctx)
		return err
	case "debit_files":
		_, err := tx.NewRaw(`UPDATE parent_billing.sepa_debit_files SET file_sealed = ?
   WHERE organization_id = ? AND id = ?`, iban, organizationID, id).Exec(ctx)
		return err
	default:
		return errors.New("unsupported bank data source")
	}
}
