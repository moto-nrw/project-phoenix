package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	operationalOverviewScopeVersion     = "1.15.322"
	operationalOverviewScopeDescription = "Introduce operations.operational_overview_scope and retire operations.admin_supervision_overview"

	operationalOverviewScopeKey   = "operations.operational_overview_scope"
	adminSupervisionOverviewKey   = "operations.admin_supervision_overview"
	operationalOverviewGroupModeK = "operations.group_mode"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     operationalOverviewScopeVersion,
		Description: operationalOverviewScopeDescription,
		DependsOn: []string{
			"1.15.25", // settings tables (config.setting_values + config.setting_audit)
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return operationalOverviewScopeUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return operationalOverviewScopeDown(ctx, db)
		},
	)
}

// operationalOverviewScopeUp folds the two rules that used to decide who may
// see every running module into the single setting that now owns that
// decision (#2380):
//
//	operations.group_mode = open_care        -> scope all_staff
//	operations.admin_supervision_overview    -> scope admins
//	neither                                  -> scope own (registry default)
//
// Open care wins over the admin flag because it was the broader of the two —
// no school loses access it had before. Schools already on the restrictive
// default get no row at all; the registry default answers for them.
//
// The organisational group mode itself is untouched: after this migration it
// only describes how the school organises children, and a school that later
// switches to open care must open the operational view deliberately.
func operationalOverviewScopeUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.322: Deriving operations.operational_overview_scope from group mode and admin overview...")

	if _, err := db.NewRaw(`
		WITH derived AS (
			SELECT
				s.id AS tenant_id,
				CASE
					WHEN gm.value #>> '{}' = 'open_care' THEN 'all_staff'
					WHEN aso.value = 'true'::jsonb THEN 'admins'
				END AS scope
			FROM platform.schools AS s
			LEFT JOIN config.setting_values AS gm
				ON gm.tenant_id = s.id AND gm.setting_key = ?
			LEFT JOIN config.setting_values AS aso
				ON aso.tenant_id = s.id AND aso.setting_key = ?
		), inserted AS (
			INSERT INTO config.setting_values (tenant_id, setting_key, value)
			SELECT tenant_id, ?, to_jsonb(scope)
			FROM derived
			WHERE scope IS NOT NULL
			ON CONFLICT (tenant_id, setting_key) DO NOTHING
			RETURNING tenant_id, setting_key, value
		)
		INSERT INTO config.setting_audit (
			tenant_id, setting_key, old_value, new_value, action, changed_by
		)
		SELECT tenant_id, setting_key, NULL, value, 'set', NULL
		FROM inserted;
	`, operationalOverviewGroupModeK, adminSupervisionOverviewKey, operationalOverviewScopeKey).Exec(ctx); err != nil {
		return fmt.Errorf("failed deriving operational overview scope: %w", err)
	}

	// The old key has no registry definition any more, so a leftover row would
	// be an orphan the settings UI can neither show nor reset.
	if _, err := db.NewRaw(
		`DELETE FROM config.setting_values WHERE setting_key = ?;`,
		adminSupervisionOverviewKey,
	).Exec(ctx); err != nil {
		return fmt.Errorf("failed deleting admin supervision overview values: %w", err)
	}

	if _, err := db.NewRaw(
		`DELETE FROM config.setting_audit WHERE setting_key = ?;`,
		adminSupervisionOverviewKey,
	).Exec(ctx); err != nil {
		return fmt.Errorf("failed deleting admin supervision overview audit rows: %w", err)
	}

	return nil
}

// operationalOverviewScopeDown restores the admin flag for the tenants whose
// derived scope is 'admins'. The all_staff tenants need nothing restored:
// their access came from operations.group_mode, which this migration never
// changed, so an old binary reads open care again and behaves as before.
func operationalOverviewScopeDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.322: restoring operations.admin_supervision_overview...")

	if _, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value)
		SELECT tenant_id, ?, 'true'::jsonb
		FROM config.setting_values
		WHERE setting_key = ? AND value = to_jsonb('admins'::text)
		ON CONFLICT (tenant_id, setting_key) DO NOTHING;
	`, adminSupervisionOverviewKey, operationalOverviewScopeKey).Exec(ctx); err != nil {
		return fmt.Errorf("failed restoring admin supervision overview values: %w", err)
	}

	if _, err := db.NewRaw(
		`DELETE FROM config.setting_values WHERE setting_key = ?;`,
		operationalOverviewScopeKey,
	).Exec(ctx); err != nil {
		return fmt.Errorf("failed deleting operational overview scope values: %w", err)
	}

	if _, err := db.NewRaw(
		`DELETE FROM config.setting_audit WHERE setting_key = ?;`,
		operationalOverviewScopeKey,
	).Exec(ctx); err != nil {
		return fmt.Errorf("failed deleting operational overview scope audit rows: %w", err)
	}

	return nil
}
