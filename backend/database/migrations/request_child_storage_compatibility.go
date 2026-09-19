package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// installRequestChildStorageCompatibility runs inside the final-delta lock and
// transaction. The compatibility shape is retained for previous-image rollback.
func installRequestChildStorageCompatibility(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, `
 ALTER TABLE enrollment.request_child_offerings RENAME TO request_child_offerings_legacy;
		CREATE SEQUENCE enrollment.request_child_compatibility_reads;
		CREATE SEQUENCE enrollment.request_child_compatibility_writes;
		GRANT USAGE ON SEQUENCE enrollment.request_child_compatibility_reads, enrollment.request_child_compatibility_writes TO phoenix_tenant;
		GRANT USAGE, SELECT ON SEQUENCE enrollment.request_child_compatibility_reads, enrollment.request_child_compatibility_writes TO phoenix_admin;
		ALTER TABLE enrollment.request_child_offerings_legacy
			DROP CONSTRAINT request_child_offerings_non_overlapping_validity;
 `)
	if err != nil {
		return err
	}
	return refreshRequestChildStorageCompatibility(ctx, tx)
}

// Recovery reuses the same view and routing definitions without replacing data
// or resetting hit counters. CREATE OR REPLACE preserves the rollback shape.
func refreshRequestChildStorageCompatibility(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, `

		CREATE OR REPLACE FUNCTION enrollment.request_child_legacy_manual(selected jsonb, manual jsonb, automatic jsonb)
		RETURNS jsonb LANGUAGE sql IMMUTABLE PARALLEL SAFE SET search_path = pg_catalog AS $function$
			SELECT CASE WHEN COALESCE(manual, '[]'::jsonb) IN ('[]'::jsonb, 'null'::jsonb)
				AND COALESCE(automatic, '[]'::jsonb) IN ('[]'::jsonb, 'null'::jsonb)
				THEN selected ELSE manual END
		$function$;

		CREATE OR REPLACE FUNCTION enrollment.request_child_effective_days(manual jsonb, automatic jsonb)
		RETURNS jsonb LANGUAGE sql IMMUTABLE PARALLEL SAFE SET search_path = pg_catalog AS $function$
			SELECT CASE WHEN manual IS NULL AND automatic IS NULL THEN NULL ELSE COALESCE((
				SELECT jsonb_agg(day ORDER BY array_position(ARRAY['mon','tue','wed','thu','fri','sat','sun'], day), day)
				FROM (
					SELECT DISTINCT jsonb_array_elements_text(
						CASE WHEN jsonb_typeof(manual) = 'array' THEN manual ELSE '[]'::jsonb END ||
						CASE WHEN jsonb_typeof(automatic) = 'array' THEN automatic ELSE '[]'::jsonb END
					) AS day
				) days
			), '[]'::jsonb) END
		$function$;



		CREATE OR REPLACE VIEW enrollment.request_child_offerings WITH (security_invoker = true) AS
		SELECT b.id, b.tenant_id, b.request_child_id, b.care_offering_id,
			CASE WHEN (b.archived).id IS NOT NULL
				AND b.manual_selected_days IS NOT DISTINCT FROM enrollment.request_child_legacy_manual((b.archived).selected_days, (b.archived).manual_selected_days, (b.archived).automatic_selected_days)
				AND b.automatic_selected_days IS NOT DISTINCT FROM (b.archived).automatic_selected_days
				THEN (b.archived).selected_days
				ELSE enrollment.request_child_effective_days(b.manual_selected_days, b.automatic_selected_days)
			END AS selected_days,
			CASE WHEN (b.archived).id IS NOT NULL THEN (b.archived).notes ELSE b.submission_notes END AS notes,
			b.created_at, b.updated_at,
			CASE WHEN (b.archived).id IS NOT NULL
				AND b.manual_selected_days IS NOT DISTINCT FROM enrollment.request_child_legacy_manual((b.archived).selected_days, (b.archived).manual_selected_days, (b.archived).automatic_selected_days)
				THEN (b.archived).manual_selected_days ELSE b.manual_selected_days
			END AS manual_selected_days,
			b.automatic_selected_days, b.valid_from, b.valid_until
		FROM (
			SELECT booking.*,
				(SELECT archived FROM enrollment.request_child_offerings_legacy archived
					WHERE archived.id = booking.id AND archived.tenant_id = booking.tenant_id) AS archived,
				(SELECT selection.notes FROM enrollment.request_child_offering_selections selection
					WHERE selection.tenant_id = booking.tenant_id
					AND selection.request_child_id = booking.request_child_id
					AND selection.care_offering_id = booking.care_offering_id) AS submission_notes
			FROM enrollment.care_offering_bookings booking
		) b
		WHERE (SELECT nextval('enrollment.request_child_compatibility_reads')) > 0;

		ALTER VIEW enrollment.request_child_offerings ALTER COLUMN id SET DEFAULT nextval('enrollment.care_offering_bookings_id_seq');
		ALTER VIEW enrollment.request_child_offerings ALTER COLUMN created_at SET DEFAULT NOW();
		ALTER VIEW enrollment.request_child_offerings ALTER COLUMN updated_at SET DEFAULT NOW();
		GRANT SELECT, INSERT, UPDATE, DELETE ON enrollment.request_child_offerings TO phoenix_tenant, phoenix_admin;
	`)
	if err != nil {
		return fmt.Errorf("install request child storage compatibility: %w", err)
	}
	return installRequestChildStorageRouting(ctx, tx)
}

