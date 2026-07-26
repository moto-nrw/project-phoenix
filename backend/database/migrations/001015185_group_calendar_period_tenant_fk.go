package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	groupCalendarPeriodTenantFKVersion     = "1.15.185"
	groupCalendarPeriodTenantFKDescription = "Enforce tenant-safe activity-group calendar-period references"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     groupCalendarPeriodTenantFKVersion,
		Description: groupCalendarPeriodTenantFKDescription,
		DependsOn:   []string{groupTargetGroupConstraintsVersion},
	})
	Migrations.MustRegister(groupCalendarPeriodTenantFKUp, groupCalendarPeriodTenantFKDown)
}

func groupCalendarPeriodTenantFKUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.185: Enforcing tenant-safe group calendar-period references...")
	_, err := db.NewRaw(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_calendar_periods_tenant_id_id
			ON schedule.calendar_periods (tenant_id, id);

		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS groups_calendar_period_id_fkey,
			DROP CONSTRAINT IF EXISTS fk_activities_groups_calendar_period_tenant,
			ADD CONSTRAINT fk_activities_groups_calendar_period_tenant
				FOREIGN KEY (tenant_id, calendar_period_id)
				REFERENCES schedule.calendar_periods (tenant_id, id)
				ON DELETE SET NULL (calendar_period_id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed enforcing tenant-safe group calendar-period references: %w", err)
	}
	fmt.Println("Migration 1.15.185: Completed successfully")
	return nil
}

func groupCalendarPeriodTenantFKDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.185: Restoring the legacy group calendar-period reference...")
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS fk_activities_groups_calendar_period_tenant,
			ADD CONSTRAINT groups_calendar_period_id_fkey
				FOREIGN KEY (calendar_period_id)
				REFERENCES schedule.calendar_periods (id)
				ON DELETE SET NULL;

		DROP INDEX IF EXISTS schedule.idx_calendar_periods_tenant_id_id;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed restoring legacy group calendar-period reference: %w", err)
	}
	return nil
}
