package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	dropPersonalPINColumnsVersion     = "1.15.431"
	dropPersonalPINColumnsDescription = "Drop the unused personal staff PIN columns from auth.accounts and the device PIN hash from platform.schools (#3311)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dropPersonalPINColumnsVersion,
		Description: dropPersonalPINColumnsDescription,
		DependsOn: []string{
			AuthAccountsVersion,
			createOrgsAndSchoolsVersion,
			disableLegacyBootstrapAdminVersion,
			staffTargetOverrideWeekdaysVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return dropPersonalPINColumnsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropPersonalPINColumnsDown(ctx, db)
		},
	)
}

// dropPersonalPINColumnsUp removes the hashed credentials of the personal
// staff PIN, which #3310 retired. Kiosk writes check the school's
// security.ogs_device_pin setting, so platform.schools.device_pin_hash has no
// reader either.
func dropPersonalPINColumnsUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.accounts
			DROP COLUMN IF EXISTS pin_hash,
			DROP COLUMN IF EXISTS pin_attempts,
			DROP COLUMN IF EXISTS pin_locked_until;

		ALTER TABLE platform.schools
			DROP COLUMN IF EXISTS device_pin_hash;
	`)
	if err != nil {
		return fmt.Errorf("error dropping personal PIN columns: %w", err)
	}

	return tx.Commit()
}

// dropPersonalPINColumnsDown restores the empty columns. The dropped hashes
// are gone, so every account starts without a PIN and without a lockout.
func dropPersonalPINColumnsDown(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.accounts
			ADD COLUMN IF NOT EXISTS pin_hash VARCHAR(255),
			ADD COLUMN IF NOT EXISTS pin_attempts INTEGER DEFAULT 0,
			ADD COLUMN IF NOT EXISTS pin_locked_until TIMESTAMPTZ;

		ALTER TABLE platform.schools
			ADD COLUMN IF NOT EXISTS device_pin_hash VARCHAR(255);
	`)
	if err != nil {
		return fmt.Errorf("error restoring personal PIN columns: %w", err)
	}

	return tx.Commit()
}
