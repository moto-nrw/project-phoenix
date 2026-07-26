package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	categoryShiftTypeVersion     = "1.15.195"
	categoryShiftTypeDescription = "Link activities.categories to schedule.shift_types via optional shift_type_id (#1837 follow-up, #1836)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     categoryShiftTypeVersion,
		Description: categoryShiftTypeDescription,
		DependsOn: []string{
			shiftTypesVersion,
			ActivitiesCategoriesVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return categoryShiftTypeUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return categoryShiftTypeDown(ctx, db)
		},
	)
}

func categoryShiftTypeUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.195: Linking activities.categories to schedule.shift_types...")

	// Optional cross-schema tenant-safe FK: a Timetable-Kategorie may map to at
	// most one Dienstplan-Schichtart. NULL = no mapping (existing data unchanged).
	// The composite (tenant_id, shift_type_id) FK plus column-scoped SET NULL
	// keeps the tenant boundary intact when a shift type is deleted (mirrors the
	// staff_shifts link in 1.15.170 and the group calendar-period FK in 1.15.185).
	// The parent unique index (tenant_id, id) already exists on
	// schedule.shift_types (uniq_shift_types_tenant_id).
	_, err := db.NewRaw(`
		ALTER TABLE activities.categories
		ADD COLUMN IF NOT EXISTS shift_type_id BIGINT;

		ALTER TABLE activities.categories
			DROP CONSTRAINT IF EXISTS fk_activities_categories_shift_type,
			ADD CONSTRAINT fk_activities_categories_shift_type
				FOREIGN KEY (tenant_id, shift_type_id)
				REFERENCES schedule.shift_types (tenant_id, id)
				ON DELETE SET NULL (shift_type_id);

		CREATE INDEX IF NOT EXISTS idx_activities_categories_shift_type
			ON activities.categories (tenant_id, shift_type_id);

		COMMENT ON COLUMN activities.categories.shift_type_id IS 'Optional Dienstplan-Schichtart this Timetable-Kategorie maps to; NULL = no mapping. Managed from the "Schichtarten verwalten" modal.';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed linking categories to shift types: %w", err)
	}

	return nil
}

func categoryShiftTypeDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.195: Removing category shift_type_id link...")

	_, err := db.NewRaw(`
		ALTER TABLE activities.categories DROP CONSTRAINT IF EXISTS fk_activities_categories_shift_type;
		DROP INDEX IF EXISTS activities.idx_activities_categories_shift_type;
		ALTER TABLE activities.categories DROP COLUMN IF EXISTS shift_type_id;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed removing category shift_type_id link: %w", err)
	}

	return nil
}
