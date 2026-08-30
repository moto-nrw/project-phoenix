package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	templateSourceSchoolClassesVersion     = "1.15.314"
	templateSourceSchoolClassesDescription = "Add activities.groups.source_school_classes so an offering-sourced Regeltermin can be narrowed to concrete Schulklassen (#2482)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     templateSourceSchoolClassesVersion,
		Description: templateSourceSchoolClassesDescription,
		DependsOn: []string{
			templateMultiOfferingSourceVersion, // 1.15.281
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return templateSourceSchoolClassesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return templateSourceSchoolClassesDown(ctx, db)
		},
	)
}

func templateSourceSchoolClassesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.314: Adding activities.groups.source_school_classes...")

	// OGS am Berg runs ONE Betreuungsangebot "Randstunde" and six Regeltermine,
	// one per Schulklasse (1a…2c), each on its own weekdays (#2482). The
	// existing Jahrgang filter cannot express that — "Jahrgang 1" would put 1a,
	// 1b and 1c into the same Termin — so the school kept curating the class
	// lists by hand and later approvals never reached the plan.
	//
	// Free-text class names, matched case- and whitespace-insensitively via
	// internal/schoolclass.Normalize, mirroring activities.groups.
	// target_school_class. The array is stored as jsonb like its two sibling
	// source_* columns.
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
			ADD COLUMN IF NOT EXISTS source_school_classes JSONB;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding source_school_classes to activities.groups: %w", err)
	}

	// Extends chk_activities_groups_offering_source (1.15.281) by the class
	// filter: it needs a source just like the grade filter does, and the two
	// filters are mutually exclusive — a class already implies its Jahrgang, so
	// combining them is either redundant ("1b" + Jahrgang 1) or empty ("1b" +
	// Jahrgang 2). Rejecting the combination is what makes the filter semantics
	// unambiguous instead of leaving an AND/OR question open (#2482).
	//
	// The explicit IS NOT NULL conjuncts are load-bearing for the same reason
	// as in 1.15.281: jsonb_typeof(NULL) is NULL, and a CHECK treats NULL as
	// satisfied.
	_, err = db.NewRaw(`
		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS chk_activities_groups_offering_source;

		ALTER TABLE activities.groups
			ADD CONSTRAINT chk_activities_groups_offering_source
			CHECK (
				(
					source_care_offering_ids IS NULL
					AND source_grade_levels IS NULL
					AND source_school_classes IS NULL
				)
				OR (
					source_care_offering_ids IS NOT NULL
					AND jsonb_typeof(source_care_offering_ids) = 'array'
					AND jsonb_array_length(source_care_offering_ids) > 0
					AND target_group_type = 'angebot'
					AND NOT (
						source_grade_levels IS NOT NULL
						AND source_school_classes IS NOT NULL
					)
					AND (
						source_school_classes IS NULL
						OR (
							jsonb_typeof(source_school_classes) = 'array'
							AND jsonb_array_length(source_school_classes) > 0
						)
					)
				)
			);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed extending offering-source check constraint: %w", err)
	}

	fmt.Println("Migration 1.15.314: Completed successfully")
	return nil
}

func templateSourceSchoolClassesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.314: Dropping activities.groups.source_school_classes...")

	// Class-filtered templates lose their filter on rollback: the pre-1.15.314
	// shape cannot express it, and keeping the column while restoring the old
	// CHECK is not an option either. Widening them to "all children of the
	// offering" would silently plan the wrong children, so the whole source is
	// dropped instead — the Termin falls back to its manual roster and the
	// admin has to reselect a source, which is the visible outcome.
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS chk_activities_groups_offering_source;

		-- Mirror the offering-source reconciler before removing the source:
		-- only purely planned future rows lose their stale roster membership.
		-- Observed attendance and manual per-occurrence decisions stay intact.
		DELETE FROM schedule.instance_students AS "instance_student"
		USING schedule.activity_instances AS "instance",
			activities.student_enrollments AS enrollment,
			activities.groups AS "group"
		WHERE "instance_student".instance_id = "instance".id
			AND "instance_student".tenant_id = "instance".tenant_id
			AND "instance".activity_group_id = "group".id
			AND "instance".tenant_id = "group".tenant_id
			AND enrollment.activity_group_id = "group".id
			AND enrollment.tenant_id = "group".tenant_id
			AND "instance_student".student_id = enrollment.student_id
			AND "group".source_school_classes IS NOT NULL
			AND enrollment.enrollment_request_child_id IS NOT NULL
			AND "instance".date >= (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date
			AND enrollment.valid_from <= "instance".date
			AND (enrollment.valid_until IS NULL OR enrollment.valid_until > "instance".date)
			AND "instance".status = 'planned'
			AND "instance".calendar_period_id IS NOT NULL
			AND "instance".is_spontaneous = FALSE
			AND "instance_student".is_unplanned = FALSE
			AND "instance_student".checked_in_at IS NULL
			AND "instance_student".checked_out_at IS NULL
			AND "instance_student".manual_status_at IS NULL
			AND (
				"instance_student".status = 'expected'
				OR "instance_student".student_status_day_id IS NOT NULL
				OR "instance_student".pickup_exception_id IS NOT NULL
			);

		DELETE FROM activities.student_enrollments AS enrollment
		USING activities.groups AS "group"
		WHERE enrollment.activity_group_id = "group".id
			AND enrollment.tenant_id = "group".tenant_id
			AND "group".source_school_classes IS NOT NULL
			AND enrollment.enrollment_request_child_id IS NOT NULL
			AND enrollment.valid_from >= (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date;

		UPDATE activities.student_enrollments AS enrollment
		SET valid_until = (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date
		FROM activities.groups AS "group"
		WHERE enrollment.activity_group_id = "group".id
			AND enrollment.tenant_id = "group".tenant_id
			AND "group".source_school_classes IS NOT NULL
			AND enrollment.enrollment_request_child_id IS NOT NULL
			AND enrollment.valid_from < (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date
			AND (enrollment.valid_until IS NULL OR enrollment.valid_until > (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date);

		UPDATE activities.groups
		SET source_care_offering_ids = NULL,
			source_grade_levels = NULL
		WHERE source_school_classes IS NOT NULL;

		ALTER TABLE activities.groups
			DROP COLUMN IF EXISTS source_school_classes;

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
		return fmt.Errorf("failed dropping source_school_classes from activities.groups: %w", err)
	}

	return nil
}