func installRequestChildStorageRouting(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, `
		-- The archive preserves rollback-only notes and original JSON shapes.
		-- Effective interval constraints are enforced exclusively by bookings;
		-- an archived interval can overlap a later, target-owned replacement.


		CREATE OR REPLACE FUNCTION enrollment.route_request_child_offering_compatibility()
		RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
		DECLARE effective_manual jsonb;
		        effective_days jsonb;
		        requested_days jsonb;
		BEGIN
			PERFORM nextval('enrollment.request_child_compatibility_writes');
			IF TG_OP = 'DELETE' THEN
				PERFORM pg_advisory_xact_lock(hashtextextended('care-offering-bookings:' || OLD.tenant_id || ':' || OLD.request_child_id, 0));
				DELETE FROM enrollment.care_offering_bookings WHERE tenant_id = OLD.tenant_id AND id = OLD.id;
				IF NOT FOUND THEN RETURN NULL; END IF;
				DELETE FROM enrollment.request_child_offerings_legacy WHERE tenant_id = OLD.tenant_id AND id = OLD.id;
				RETURN OLD;
			END IF;
			IF TG_OP = 'UPDATE' AND (NEW.id, NEW.tenant_id, NEW.request_child_id, NEW.care_offering_id)
				IS DISTINCT FROM (OLD.id, OLD.tenant_id, OLD.request_child_id, OLD.care_offering_id) THEN
				RAISE EXCEPTION 'request child offering identity is immutable' USING ERRCODE = '23514';
			END IF;
			PERFORM pg_advisory_xact_lock(hashtextextended('care-offering-bookings:' || NEW.tenant_id || ':' || NEW.request_child_id, 0));
			effective_manual := enrollment.request_child_legacy_manual(NEW.selected_days, NEW.manual_selected_days, NEW.automatic_selected_days);
			effective_days := COALESCE(enrollment.request_child_effective_days(effective_manual, NEW.automatic_selected_days), '[]'::jsonb);
			requested_days := CASE WHEN jsonb_typeof(NEW.selected_days) = 'array' THEN NEW.selected_days ELSE '[]'::jsonb END;
			IF NOT (effective_days @> requested_days AND requested_days @> effective_days) THEN
				RAISE EXCEPTION 'request child offering effective days disagree with manual and automatic days' USING ERRCODE = '23514';
			END IF;
			IF TG_OP = 'INSERT' THEN
				INSERT INTO enrollment.care_offering_bookings
					(id, tenant_id, request_child_id, care_offering_id, manual_selected_days, automatic_selected_days, valid_from, valid_until, created_at, updated_at)
				VALUES (NEW.id, NEW.tenant_id, NEW.request_child_id, NEW.care_offering_id, effective_manual, NEW.automatic_selected_days, NEW.valid_from, NEW.valid_until, NEW.created_at, NEW.updated_at)
				ON CONFLICT (id) DO NOTHING;
				IF NOT FOUND THEN RETURN NULL; END IF;
				INSERT INTO enrollment.request_child_offering_selections
					(tenant_id, request_child_id, care_offering_id, selected_days, notes, created_at)
				VALUES (NEW.tenant_id, NEW.request_child_id, NEW.care_offering_id, effective_manual, NEW.notes, NEW.created_at)
				ON CONFLICT (tenant_id, request_child_id, care_offering_id) DO NOTHING;
			ELSE
				UPDATE enrollment.care_offering_bookings
				SET manual_selected_days = effective_manual, automatic_selected_days = NEW.automatic_selected_days,
					valid_from = NEW.valid_from, valid_until = NEW.valid_until, created_at = NEW.created_at, updated_at = NEW.updated_at
				WHERE tenant_id = OLD.tenant_id AND id = OLD.id;
				IF NOT FOUND THEN RETURN NULL; END IF;
			END IF;

			-- Only the rollback interface writes this metadata cache. Current
			-- application providers never read or write the legacy shape.
			INSERT INTO enrollment.request_child_offerings_legacy
				(id, tenant_id, request_child_id, care_offering_id, selected_days, notes, created_at, updated_at, manual_selected_days, automatic_selected_days, valid_from, valid_until)
			VALUES (NEW.id, NEW.tenant_id, NEW.request_child_id, NEW.care_offering_id, NEW.selected_days, NEW.notes, NEW.created_at, NEW.updated_at, NEW.manual_selected_days, NEW.automatic_selected_days, NEW.valid_from, NEW.valid_until)
			ON CONFLICT (id) DO UPDATE SET
				selected_days = EXCLUDED.selected_days, notes = EXCLUDED.notes,
				created_at = EXCLUDED.created_at, updated_at = EXCLUDED.updated_at,
				manual_selected_days = EXCLUDED.manual_selected_days, automatic_selected_days = EXCLUDED.automatic_selected_days,
				valid_from = EXCLUDED.valid_from, valid_until = EXCLUDED.valid_until;
			RETURN NEW;
		END
		$function$;
		CREATE OR REPLACE TRIGGER request_child_offering_compatibility_write
			INSTEAD OF INSERT OR UPDATE OR DELETE ON enrollment.request_child_offerings
			FOR EACH ROW EXECUTE FUNCTION enrollment.route_request_child_offering_compatibility();
	`)
	if err != nil {
		return fmt.Errorf("install request child storage compatibility routing: %w", err)
	}
	return nil
}
