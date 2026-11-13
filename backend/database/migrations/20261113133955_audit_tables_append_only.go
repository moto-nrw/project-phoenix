package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const auditTablesAppendOnlyVersion = "20261113133955"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     auditTablesAppendOnlyVersion,
		Description: "Audit-Tabellen in config, active und platform nur noch anhängen; Löschen nur über die Löschfrist-Funktionen (#6844)",
		DependsOn: []string{
			configSettingValuesVersion,
			vacationWorkflowGrantsVersion,
			traegerSettingsVersion,
			traegerMasterDataVersion,
		},
	})
	Migrations.MustRegister(auditTablesAppendOnlyUp, auditTablesAppendOnlyDown)
}

// auditTablesAppendOnlySQL extends 1.15.225 (#1924) to the audit trails
// outside the audit schema (#6844). The default ACL of 1.14.1 hands
// phoenix_tenant UPDATE and DELETE on every new table in config and active,
// and the later narrow GRANTs only added to it; the Träger ledgers were
// granted DELETE for their inline pruning. A request under the tenant role
// could therefore change or erase its own audit rows.
//
// Expired rows now leave only through the SECURITY DEFINER functions in
// repeatable_config_delete_expired_setting_audit.go,
// repeatable_config_delete_expired_traeger_setting_audit.go and
// repeatable_platform_delete_expired_traeger_master_data_audit.go: each
// accepts only the school or Träger of the calling transaction and deletes
// with its fixed three-year window on the database clock.
// active.staff_absence_audit has no retention of its own; its rows go with
// their absence through ON DELETE CASCADE, which needs no privilege.
//
// Three change logs without "audit" in their name are append-only too: the
// vacation quota and leave allowance changes leave with
// users.erase_retired_staff_records, the meal sickness history with its
// child or the school erasure, all SECURITY DEFINER or cascades.
// education.grade_transition_history keeps UPDATE: the student deletion
// anonymizes its names (#2710).
//
// TestAuditTablesAppendOnlyForTenantRole guards every table named *audit*
// and these change logs.
const auditTablesAppendOnlySQL = `
REVOKE UPDATE, DELETE, TRUNCATE ON
	config.setting_audit,
	config.traeger_setting_audit,
	active.staff_absence_audit,
	platform.traeger_master_data_audit,
	active.staff_vacation_quota_changes,
	active.staff_absence_type_allowance_changes,
	schedule.meal_sickness_status_history
FROM phoenix_tenant;
`

func auditTablesAppendOnlyUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, auditTablesAppendOnlySQL); err != nil {
		return fmt.Errorf("make audit tables append-only: %w", err)
	}
	return nil
}

// TRUNCATE was never granted, so the rollback does not restore it.
const auditTablesAppendOnlyDownSQL = `
GRANT UPDATE, DELETE ON config.setting_audit, active.staff_absence_audit,
	active.staff_vacation_quota_changes, active.staff_absence_type_allowance_changes,
	schedule.meal_sickness_status_history TO phoenix_tenant;
GRANT DELETE ON config.traeger_setting_audit, platform.traeger_master_data_audit TO phoenix_tenant;
`

func auditTablesAppendOnlyDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, auditTablesAppendOnlyDownSQL); err != nil {
		return fmt.Errorf("restore audit table grants: %w", err)
	}
	return nil
}
