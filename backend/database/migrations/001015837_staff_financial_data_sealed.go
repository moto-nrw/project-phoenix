package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const staffFinancialDataSealedVersion = "1.15.837"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffFinancialDataSealedVersion,
		Description: "Bank- und Steuerdaten des Personals versiegelt mit dem Bankdaten-Schlüsselring (#6332)",
		DependsOn: []string{
			guardianFinancialDataSealedVersion, // the last migration before this one
			staffStammdatenVersion,             // the staff bank and tax rows
		},
	})
	Migrations.MustRegister(staffFinancialDataSealedUp, staffFinancialDataSealedDown)
}

// staffFinancialDataSealedUpSQL adds the sealed form of the staff bank and
// tax values (#6332, ADR 0183): IBAN, tax ID and social security number
// sealed with the bank ring and bound to field, school and staff member,
// plus the IBAN mask. Tax ID and social security number need no mask: the
// default read shows only that a value exists. The maintenance command
// bank-data-rekey --school seals the existing plaintext and clears it;
// migration 1.15.861 drops the plaintext columns once no row keeps one.
const staffFinancialDataSealedUpSQL = `
ALTER TABLE users.staff_financial_data
	ADD COLUMN IF NOT EXISTS iban_sealed BYTEA,
	ADD COLUMN IF NOT EXISTS iban_masked VARCHAR(48),
	ADD COLUMN IF NOT EXISTS tax_id_sealed BYTEA,
	ADD COLUMN IF NOT EXISTS social_security_number_sealed BYTEA;

COMMENT ON COLUMN users.staff_financial_data.iban_sealed IS
	'IBAN sealed with BANK_DATA_KEYS, bound to field, tenant_id and staff_id (#6332)';
COMMENT ON COLUMN users.staff_financial_data.tax_id_sealed IS
	'Steuer-ID sealed with BANK_DATA_KEYS, bound to field, tenant_id and staff_id (#6332)';
COMMENT ON COLUMN users.staff_financial_data.social_security_number_sealed IS
	'Sozialversicherungsnummer sealed with BANK_DATA_KEYS, bound to field, tenant_id and staff_id (#6332)';
`

const staffFinancialDataSealedDownSQL = `
ALTER TABLE users.staff_financial_data
	DROP COLUMN IF EXISTS social_security_number_sealed,
	DROP COLUMN IF EXISTS tax_id_sealed,
	DROP COLUMN IF EXISTS iban_masked,
	DROP COLUMN IF EXISTS iban_sealed;
`

func staffFinancialDataSealedUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, staffFinancialDataSealedUpSQL); err != nil {
		return fmt.Errorf("add the sealed staff bank and tax columns: %w", err)
	}
	return nil
}

// staffFinancialDataSealedDown refuses while a sealed value exists, locked
// before the check like guardianFinancialDataSealedDown.
func staffFinancialDataSealedDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Bounded like bankDataLockTimeout of 1.15.861: a rollback must not
		// queue an exclusive lock behind a foreign transaction without end.
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '30s'`); err != nil {
			return fmt.Errorf("bound the staff bank data lock wait: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `LOCK TABLE users.staff_financial_data IN ACCESS EXCLUSIVE MODE`); err != nil {
			return fmt.Errorf("lock the staff bank data: %w", err)
		}
		var sealed int
		if err := tx.NewRaw(`SELECT count(*) FROM users.staff_financial_data
   WHERE iban_sealed IS NOT NULL OR tax_id_sealed IS NOT NULL OR social_security_number_sealed IS NOT NULL`).Scan(ctx, &sealed); err != nil {
			return fmt.Errorf("count sealed staff bank values: %w", err)
		}
		if sealed > 0 {
			return fmt.Errorf("%w: %d staff rows", errSealedBankDataRollback, sealed)
		}
		if _, err := tx.ExecContext(ctx, staffFinancialDataSealedDownSQL); err != nil {
			return fmt.Errorf("drop the sealed staff bank columns: %w", err)
		}
		return nil
	})
}
