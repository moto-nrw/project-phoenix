package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	moduleNFCSwitchVersion     = "1.15.501"
	moduleNFCSwitchDescription = "Modulschalter NFC-Anwesenheit übernimmt attendance.nfc_enabled (#4892)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     moduleNFCSwitchVersion,
		Description: moduleNFCSwitchDescription,
		DependsOn:   []string{configSettingValuesVersion},
	})

	Migrations.MustRegister(moduleNFCSwitchUp, moduleNFCSwitchDown)
}

// moduleNFCSwitchUp carries every school's NFC state over to the module
// switch module.nfc_anwesenheit (#4892). The module switch defaults to
// booked, the old setting defaulted to off, so every school gets an explicit
// value: its stored attendance.nfc_enabled, else false. No school sees NFC
// areas appear or disappear with the deployment. The old values are deleted;
// both steps are recorded in the settings audit as system changes. The audit
// history of the old key stays as it is.
func moduleNFCSwitchUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin module NFC switch migration: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	if _, err := tx.ExecContext(ctx, `
		WITH carried AS (
			INSERT INTO config.setting_values (tenant_id, setting_key, value, updated_by)
			SELECT s.id, 'module.nfc_anwesenheit', COALESCE(old.value, 'false'::jsonb), old.updated_by
			FROM platform.schools s
			LEFT JOIN config.setting_values old
			  ON old.tenant_id = s.id AND old.setting_key = 'attendance.nfc_enabled'
			ON CONFLICT (tenant_id, setting_key) DO NOTHING
			RETURNING tenant_id, setting_key, value
		)
		INSERT INTO config.setting_audit (tenant_id, setting_key, old_value, new_value, action, changed_by)
		SELECT tenant_id, setting_key, NULL, value, 'set', NULL
		FROM carried;
	`); err != nil {
		return fmt.Errorf("carry attendance.nfc_enabled over to module.nfc_anwesenheit: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		WITH deleted AS (
			DELETE FROM config.setting_values
			WHERE setting_key = 'attendance.nfc_enabled'
			RETURNING tenant_id, setting_key, value
		)
		INSERT INTO config.setting_audit (tenant_id, setting_key, old_value, new_value, action, changed_by)
		SELECT tenant_id, setting_key, value, NULL, 'delete', NULL
		FROM deleted;
	`); err != nil {
		return fmt.Errorf("delete attendance.nfc_enabled values: %w", err)
	}

	return tx.Commit()
}

// moduleNFCSwitchDown restores attendance.nfc_enabled from the module switch
// and removes the module switch, so the previous code reads the same NFC
// state again.
func moduleNFCSwitchDown(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin module NFC switch rollback: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO config.setting_values (tenant_id, setting_key, value, updated_by)
		SELECT tenant_id, 'attendance.nfc_enabled', value, updated_by
		FROM config.setting_values
		WHERE setting_key = 'module.nfc_anwesenheit'
		ON CONFLICT (tenant_id, setting_key) DO NOTHING;
		DELETE FROM config.setting_values WHERE setting_key = 'module.nfc_anwesenheit';
	`); err != nil {
		return fmt.Errorf("restore attendance.nfc_enabled: %w", err)
	}

	return tx.Commit()
}
