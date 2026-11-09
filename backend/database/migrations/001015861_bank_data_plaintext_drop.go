package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const bankDataPlaintextDropVersion = "1.15.861"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     bankDataPlaintextDropVersion,
		Description: "Klartextspalten der Bank- und Steuerdaten von Eltern und Personal entfernt (#6332)",
		DependsOn: []string{
			staffFinancialDataSealedVersion,    // the last migration before this one
			guardianFinancialDataSealedVersion, // the sealed guardian columns
		},
		Precondition: bankDataPlaintextDropPrecondition,
	})
	Migrations.MustRegister(bankDataPlaintextDropUp, bankDataPlaintextDropDown)
}

// The plaintext columns before #6332. bank-data-rekey --school seals their
// values and clears them in every school (ADR 0188); only then may they go.
// A row that still carries one stops the release in the preflight, before
// the application stops: no key reaches a migration, so it cannot seal.
const guardianBankDataPlaintextRowsSQL = `SELECT count(*) FROM users.guardian_financial_data
         WHERE iban IS NOT NULL OR account_holder IS NOT NULL`

const staffBankDataPlaintextRowsSQL = `SELECT count(*) FROM users.staff_financial_data
         WHERE iban IS NOT NULL OR tax_id IS NOT NULL OR social_security_number IS NOT NULL`

var errBankDataPlaintextRemains = errors.New("bank data plaintext remains; run bank-data-rekey --school for every school first (ADR 0188)")

func bankDataPlaintextRows(ctx context.Context, db bun.IDB, checkGuardians, checkStaff bool) (int, error) {
	var guardians, staff int
	if checkGuardians {
		if err := db.NewRaw(guardianBankDataPlaintextRowsSQL).Scan(ctx, &guardians); err != nil {
			return 0, fmt.Errorf("count plaintext guardian bank data: %w", err)
		}
	}
	if checkStaff {
		if err := db.NewRaw(staffBankDataPlaintextRowsSQL).Scan(ctx, &staff); err != nil {
			return 0, fmt.Errorf("count plaintext staff bank data: %w", err)
		}
	}
	return guardians + staff, nil
}

func bankDataPlaintextDropPrecondition(ctx context.Context, db *bun.DB) error {
	// Preflight runs before pending migrations create their tables. Check
	// each existing table even when the other one's creation is pending.
	rows, err := bankDataPlaintextRows(ctx, db,
		!migrationPending(ctx, guardianPaymentDataVersion),
		!migrationPending(ctx, staffStammdatenVersion))
	if err != nil {
		return err
	}
	if rows > 0 {
		return fmt.Errorf("%w: %d rows", errBankDataPlaintextRemains, rows)
	}
	return nil
}

// bankDataLockTimeout bounds the wait for the exclusive lock. The deployment
// stops the application before it migrates, so only a foreign transaction (a
// diagnostics session, a manual query) can hold the tables. A queued
// ACCESS EXCLUSIVE request also blocks every later reader, so the migration
// gives up after this time instead of waiting without bound; the deployment
// then restores its backup and the release can be repeated.
const bankDataLockTimeout = "30s"

var errBankDataLockTimeout = errors.New("bank data tables stayed locked by another transaction; end it and repeat the release")

// bankDataPlaintextDropUp locks both tables against every writer first, then
// rechecks and drops in the same transaction: a writer of an old release
// still open at the check commits before the lock is granted, so its
// plaintext is counted and the drop refused instead of losing the value.
func bankDataPlaintextDropUp(ctx context.Context, db *bun.DB) error {
	return bankDataPlaintextDropUpWithin(ctx, db, bankDataLockTimeout)
}

func bankDataPlaintextDropUpWithin(ctx context.Context, db *bun.DB, lockTimeout string) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SELECT set_config('lock_timeout', ?, true)`, lockTimeout); err != nil {
			return fmt.Errorf("bound the bank data lock wait: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `LOCK TABLE users.guardian_financial_data, users.staff_financial_data IN ACCESS EXCLUSIVE MODE`); err != nil {
			if isLockNotAvailable(err) {
				return fmt.Errorf("%w (waited %s)", errBankDataLockTimeout, lockTimeout)
			}
			return fmt.Errorf("lock the bank data tables: %w", err)
		}
		rows, err := bankDataPlaintextRows(ctx, tx, true, true)
		if err != nil {
			return err
		}
		if rows > 0 {
			return fmt.Errorf("%w: %d rows", errBankDataPlaintextRemains, rows)
		}
		if _, err := tx.ExecContext(ctx, `
ALTER TABLE users.guardian_financial_data
	DROP COLUMN iban,
	DROP COLUMN account_holder;
ALTER TABLE users.staff_financial_data
	DROP COLUMN iban,
	DROP COLUMN tax_id,
	DROP COLUMN social_security_number;`); err != nil {
			return fmt.Errorf("drop the plaintext bank columns: %w", err)
		}
		return nil
	})
}

// bankDataPlaintextDropDown restores the empty columns; the values stay sealed.
func bankDataPlaintextDropDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
ALTER TABLE users.guardian_financial_data
	ADD COLUMN IF NOT EXISTS iban TEXT,
	ADD COLUMN IF NOT EXISTS account_holder TEXT;
ALTER TABLE users.staff_financial_data
	ADD COLUMN IF NOT EXISTS iban TEXT,
	ADD COLUMN IF NOT EXISTS tax_id TEXT,
	ADD COLUMN IF NOT EXISTS social_security_number TEXT;`); err != nil {
		return fmt.Errorf("restore the plaintext bank columns: %w", err)
	}
	return nil
}

// isLockNotAvailable reports PostgreSQL's lock_not_available (55P03), the
// error lock_timeout raises.
func isLockNotAvailable(err error) bool {
	var state interface{ Field(byte) string }
	return errors.As(err, &state) && state.Field('C') == "55P03"
}
