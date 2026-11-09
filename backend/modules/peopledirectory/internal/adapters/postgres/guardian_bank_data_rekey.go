package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// GuardianBankRekeyStore joins the CLI maintenance school transaction, never
// HTTP (#6332). It registers no query logger: ciphertext, plaintext and
// errors are no diagnostics.
type GuardianBankRekeyStore struct{}

func NewGuardianBankRekeyStore() *GuardianBankRekeyStore { return &GuardianBankRekeyStore{} }

type guardianBankRekeyRow struct {
	ID                  int64   `bun:"id"`
	GuardianProfileID   int64   `bun:"guardian_profile_id"`
	IBAN                *string `bun:"iban"`
	AccountHolder       *string `bun:"account_holder"`
	IBANSealed          []byte  `bun:"iban_sealed"`
	IBANMasked          *string `bun:"iban_masked"`
	AccountHolderSealed []byte  `bun:"account_holder_sealed"`
}

// RewriteGuardianBankData locks one bounded batch of one school, ordered by
// ID and without skipped locks, so the cursor never passes a concurrent
// writer. It changes only the sealed values, the mask and the plaintext
// columns; every business field and timestamp stays put.
func (s *GuardianBankRekeyStore) RewriteGuardianBankData(ctx context.Context, schoolID, afterID int64, limit int,
	rewrite func(*peopledirectory.GuardianPayment) (bool, error),
) (peopledirectory.BankDataRekeyResult, error) {
	tx, err := maintenanceSchoolTransaction(ctx, schoolID)
	if err != nil {
		return peopledirectory.BankDataRekeyResult{}, errors.New("bank data rekey requires a privileged maintenance school transaction")
	}
	result, err := rewriteGuardianBankRows(ctx, tx, schoolID, afterID, limit, rewrite)
	if err != nil {
		tenant.MarkRollback(ctx)
		return peopledirectory.BankDataRekeyResult{}, errors.New("bank data rekey batch failed; transaction must roll back")
	}
	return result, nil
}

func rewriteGuardianBankRows(ctx context.Context, tx bun.IDB, schoolID, afterID int64, limit int,
	rewrite func(*peopledirectory.GuardianPayment) (bool, error),
) (peopledirectory.BankDataRekeyResult, error) {
	var rows []guardianBankRekeyRow
	// The school filter stays explicit: the maintenance login bypasses RLS.
	if err := tx.NewRaw(`SELECT id, guardian_profile_id, iban, account_holder, iban_sealed, iban_masked, account_holder_sealed
   FROM users.guardian_financial_data WHERE tenant_id = ? AND id > ? ORDER BY id LIMIT ? FOR UPDATE`,
		schoolID, afterID, limit).Scan(ctx, &rows); err != nil {
		return peopledirectory.BankDataRekeyResult{}, err
	}
	result := peopledirectory.BankDataRekeyResult{}
	for _, row := range rows {
		value := &peopledirectory.GuardianPayment{ID: row.ID, TenantID: schoolID, GuardianProfileID: row.GuardianProfileID,
			IBAN: row.IBAN, AccountHolder: row.AccountHolder,
			IBANSealed: row.IBANSealed, IBANMasked: row.IBANMasked, AccountHolderSealed: row.AccountHolderSealed}
		changed, err := rewrite(value)
		if err != nil {
			return result, err
		}
		result.Scanned++
		result.LastID = row.ID
		if !changed {
			continue
		}
		if _, err := tx.NewRaw(`UPDATE users.guardian_financial_data
   SET iban_sealed = ?, iban_masked = ?, account_holder_sealed = ?, iban = NULL, account_holder = NULL
   WHERE tenant_id = ? AND id = ?`, value.IBANSealed, value.IBANMasked, value.AccountHolderSealed, schoolID, row.ID).Exec(ctx); err != nil {
			return result, err
		}
		result.Changed++
	}
	return result, nil
}

// maintenanceSchoolTransaction requires the school transaction of exactly
// this school and drops its request role back to the maintenance login. Only
// a superuser may set the replica role, so an HTTP login never gains the
// bypass; the replica role keeps the updated_at trigger from touching the
// timestamps of a pure rewrap. Role, replica role and lock timeout die with
// the transaction.
func maintenanceSchoolTransaction(ctx context.Context, schoolID int64) (bun.IDB, error) {
	if schoolID <= 0 || tenant.FromContext(ctx) != schoolID {
		return nil, errors.New("people directory postgres: the school transaction of this school is required")
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("people directory postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("people directory postgres: unsupported transaction %T", transaction)
	}
	if _, err := tx.ExecContext(ctx, "RESET ROLE; SET LOCAL lock_timeout = '5s'; SET LOCAL session_replication_role = replica"); err != nil {
		tenant.MarkRollback(ctx)
		return nil, err
	}
	return tx, nil
}
