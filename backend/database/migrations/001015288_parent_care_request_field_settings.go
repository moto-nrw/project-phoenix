package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentCareRequestFieldSettingsVersion     = "1.15.288"
	parentCareRequestFieldSettingsDescription = "Backfill independent parent permanent-care request field settings (#2248)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentCareRequestFieldSettingsVersion,
		Description: parentCareRequestFieldSettingsDescription,
		DependsOn:   []string{parentMessageTeamHandledVersion},
	})
	Migrations.MustRegister(parentCareRequestFieldSettingsUp, parentCareRequestFieldSettingsDown)
}

func parentCareRequestFieldSettingsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.288: Backfilling parent permanent-care request field settings...")
	_, err := db.NewRaw(`
		WITH schools_and_effective_value AS (
			SELECT school.id AS tenant_id,
				COALESCE((notes.value #>> '{}')::boolean, true) AS enabled
			FROM platform.schools AS school
			LEFT JOIN config.setting_values AS notes
				ON notes.tenant_id = school.id
				AND notes.setting_key = 'operations.parent_notes_enabled'
		), inserted AS (
			INSERT INTO config.setting_values (tenant_id, setting_key, value, updated_by)
			SELECT current.tenant_id, field_key, to_jsonb(current.enabled), NULL
			FROM schools_and_effective_value AS current
			CROSS JOIN (VALUES
				('operations.parent_care_pickup_request_enabled'),
				('operations.parent_care_mode_request_enabled')
			) AS fields(field_key)
			ON CONFLICT (tenant_id, setting_key) DO NOTHING
			RETURNING tenant_id, setting_key, value
		)
		INSERT INTO config.setting_audit (
			tenant_id, setting_key, old_value, new_value, action, changed_by
		)
		SELECT tenant_id, setting_key, NULL, value, 'set', NULL
		FROM inserted
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("backfill parent care request field settings: %w", err)
	}
	return nil
}

func parentCareRequestFieldSettingsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.288: Removing parent permanent-care request field settings...")
	_, err := db.NewRaw(`
		WITH deleted AS (
			DELETE FROM config.setting_values
			WHERE setting_key IN (
				'operations.parent_care_arrival_request_enabled',
				'operations.parent_care_pickup_request_enabled',
				'operations.parent_care_mode_request_enabled'
			)
			RETURNING tenant_id, setting_key, value
		)
		INSERT INTO config.setting_audit (
			tenant_id, setting_key, old_value, new_value, action, changed_by
		)
		SELECT tenant_id, setting_key, value, NULL, 'reset', NULL
		FROM deleted
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("remove parent care request field settings: %w", err)
	}
	return nil
}
