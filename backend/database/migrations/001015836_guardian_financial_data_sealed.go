package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const guardianFinancialDataSealedVersion = "1.15.836"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianFinancialDataSealedVersion,
		Description: "Bankdaten der Erziehungsberechtigten versiegelt mit dem Bankdaten-Schlüsselring (#6332)",
		DependsOn: []string{
			accountCredentialGenerationBindingVersion, // the last migration before this one
			guardianPaymentDataVersion,                // the guardian bank rows
		},
	})
	Migrations.MustRegister(guardianFinancialDataSealedUp, guardianFinancialDataSealedDown)
}

// guardianFinancialDataSealedUpSQL adds the sealed form of the guardian bank
// values (#6332, ADR 0183): IBAN and account holder sealed with the bank ring
// and bound to field, school and guardian, plus the IBAN mask the default
// reads serve without opening anything. Migrations hold no key, so the
// maintenance command bank-data-rekey --school seals the existing plaintext
// and clears it; writers store only the sealed form from this release on.
// Migration 1.15.861 drops the plaintext columns once no row keeps one.
const guardianFinancialDataSealedUpSQL = `
ALTER TABLE users.guardian_financial_data
	ADD COLUMN IF NOT EXISTS iban_sealed BYTEA,
	ADD COLUMN IF NOT EXISTS iban_masked VARCHAR(48),
	ADD COLUMN IF NOT EXISTS account_holder_sealed BYTEA;

COMMENT ON COLUMN users.guardian_financial_data.iban_sealed IS
	'IBAN sealed with BANK_DATA_KEYS, bound to field, tenant_id and guardian_profile_id (#6332)';
COMMENT ON COLUMN users.guardian_financial_data.account_holder_sealed IS
	'Kontoinhaber sealed with BANK_DATA_KEYS when the account does not run on the guardian''s own name; NULL = the guardian is the holder (#6332)';
`

// errSealedBankDataRollback stops a rollback that would drop the only copy
// of sealed bank data.
var errSealedBankDataRollback = errors.New("sealed bank data exists; a rollback would destroy it, restore the release backup instead")

const guardianFinancialDataSealedDownSQL = `
ALTER TABLE users.guardian_financial_data
	DROP COLUMN IF EXISTS account_holder_sealed,
	DROP COLUMN IF EXISTS iban_masked,
	DROP COLUMN IF EXISTS iban_sealed;
`

func guardianFinancialDataSealedUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, guardianFinancialDataSealedUpSQL); err != nil {
		return fmt.Errorf("add the sealed guardian bank columns: %w", err)
	}
	return nil
}

// guardianFinancialDataSealedDown refuses while a sealed value exists: after a
// write or the backfill the sealed columns hold the only copy, so dropping
// them would destroy bank data (#6332). Restore the release backup instead.
// It locks the table against every writer first, then counts and drops in the
// same transaction: a writer still open at the check commits before the lock
// is granted, so its sealed value is counted and never dropped unseen.
func guardianFinancialDataSealedDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Bounded like bankDataLockTimeout of 1.15.861: a rollback must not
		// queue an exclusive lock behind a foreign transaction without end.
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '30s'`); err != nil {
			return fmt.Errorf("bound the guardian bank data lock wait: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `LOCK TABLE users.guardian_financial_data IN ACCESS EXCLUSIVE MODE`); err != nil {
			return fmt.Errorf("lock the guardian bank data: %w", err)
		}
		var sealed int
		if err := tx.NewRaw(`SELECT count(*) FROM users.guardian_financial_data
   WHERE iban_sealed IS NOT NULL OR account_holder_sealed IS NOT NULL`).Scan(ctx, &sealed); err != nil {
			return fmt.Errorf("count sealed guardian bank values: %w", err)
		}
		if sealed > 0 {
			return fmt.Errorf("%w: %d guardian rows", errSealedBankDataRollback, sealed)
		}
		if _, err := tx.ExecContext(ctx, guardianFinancialDataSealedDownSQL); err != nil {
			return fmt.Errorf("drop the sealed guardian bank columns: %w", err)
		}
		return nil
	})
}
