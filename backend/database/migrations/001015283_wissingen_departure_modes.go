package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	wissingenDepartureModesVersion     = "1.15.283"
	wissingenDepartureModesDescription = "Derive departure modes for approved Wissingen 2026/2027 enrollments from the confirmed legacy form answers"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     wissingenDepartureModesVersion,
		Description: wissingenDepartureModesDescription,
		DependsOn: []string{
			mfaChallengePortalBindingVersion, // 1.15.282
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return wissingenDepartureModesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return wissingenDepartureModesDown(ctx, db)
		},
	)
}

func wissingenDepartureModesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.283: Deriving departure modes for approved Wissingen 2026/2027 enrollments...")

	result, err := db.NewRaw(`
		WITH matching_children AS (
			SELECT
				COALESCE(child.created_student_id, child.matched_student_id) AS student_id,
				child.custom_data,
				COUNT(*) OVER (
					PARTITION BY COALESCE(child.created_student_id, child.matched_student_id)
				) AS matching_child_count
			FROM enrollment.request_children AS child
			JOIN enrollment.requests AS request
				ON request.id = child.request_id
				AND request.tenant_id = child.tenant_id
			JOIN enrollment.phases AS phase
				ON phase.id = request.phase_id
				AND phase.tenant_id = request.tenant_id
			JOIN enrollment.form_schemas AS schema
				ON schema.id = request.schema_id
				AND schema.tenant_id = request.tenant_id
			JOIN platform.schools AS school
				ON school.id = child.tenant_id
			WHERE school.slug = 'ogs-wissingen'
			  AND phase.name = 'Schuljahr 2026/2027'
			  AND schema.name = 'Schuljahr 2026/2027'
			  AND schema.version = 20
			  AND child.status = 'approved'
			  AND COALESCE(child.created_student_id, child.matched_student_id) IS NOT NULL
			  AND jsonb_typeof(child.custom_data -> 'schedule_pickup') = 'object'
			  AND jsonb_typeof(child.custom_data -> 'darf_ihr_kind_alleine_nach_hause_gehen') = 'boolean'
			  AND (
				child.custom_data -> 'fahrt_ihr_kind_mit_dem_bus_nach_hause' IS NULL
				OR jsonb_typeof(child.custom_data -> 'fahrt_ihr_kind_mit_dem_bus_nach_hause') = 'boolean'
			  )
			  AND (
				child.custom_data -> 'soll_ihr_kind_nur_gemeinsam_mit_einem_anderen_kind_gehen_durfen' IS NULL
				OR jsonb_typeof(child.custom_data -> 'soll_ihr_kind_nur_gemeinsam_mit_einem_anderen_kind_gehen_durfen') = 'boolean'
			  )
			  AND EXISTS (
				SELECT 1 FROM jsonb_array_elements(schema.fields) AS field
				WHERE field ->> 'key' = 'schedule_pickup'
				  AND field ->> 'type' = 'weekday_schedule'
				  AND field ->> 'target' = 'schedule.pickup'
			  )
			  AND EXISTS (
				SELECT 1 FROM jsonb_array_elements(schema.fields) AS field
				WHERE field ->> 'key' = 'darf_ihr_kind_alleine_nach_hause_gehen'
				  AND field ->> 'type' = 'boolean'
				  AND COALESCE(field ->> 'target', '') = ''
			  )
			  AND EXISTS (
				SELECT 1 FROM jsonb_array_elements(schema.fields) AS field
				WHERE field ->> 'key' = 'fahrt_ihr_kind_mit_dem_bus_nach_hause'
				  AND field ->> 'type' = 'boolean'
				  AND COALESCE(field ->> 'target', '') = ''
			  )
			  AND EXISTS (
				SELECT 1 FROM jsonb_array_elements(schema.fields) AS field
				WHERE field ->> 'key' = 'soll_ihr_kind_nur_gemeinsam_mit_einem_anderen_kind_gehen_durfen'
				  AND field ->> 'type' = 'boolean'
				  AND COALESCE(field ->> 'target', '') = ''
			  )
			  AND EXISTS (
				SELECT 1 FROM jsonb_array_elements(schema.fields) AS field
				WHERE field ->> 'key' = 'mit_welchem_kind_soll_ihr_kind_zusammen_gehen'
				  AND field ->> 'type' = 'text'
				  AND COALESCE(field ->> 'target', '') = ''
			  )
		),
		answers AS (
			SELECT
				student_id,
				custom_data,
				(custom_data ->> 'darf_ihr_kind_alleine_nach_hause_gehen')::boolean AS goes_alone,
				COALESCE((custom_data ->> 'fahrt_ihr_kind_mit_dem_bus_nach_hause')::boolean, false) AS goes_by_bus,
				COALESCE((custom_data ->> 'soll_ihr_kind_nur_gemeinsam_mit_einem_anderen_kind_gehen_durfen')::boolean, false) AS goes_accompanied
			FROM matching_children
			WHERE matching_child_count = 1
		),
		plans AS (
			SELECT
				answers.*,
				plan.allowed_departure_modes,
				plan.departure_days,
				plan.bus_days,
				plan.pickup_days
			FROM answers
			JOIN LATERAL (
				SELECT
					jsonb_object_agg(day, modes) AS allowed_departure_modes,
					COALESCE(jsonb_object_agg(day, departure_mode) FILTER (WHERE departure_mode IS NOT NULL), '{}'::jsonb) AS departure_days,
					COALESCE(jsonb_object_agg(day, true) FILTER (WHERE answers.goes_by_bus), '{}'::jsonb) AS bus_days,
					COALESCE(jsonb_object_agg(day, true) FILTER (WHERE NOT answers.goes_alone), '{}'::jsonb) AS pickup_days
				FROM (
					SELECT
						entry.key AS day,
						CASE
							WHEN NOT answers.goes_alone THEN jsonb_build_array('pickup')
							ELSE jsonb_build_array('alone')
								|| CASE WHEN answers.goes_by_bus THEN jsonb_build_array('bus') ELSE '[]'::jsonb END
								|| CASE WHEN answers.goes_accompanied THEN jsonb_build_array('accompanied') ELSE '[]'::jsonb END
						END AS modes,
						CASE
							WHEN NOT answers.goes_alone THEN 'pickup'
							WHEN answers.goes_by_bus THEN 'bus'
							WHEN answers.goes_accompanied THEN 'accompanied'
						END AS departure_mode
					FROM jsonb_each(answers.custom_data -> 'schedule_pickup') AS entry(key, value)
					WHERE entry.key IN ('mon', 'tue', 'wed', 'thu', 'fri')
					  AND jsonb_typeof(entry.value) = 'string'
					  AND btrim(entry.value #>> '{}') <> ''
				) AS days
				HAVING COUNT(*) > 0
			) AS plan ON true
			WHERE NOT goes_accompanied
			   OR (
				jsonb_typeof(custom_data -> 'mit_welchem_kind_soll_ihr_kind_zusammen_gehen') = 'string'
				AND btrim(custom_data ->> 'mit_welchem_kind_soll_ihr_kind_zusammen_gehen') <> ''
			   )
		)
		UPDATE users.students AS student
		SET allowed_departure_modes = plans.allowed_departure_modes,
			departure_days = plans.departure_days,
			bus_days = plans.bus_days,
			pickup_days = plans.pickup_days,
			pickup_status = CASE
				WHEN NOT plans.goes_alone THEN 'Wird abgeholt'
				WHEN plans.goes_accompanied THEN 'Geht mit anderem Kind'
				ELSE 'Geht alleine nach Hause'
			END,
			departure_companion_note = CASE
				WHEN plans.goes_accompanied THEN left(btrim(plans.custom_data ->> 'mit_welchem_kind_soll_ihr_kind_zusammen_gehen'), 255)
				ELSE NULL
			END,
			updated_at = NOW()
		FROM plans
		WHERE student.id = plans.student_id
		  AND student.tenant_id = (
			SELECT id FROM platform.schools WHERE slug = 'ogs-wissingen'
		  )
		  AND COALESCE(student.allowed_departure_modes, '{}'::jsonb) = '{}'::jsonb
		  AND COALESCE(student.departure_days, '{}'::jsonb) = '{}'::jsonb
		  AND COALESCE(student.bus_days, '{}'::jsonb) = '{}'::jsonb
		  AND COALESCE(student.pickup_days, '{}'::jsonb) = '{}'::jsonb;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed deriving Wissingen departure modes: %w", err)
	}

	if affected, rowsErr := result.RowsAffected(); rowsErr == nil {
		fmt.Printf("Migration 1.15.283: updated %d Wissingen student departure plan(s)\n", affected)
	}
	return nil
}

func wissingenDepartureModesDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.283: derived departure plans are not cleared automatically")
	return nil
}
