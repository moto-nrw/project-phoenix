package postgres

import (
	"bytes"
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// BankDataRekeyStore has a dedicated administrative pool, never the serving
// pool. It registers no query logger: ciphertext and errors are not diagnostics.
type BankDataRekeyStore struct{}

func NewBankDataRekeyStore() *BankDataRekeyStore { return &BankDataRekeyStore{} }

type bankRekeyRow struct {
	ID        int64
	Reference string
	IBAN      []byte `bun:"iban_sealed"`
	Holder    []byte `bun:"account_holder_sealed"`
}

// RewriteBankData locks a bounded batch before opening any value. No skipped
// locks: the cursor never passes a concurrent writer. session_replication_role
// is transaction-local, on the privileged maintenance connection only, so
// revoked history may change its encryption without changing any business
// field. The only update below names the two sealed columns explicitly.
func (s *BankDataRekeyStore) RewriteBankData(ctx context.Context, organizationID int64, source string, afterID int64, limit int,
	rewrite func(string, []byte, []byte) ([]byte, []byte, error),
) (parentbilling.BankDataRekeyResult, error) {
	result := parentbilling.BankDataRekeyResult{}
	err := tenant.WithinAdmin(ctx, func(ctx context.Context) error {
		raw, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return errors.New("administrative transaction required")
		}
		tx, ok := raw.(bun.Tx)
		if !ok {
			return errors.New("unsupported administrative transaction")
		}
		// The normal admin runtime assumes phoenix_admin. Only the original
		// superuser maintenance login may disable rewrite triggers locally.
		if _, err := tx.ExecContext(ctx, "RESET ROLE"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout = '5s'"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "SET LOCAL session_replication_role = replica"); err != nil {
			return err
		}
		var rows []bankRekeyRow
		switch source {
		case "mandates":
			err := tx.NewRaw(`SELECT id, reference, iban_sealed, account_holder_sealed
    FROM parent_billing.sepa_mandates WHERE organization_id = ? AND id > ? ORDER BY id LIMIT ? FOR UPDATE`, organizationID, afterID, limit).Scan(ctx, &rows)
			if err != nil {
				return err
			}
		case "debits":
			// #6265 introduces these frozen copies. Include them as soon as its
			// migration is installed; earlier releases have no such table.
			var exists bool
			if err := tx.NewRaw("SELECT to_regclass('parent_billing.sepa_debits') IS NOT NULL").Scan(ctx, &exists); err != nil {
				return err
			}
			if !exists {
				return nil
			}
			err := tx.NewRaw(`SELECT id, mandate_reference AS reference, iban_sealed, account_holder_sealed
    FROM parent_billing.sepa_debits WHERE organization_id = ? AND id > ? ORDER BY id LIMIT ? FOR UPDATE`, organizationID, afterID, limit).Scan(ctx, &rows)
			if err != nil {
				return err
			}
		default:
			return errors.New("unsupported bank data source")
		}
		for _, row := range rows {
			iban, holder, err := rewrite(row.Reference, row.IBAN, row.Holder)
			if err != nil {
				return err
			}
			result.Scanned++
			result.LastID = row.ID
			if bytes.Equal(iban, row.IBAN) && bytes.Equal(holder, row.Holder) {
				continue
			}
			var errUpdate error
			switch source {
			case "mandates":
				_, errUpdate = tx.NewRaw(`UPDATE parent_billing.sepa_mandates SET iban_sealed = ?, account_holder_sealed = ?
     WHERE organization_id = ? AND id = ?`, iban, holder, organizationID, row.ID).Exec(ctx)
			case "debits":
				_, errUpdate = tx.NewRaw(`UPDATE parent_billing.sepa_debits SET iban_sealed = ?, account_holder_sealed = ?
     WHERE organization_id = ? AND id = ?`, iban, holder, organizationID, row.ID).Exec(ctx)
			}
			if errUpdate != nil {
				return errUpdate
			}
			result.Changed++
		}
		return nil
	})
	if err != nil {
		return parentbilling.BankDataRekeyResult{}, errors.New("bank data rekey batch failed; no changes committed")
	}
	return result, nil
}
