package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	archiveTimetableTemplatesVersion     = "1.15.41"
	archiveTimetableTemplatesDescription = "Add archived_at to activity templates for timetable template archiving"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     archiveTimetableTemplatesVersion,
		Description: archiveTimetableTemplatesDescription,
		DependsOn:   []string{backfillDefaultCalendarPeriodsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return archiveTimetableTemplatesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return archiveTimetableTemplatesDown(ctx, db)
		},
	)
}

func archiveTimetableTemplatesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.41: Adding archived_at to activities.groups...")
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
			ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

		CREATE INDEX IF NOT EXISTS idx_activity_groups_active_templates
			ON activities.groups (tenant_id, is_template, archived_at)
			WHERE is_template = true AND archived_at IS NULL;
	`).Exec(ctx)
	return err
}

func archiveTimetableTemplatesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.41: Removing archived_at from activities.groups...")
	_, _ = db.NewRaw(`
		DROP INDEX IF EXISTS activities.idx_activity_groups_active_templates;
		ALTER TABLE activities.groups
			DROP COLUMN IF EXISTS archived_at;
	`).Exec(ctx)
	return nil
}
