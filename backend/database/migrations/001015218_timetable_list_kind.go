package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	timetableListKindVersion     = "1.15.218"
	timetableListKindDescription = "Add timetable list kind to activity templates and instances"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     timetableListKindVersion,
		Description: timetableListKindDescription,
		DependsOn:   []string{studentCompanionsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return timetableListKindUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return timetableListKindDown(ctx, db)
		},
	)
}

func timetableListKindUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.218: Adding timetable list_kind columns...")

	statements := []string{
		`ALTER TABLE activities.groups ADD COLUMN IF NOT EXISTS list_kind TEXT`,
		`ALTER TABLE schedule.activity_instances ADD COLUMN IF NOT EXISTS list_kind TEXT`,
		`ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS chk_activity_groups_list_kind,
			ADD CONSTRAINT chk_activity_groups_list_kind
				CHECK (list_kind IS NULL OR list_kind IN ('edge_hours', 'learning_time', 'activity', 'mensa'))`,
		`ALTER TABLE schedule.activity_instances
			DROP CONSTRAINT IF EXISTS chk_activity_instances_list_kind,
			ADD CONSTRAINT chk_activity_instances_list_kind
				CHECK (list_kind IS NULL OR list_kind IN ('edge_hours', 'learning_time', 'activity', 'mensa'))`,
		`CREATE INDEX IF NOT EXISTS idx_activity_groups_tenant_list_kind
			ON activities.groups(tenant_id, list_kind)
			WHERE list_kind IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_activity_instances_tenant_date_list_kind
			ON schedule.activity_instances(tenant_id, date, list_kind)
			WHERE list_kind IS NOT NULL`,
	}

	for _, statement := range statements {
		if _, err := db.NewRaw(statement).Exec(ctx); err != nil {
			return fmt.Errorf("failed adding timetable list_kind columns: %w", err)
		}
	}

	return nil
}

func timetableListKindDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.218: Removing timetable list_kind columns...")

	statements := []string{
		`DROP INDEX IF EXISTS activities.idx_activity_groups_tenant_list_kind`,
		`DROP INDEX IF EXISTS schedule.idx_activity_instances_tenant_date_list_kind`,
		`ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS chk_activity_groups_list_kind,
			DROP COLUMN IF EXISTS list_kind`,
		`ALTER TABLE schedule.activity_instances
			DROP CONSTRAINT IF EXISTS chk_activity_instances_list_kind,
			DROP COLUMN IF EXISTS list_kind`,
	}

	for _, statement := range statements {
		if _, err := db.NewRaw(statement).Exec(ctx); err != nil {
			return fmt.Errorf("failed removing timetable list_kind columns: %w", err)
		}
	}

	return nil
}
