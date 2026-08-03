package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	templateOfferingSourceVersion     = "1.15.262"
	templateOfferingSourceDescription = "Add source_care_offering_id/source_grade_levels to activities.groups so a Betreuungsangebot can feed multiple filtered Regeltermine (#2137)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     templateOfferingSourceVersion,
		Description: templateOfferingSourceDescription,
		DependsOn: []string{
			openingBalancesVersion, // 1.15.261
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return templateOfferingSourceUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return templateOfferingSourceDown(ctx, db)
		},
	)
}

func templateOfferingSourceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.262: Adding offering-source columns to activities.groups...")

	// Inverts the CareOffering.ActivityGroupID bridge (#1651): a template may
	// declare ONE care offering as its roster source plus an optional grade
	// filter, and the same offering may feed many templates. The legacy
	// offering->template link stays untouched for existing setups.
	//
	// source_grade_levels is JSONB (int array) rather than a smallint[] to
	// match the established jsonb convention in the enrollment domain
	// (care_offerings.auto_add_grade_levels). NULL or [] means "all children
	// of the offering".
	//
	// ON DELETE SET NULL: deleting an offering degrades its sourced templates
	// to plain manually-curated rosters instead of blocking the delete or
	// dropping the templates. Already-materialized enrollment rows stay.
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
			ADD COLUMN IF NOT EXISTS source_care_offering_id BIGINT
				REFERENCES enrollment.care_offerings(id) ON DELETE SET NULL,
			ADD COLUMN IF NOT EXISTS source_grade_levels JSONB;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding offering-source columns to activities.groups: %w", err)
	}

	// The source only makes sense for the 'angebot' Zielgruppe, and a grade
	// filter only together with a source.
	_, err = db.NewRaw(`
		ALTER TABLE activities.groups
			ADD CONSTRAINT chk_activities_groups_offering_source
			CHECK (
				(source_care_offering_id IS NULL AND source_grade_levels IS NULL)
				OR (source_care_offering_id IS NOT NULL AND target_group_type = 'angebot')
			);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding offering-source check constraint: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_activities_groups_source_offering
			ON activities.groups (tenant_id, source_care_offering_id)
			WHERE source_care_offering_id IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating activities.groups source offering index: %w", err)
	}

	fmt.Println("Migration 1.15.262: Completed successfully")
	return nil
}

func templateOfferingSourceDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.262: Dropping offering-source columns from activities.groups...")

	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS activities.idx_activities_groups_source_offering;

		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS chk_activities_groups_offering_source;

		ALTER TABLE activities.groups
			DROP COLUMN IF EXISTS source_care_offering_id,
			DROP COLUMN IF EXISTS source_grade_levels;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping offering-source columns from activities.groups: %w", err)
	}

	return nil
}
