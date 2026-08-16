package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	removeStudentGroupScopeSettingsVersion     = "1.15.296"
	removeStudentGroupScopeSettingsDescription = "Remove the per-group student scope settings (gdpr.student_data_scope, gdpr.attendance_log_scope, attendance.web_checkin_access)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     removeStudentGroupScopeSettingsVersion,
		Description: removeStudentGroupScopeSettingsDescription,
		DependsOn: []string{
			"1.15.25", // settings tables (config.setting_values + config.setting_audit)
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return removeStudentGroupScopeSettingsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeStudentGroupScopeSettingsDown(ctx, db)
		},
	)
}

// removeStudentGroupScopeSettingsUp deletes the stored tenant overrides and
// audit rows of the three settings that scoped student access to the caller's
// supervised groups. #2329 removed the group scope entirely — verified staff
// read and write every child of their tenant, the Betreuer/Admin split lives
// in the permission catalog — so the registry definitions are gone and any
// remaining rows would be orphans the settings UI can no longer display.
func removeStudentGroupScopeSettingsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.296: Removing student group scope settings...")

	keys := []string{
		"gdpr.student_data_scope",
		"gdpr.attendance_log_scope",
		"attendance.web_checkin_access",
	}

	if _, err := db.NewRaw(
		`DELETE FROM config.setting_values WHERE setting_key IN (?);`,
		bun.List(keys),
	).Exec(ctx); err != nil {
		return fmt.Errorf("failed deleting scope setting values: %w", err)
	}

	if _, err := db.NewRaw(
		`DELETE FROM config.setting_audit WHERE setting_key IN (?);`,
		bun.List(keys),
	).Exec(ctx); err != nil {
		return fmt.Errorf("failed deleting scope setting audit rows: %w", err)
	}

	return nil
}

// removeStudentGroupScopeSettingsDown is a no-op: the deleted overrides are
// gone, and re-registering the settings is a code-level change this migration
// cannot restore.
func removeStudentGroupScopeSettingsDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.296: nothing to restore (setting definitions were removed in code)")
	return nil
}
