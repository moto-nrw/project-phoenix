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

// Stored values may have been changed after rollout and the audit table has no
// migration identifier. Keeping the explicit values is safer than guessing
// which tenant changes a rollback should undo.
func operationalOverviewTwoModesDown(_ context.Context, _ *bun.DB) error {
	slog.Info("migration rollback keeps existing values",
		slog.String("migration", operationalOverviewTwoModesVersion),
		slog.String("detail", "operational overview scope values are retained"),
	)
	return nil
}
