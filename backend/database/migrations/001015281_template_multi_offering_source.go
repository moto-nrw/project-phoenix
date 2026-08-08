package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	templateMultiOfferingSourceVersion     = "1.15.281"
	templateMultiOfferingSourceDescription = "Replace activities.groups.source_care_offering_id with the source_care_offering_ids jsonb array so one Regeltermin can union several Betreuungsangebote as its roster source"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     templateMultiOfferingSourceVersion,
		Description: templateMultiOfferingSourceDescription,
		DependsOn: []string{
			templateOfferingSourceVersion, // 1.15.266
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return templateMultiOfferingSourceUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return templateMultiOfferingSourceDown(ctx, db)
		},
	)
}

func templateMultiOfferingSourceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.281: Converting activities.groups offering source to a jsonb id array...")

	// A Regeltermin may now union SEVERAL care offerings as its roster source
	// (customer request on top of #2137): a block that holds the children of
	// "Betreuung bis 14:30" AND "bis 16:00" AND "montags Musikunterricht" at
	// once, then filtered by Jahrgang. The single-FK column becomes a jsonb
	// int array, matching the established jsonb convention of the sibling
	// source_grade_levels column and enrollment.care_offerings.
	//
	// Trade-off, decided deliberately: the array cannot carry a foreign key,
	// so the ON DELETE SET NULL backstop of 1.15.266 disappears. That backstop
	// was never the load-bearing cleanup — the care-offering and phase delete
	// flows run DetachTemplatesSourcedFromOffering BEFORE the row delete
	// (#2147 review round 11), which retires the offering-derived enrollment
	// rows, reconciles materialized occurrences, and now also rewrites the
	// template's id array itself (removing just the deleted offering and
	// keeping the remaining sources). The orphaned-filter trigger of 1.15.266
	// existed only to accompany the FK's SET NULL and goes with it.
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
			ADD COLUMN IF NOT EXISTS source_care_offering_ids JSONB;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding source_care_offering_ids to activities.groups: %w", err)
	}

	_, err = db.NewRaw(`
		UPDATE activities.groups
		SET source_care_offering_ids = jsonb_build_array(source_care_offering_id)
		WHERE source_care_offering_id IS NOT NULL
		  AND source_care_offering_ids IS NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed backfilling source_care_offering_ids: %w", err)
	}

	_, err = db.NewRaw(`
		DROP TRIGGER IF EXISTS trg_groups_clear_orphaned_source_filter ON activities.groups;
		DROP FUNCTION IF EXISTS activities.groups_clear_orphaned_source_filter();

		DROP INDEX IF EXISTS activities.idx_activities_groups_source_offering;

		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS chk_activities_groups_offering_source;

		ALTER TABLE activities.groups
			DROP COLUMN IF EXISTS source_care_offering_id;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping legacy single-source column from activities.groups: %w", err)
	}

	// Same invariant as 1.15.266, expressed over the array: a source only on
	// the 'angebot' Zielgruppe, a grade filter only together with a source,
	// and never an empty array (empty means "no source" and must be NULL so
	// the IS NOT NULL lookups stay honest). The explicit IS NOT NULL conjunct
	// is load-bearing: without it, NULL ids next to a non-NULL filter make
	// jsonb_typeof(NULL) evaluate the branch to NULL, which a CHECK treats as
	// satisfied — the filter-without-source row 1.15.266 rejected would slip
	// through.
	_, err = db.NewRaw(`
		ALTER TABLE activities.groups
			ADD CONSTRAINT chk_activities_groups_offering_source
			CHECK (
				(source_care_offering_ids IS NULL AND source_grade_levels IS NULL)
				OR (
					source_care_offering_ids IS NOT NULL
					AND jsonb_typeof(source_care_offering_ids) = 'array'
					AND jsonb_array_length(source_care_offering_ids) > 0
					AND target_group_type = 'angebot'
				)
			);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding offering-source array check constraint: %w", err)
	}

	// Containment lookups ("every template sourcing offering X" — decision
	// fan-out, detach, offering edits) run source_care_offering_ids @> to_jsonb(X).
	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_activities_groups_source_offerings
			ON activities.groups USING gin (source_care_offering_ids jsonb_path_ops)
			WHERE source_care_offering_ids IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating activities.groups source offerings index: %w", err)
	}

	fmt.Println("Migration 1.15.281: Completed successfully")
	return nil
}

func templateMultiOfferingSourceDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.281: Restoring single source_care_offering_id column...")

	// Multi-source templates collapse to their FIRST source on rollback; the
	// remaining ids cannot be represented in the single-FK shape.
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
			ADD COLUMN IF NOT EXISTS source_care_offering_id BIGINT
				REFERENCES enrollment.care_offerings(id) ON DELETE SET NULL;

		UPDATE activities.groups
		SET source_care_offering_id = (source_care_offering_ids ->> 0)::BIGINT
		WHERE source_care_offering_ids IS NOT NULL
		  AND jsonb_array_length(source_care_offering_ids) > 0;

		DROP INDEX IF EXISTS activities.idx_activities_groups_source_offerings;

		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS chk_activities_groups_offering_source;

		ALTER TABLE activities.groups
			DROP COLUMN IF EXISTS source_care_offering_ids;

		ALTER TABLE activities.groups
			ADD CONSTRAINT chk_activities_groups_offering_source
			CHECK (
				(source_care_offering_id IS NULL AND source_grade_levels IS NULL)
				OR (source_care_offering_id IS NOT NULL AND target_group_type = 'angebot')
			);

		CREATE OR REPLACE FUNCTION activities.groups_clear_orphaned_source_filter()
		RETURNS trigger AS $$
		BEGIN
			NEW.source_grade_levels := NULL;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		DROP TRIGGER IF EXISTS trg_groups_clear_orphaned_source_filter ON activities.groups;
		CREATE TRIGGER trg_groups_clear_orphaned_source_filter
			BEFORE UPDATE OF source_care_offering_id ON activities.groups
			FOR EACH ROW
			WHEN (NEW.source_care_offering_id IS NULL AND NEW.source_grade_levels IS NOT NULL)
			EXECUTE FUNCTION activities.groups_clear_orphaned_source_filter();

		CREATE INDEX IF NOT EXISTS idx_activities_groups_source_offering
			ON activities.groups (tenant_id, source_care_offering_id)
			WHERE source_care_offering_id IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed restoring single-source column on activities.groups: %w", err)
	}

	return nil
}
