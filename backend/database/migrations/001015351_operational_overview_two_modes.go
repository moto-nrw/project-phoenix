package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	operationalOverviewTwoModesVersion     = "1.15.351"
	operationalOverviewTwoModesDescription = "Preserve existing operational overview scopes before enabling the whole-team default"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: operationalOverviewTwoModesVersion, Description: operationalOverviewTwoModesDescription,
		DependsOn: []string{operationalOverviewScopeVersion},
	})
	Migrations.MustRegister(operationalOverviewTwoModesUp, operationalOverviewTwoModesDown)
}

func operationalOverviewTwoModesUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration starting",
		slog.String("migration", operationalOverviewTwoModesVersion),
		slog.String("detail", "preserving operational overview scopes for existing schools"),
	)

	if _, err := db.NewRaw(`
		WITH desired AS (
			SELECT
				s.id AS tenant_id,
				stored.value AS old_value,
				CASE
					WHEN stored.value #>> '{}' = 'all_staff' THEN '"all_staff"'::jsonb
					ELSE '"own"'::jsonb
				END AS new_value
			FROM platform.schools AS s
			LEFT JOIN config.setting_values AS stored
				ON stored.tenant_id = s.id
				AND stored.setting_key = ?
		), changed AS (
			INSERT INTO config.setting_values (tenant_id, setting_key, value)
			SELECT tenant_id, ?, new_value
			FROM desired
			ON CONFLICT (tenant_id, setting_key) DO UPDATE
			SET value = EXCLUDED.value
			WHERE config.setting_values.value IS DISTINCT FROM EXCLUDED.value
			RETURNING tenant_id, setting_key, value
		)
		INSERT INTO config.setting_audit (
			tenant_id, setting_key, old_value, new_value, action, changed_by
		)
		SELECT changed.tenant_id, changed.setting_key, desired.old_value, changed.value, 'set', NULL
		FROM changed
		JOIN desired ON desired.tenant_id = changed.tenant_id;
	`, operationalOverviewScopeKey, operationalOverviewScopeKey).Exec(ctx); err != nil {
		return fmt.Errorf("preserve operational overview scopes: %w", err)
	}

	return nil
}

func operationalOverviewTwoModesDown(ctx context.Context, db *bun.DB) error {
	slog.Info("migration rollback restores untouched admin scopes",
		slog.String("migration", operationalOverviewTwoModesVersion),
		slog.String("detail", "restoring admins scopes without later setting changes"),
	)

	if _, err := db.NewRaw(`
		WITH migration_changes AS (
			SELECT id, tenant_id
			FROM config.setting_audit
			WHERE setting_key = ?
				AND action = 'set'
				AND changed_by IS NULL
				AND old_value = '"admins"'::jsonb
				AND new_value = '"own"'::jsonb
		), unchanged AS (
			SELECT migration_changes.tenant_id
			FROM migration_changes
			WHERE NOT EXISTS (
				SELECT 1
				FROM config.setting_audit AS later
				WHERE later.tenant_id = migration_changes.tenant_id
					AND later.setting_key = ?
					AND later.id > migration_changes.id
			)
		)
		UPDATE config.setting_values AS stored
		SET value = '"admins"'::jsonb
		FROM unchanged
		WHERE stored.tenant_id = unchanged.tenant_id
			AND stored.setting_key = ?
			AND stored.value = '"own"'::jsonb;
	`, operationalOverviewScopeKey, operationalOverviewScopeKey, operationalOverviewScopeKey).Exec(ctx); err != nil {
		return fmt.Errorf("restore operational overview admin scopes: %w", err)
	}

	return nil
}
