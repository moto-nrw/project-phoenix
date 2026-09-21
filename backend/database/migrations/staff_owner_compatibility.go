package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

// installStaffOwnerCompatibility runs inside the final-delta lock and
// transaction. It moves every staff foreign key onto the School Membership
// row, archives the old base table and republishes its name as the
// compatibility view. The shape exists for previous-image rollback only: no
// current provider reads or writes it, and #2754 removes it after the rollback
// window.
func installStaffOwnerCompatibility(ctx context.Context, tx bun.Tx) error {
	if err := repointStaffForeignKeys(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		ALTER TABLE users.staff RENAME TO staff_legacy;
		COMMENT ON TABLE users.staff_legacy IS
			'Rollback-only archive of the pre-Cutover users.staff (#2753). users.staff_school_memberships and users.staff_employment_profiles are authoritative; nothing writes this table.';
		-- The frozen archive must not keep a work-time model alive: after the
		-- switch Workforce detaches a model on the profile and then deletes it.
		ALTER TABLE users.staff_legacy DROP CONSTRAINT fk_staff_work_time_model;
		-- The previous image classifies a duplicate staff member and a taken
		-- personnel number by these names, so the owner storage carries them.
		ALTER INDEX users.idx_staff_tenant_person RENAME TO idx_staff_legacy_tenant_person;
		ALTER INDEX users.uq_staff_tenant_personnel_number RENAME TO uq_staff_legacy_tenant_personnel_number;
		ALTER INDEX users.uq_staff_school_memberships_active_person RENAME TO idx_staff_tenant_person;

		-- The membership takes over the updated_at maintenance users.staff had.
		-- It is created after the final delta on purpose: the copy carries the
		-- source timestamps and a bumping trigger would have rewritten them out
		-- from under the checksum.
		CREATE TRIGGER update_staff_school_memberships_updated_at BEFORE UPDATE ON users.staff_school_memberships
			FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		CREATE SEQUENCE users.staff_compatibility_reads;
		CREATE SEQUENCE users.staff_compatibility_writes;
		COMMENT ON SEQUENCE users.staff_compatibility_reads IS
			'Queries served by the rollback-only users.staff view (#2753). Must trend to zero before #2754.';
		COMMENT ON SEQUENCE users.staff_compatibility_writes IS
			'Rows routed through the rollback-only users.staff view (#2753). Must trend to zero before #2754.';
		GRANT USAGE ON SEQUENCE users.staff_compatibility_reads, users.staff_compatibility_writes TO phoenix_tenant;
		GRANT USAGE, SELECT ON SEQUENCE users.staff_compatibility_reads, users.staff_compatibility_writes TO phoenix_admin;
	`); err != nil {
		return fmt.Errorf("archive users.staff: %w", err)
	}
	for _, statement := range []struct{ name, sql string }{
		{"personnel number uniqueness", staffPersonnelNumberUniqueness},
		{"compatibility view", staffOwnerCompatibilityView},
		{"compatibility routing", staffOwnerCompatibilityRouting},
		{"time-tracking audit view", staffOwnerAuditLogView},
	} {
		if _, err := tx.ExecContext(ctx, statement.sql); err != nil {
			return fmt.Errorf("install staff %s: %w", statement.name, err)
		}
	}
	return nil
}

// staffPersonnelNumberUniqueness keeps the rule users.staff enforced with a
// partial unique index: one live staff member per personnel number and school.
// "Live" is the membership's deleted_at and the number is the profile's, so no
// single-table index can express it any more. The advisory lock serialises two
// writers of the same number; each re-checks after acquiring it, when the
// other's commit is visible. The error carries the old index name, so both
// images classify the conflict exactly as before.
const staffPersonnelNumberUniqueness = `
	CREATE OR REPLACE FUNCTION users.enforce_staff_personnel_number()
	RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
	BEGIN
		IF NEW.personnel_number IS NULL THEN
			RETURN NEW;
		END IF;
		IF TG_OP = 'UPDATE' AND (NEW.personnel_number, NEW.tenant_id) IS NOT DISTINCT FROM (OLD.personnel_number, OLD.tenant_id) THEN
			RETURN NEW;
		END IF;
		PERFORM pg_advisory_xact_lock(hashtextextended('staff-personnel-number:' || NEW.tenant_id || ':' || NEW.personnel_number, 0));
		IF EXISTS (
			SELECT 1 FROM users.staff_school_memberships AS own
			WHERE own.tenant_id = NEW.tenant_id AND own.id = NEW.membership_id AND own.deleted_at IS NULL
		) AND EXISTS (
			SELECT 1 FROM users.staff_employment_profiles AS other
			JOIN users.staff_school_memberships AS membership
				ON membership.tenant_id = other.tenant_id AND membership.id = other.membership_id
			WHERE other.tenant_id = NEW.tenant_id AND other.personnel_number = NEW.personnel_number
			  AND other.membership_id <> NEW.membership_id AND membership.deleted_at IS NULL
		) THEN
			RAISE EXCEPTION 'duplicate key value violates unique constraint "uq_staff_tenant_personnel_number"'
				USING ERRCODE = '23505', CONSTRAINT = 'uq_staff_tenant_personnel_number',
					SCHEMA = 'users', TABLE = 'staff_employment_profiles';
		END IF;
		RETURN NEW;
	END
	$function$;
	CREATE OR REPLACE TRIGGER staff_employment_profiles_personnel_number
		BEFORE INSERT OR UPDATE OF personnel_number, tenant_id ON users.staff_employment_profiles
		FOR EACH ROW EXECUTE FUNCTION users.enforce_staff_personnel_number();`

// staffOwnerCompatibilityView republishes the old column set, in the old
// order, from the two owner tables. The join is inner: every membership owns
// exactly one employment profile, and a row-locking read (SELECT ... FOR
// UPDATE, which the old providers use) cannot be applied to the nullable side
// of an outer join. Unlike users.students, users.staff had soft deletion, so a
// retired membership stays visible with its deleted_at.
const staffOwnerCompatibilityView = `
	CREATE OR REPLACE VIEW users.staff WITH (security_invoker = true) AS
	SELECT m.id, m.person_id, p.staff_notes, m.created_at, m.updated_at, p.employment_type,
		m.tenant_id, p.work_time_model_id, p.rotation_anchor_date, m.deleted_at,
		p.personnel_number, p.birthday_display_opt_out
	FROM users.staff_school_memberships AS m
	JOIN users.staff_employment_profiles AS p ON p.tenant_id = m.tenant_id AND p.membership_id = m.id
	WHERE (SELECT nextval('users.staff_compatibility_reads')) > 0;

	ALTER VIEW users.staff ALTER COLUMN id SET DEFAULT nextval('users.staff_school_memberships_id_seq');
	ALTER VIEW users.staff ALTER COLUMN created_at SET DEFAULT NOW();
	ALTER VIEW users.staff ALTER COLUMN updated_at SET DEFAULT NOW();
	ALTER VIEW users.staff ALTER COLUMN birthday_display_opt_out SET DEFAULT false;
	GRANT SELECT, INSERT, UPDATE, DELETE ON users.staff TO phoenix_tenant, phoenix_admin;`

// staffOwnerCompatibilityRouting sends every write on the view to the owner
// that holds the column. Routed UPDATE locks membership, then profile — the
// order SELECT ... FOR UPDATE on the view takes — and applies only the columns
// the statement assigned onto the locked rows, so a concurrent owner write is
// not overwritten by the statement's unlocked snapshot. The membership's
// updated_at trigger moves the one timestamp the old shape exposed, as the
// users.staff trigger did for every update.
const staffOwnerCompatibilityRouting = `
	CREATE OR REPLACE FUNCTION users.route_staff_compatibility()
	RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
	DECLARE
		membership users.staff_school_memberships%ROWTYPE;
		profile users.staff_employment_profiles%ROWTYPE;
	BEGIN
		PERFORM nextval('users.staff_compatibility_writes');
		IF TG_OP = 'DELETE' THEN
			-- The profile follows through its foreign-key cascade.
			DELETE FROM users.staff_school_memberships WHERE tenant_id = OLD.tenant_id AND id = OLD.id;
			IF NOT FOUND THEN RETURN NULL; END IF;
			RETURN OLD;
		END IF;
		IF TG_OP = 'INSERT' THEN
			NEW.birthday_display_opt_out := COALESCE(NEW.birthday_display_opt_out, false);
			INSERT INTO users.staff_school_memberships (id, tenant_id, person_id, created_at, updated_at, deleted_at)
			VALUES (NEW.id, NEW.tenant_id, NEW.person_id, NEW.created_at, NEW.updated_at, NEW.deleted_at);
			INSERT INTO users.staff_employment_profiles
				(membership_id, tenant_id, staff_notes, employment_type, work_time_model_id,
				 personnel_number, rotation_anchor_date, birthday_display_opt_out)
			VALUES (NEW.id, NEW.tenant_id, NEW.staff_notes, NEW.employment_type, NEW.work_time_model_id,
				NEW.personnel_number, NEW.rotation_anchor_date, NEW.birthday_display_opt_out);
			RETURN NEW;
		END IF;
		IF (NEW.id, NEW.tenant_id) IS DISTINCT FROM (OLD.id, OLD.tenant_id) THEN
			RAISE EXCEPTION 'staff identity is immutable' USING ERRCODE = '23514';
		END IF;
		SELECT * INTO membership FROM users.staff_school_memberships AS m
		WHERE m.tenant_id = OLD.tenant_id AND m.id = OLD.id
		FOR UPDATE;
		IF NOT FOUND THEN RETURN NULL; END IF;
		SELECT * INTO profile FROM users.staff_employment_profiles AS p
		WHERE p.tenant_id = OLD.tenant_id AND p.membership_id = OLD.id
		FOR UPDATE;
		IF NOT FOUND THEN RETURN NULL; END IF;
		-- Heap EvalPlanQual would recheck the old writers' WHERE deleted_at IS
		-- NULL after waiting. INSTEAD OF has no WHERE, so a row whose lifecycle
		-- moved under the statement's snapshot counts as no row updated.
		IF membership.deleted_at IS DISTINCT FROM OLD.deleted_at THEN
			RETURN NULL;
		END IF;
		UPDATE users.staff_school_memberships SET
			person_id = CASE WHEN NEW.person_id IS DISTINCT FROM OLD.person_id THEN NEW.person_id ELSE membership.person_id END,
			created_at = CASE WHEN NEW.created_at IS DISTINCT FROM OLD.created_at THEN NEW.created_at ELSE membership.created_at END,
			deleted_at = CASE WHEN NEW.deleted_at IS DISTINCT FROM OLD.deleted_at THEN NEW.deleted_at ELSE membership.deleted_at END
		WHERE tenant_id = OLD.tenant_id AND id = OLD.id
		RETURNING * INTO membership;
		UPDATE users.staff_employment_profiles SET
			staff_notes = CASE WHEN NEW.staff_notes IS DISTINCT FROM OLD.staff_notes THEN NEW.staff_notes ELSE profile.staff_notes END,
			employment_type = CASE WHEN NEW.employment_type IS DISTINCT FROM OLD.employment_type THEN NEW.employment_type ELSE profile.employment_type END,
			work_time_model_id = CASE WHEN NEW.work_time_model_id IS DISTINCT FROM OLD.work_time_model_id THEN NEW.work_time_model_id ELSE profile.work_time_model_id END,
			personnel_number = CASE WHEN NEW.personnel_number IS DISTINCT FROM OLD.personnel_number THEN NEW.personnel_number ELSE profile.personnel_number END,
			rotation_anchor_date = CASE WHEN NEW.rotation_anchor_date IS DISTINCT FROM OLD.rotation_anchor_date THEN NEW.rotation_anchor_date ELSE profile.rotation_anchor_date END,
			birthday_display_opt_out = CASE WHEN NEW.birthday_display_opt_out IS DISTINCT FROM OLD.birthday_display_opt_out THEN COALESCE(NEW.birthday_display_opt_out, false) ELSE profile.birthday_display_opt_out END
		WHERE tenant_id = OLD.tenant_id AND membership_id = OLD.id
		RETURNING * INTO profile;
		NEW.person_id := membership.person_id;
		NEW.created_at := membership.created_at;
		NEW.updated_at := membership.updated_at;
		NEW.deleted_at := membership.deleted_at;
		NEW.staff_notes := profile.staff_notes;
		NEW.employment_type := profile.employment_type;
		NEW.work_time_model_id := profile.work_time_model_id;
		NEW.personnel_number := profile.personnel_number;
		NEW.rotation_anchor_date := profile.rotation_anchor_date;
		NEW.birthday_display_opt_out := profile.birthday_display_opt_out;
		RETURN NEW;
	END
	$function$;
	CREATE OR REPLACE TRIGGER staff_compatibility_write
		INSTEAD OF INSERT OR UPDATE OR DELETE ON users.staff
		FOR EACH ROW EXECUTE FUNCTION users.route_staff_compatibility();`

// staffOwnerAuditLogView is the 1.15.396 view with its one users.staff join
// (the actor of an absence decision) moved onto the membership. It would
// otherwise have followed the rename onto the frozen archive and stopped
// resolving staff members who joined after Cutover.
const staffOwnerAuditLogView = `
		CREATE OR REPLACE VIEW audit.time_tracking_audit_log WITH (security_invoker = true) AS
		SELECT e.tenant_id, e.created_at AS occurred_at, 'session_edit'::text AS source,
			MIN(e.id) AS entry_id, e.staff_id, ARRAY[e.staff_id] AS staff_ids,
			NULLIF(e.edited_by, 0) AS actor_staff_id, e.edited_by = 0 AS actor_is_system,
			COALESCE(MAX(NULLIF(e.notes, '')), '')::text AS reason,
			jsonb_build_object(
				'session_id', e.session_id, 'session_date', to_char(ws.date, 'YYYY-MM-DD'),
				'fields', jsonb_agg(jsonb_build_object(
					'field', e.field_name, 'old', e.old_value, 'new', e.new_value
				) ORDER BY e.id)
			) AS detail
		FROM audit.work_session_edits e
		JOIN active.work_sessions ws ON ws.id = e.session_id AND ws.tenant_id = e.tenant_id
		GROUP BY e.tenant_id, e.session_id, e.staff_id, e.edited_by, e.created_at, ws.date
		UNION ALL
		SELECT saa.tenant_id, saa.changed_at, 'absence'::text, saa.id, sa.staff_id,
			ARRAY[sa.staff_id], st.id, FALSE, COALESCE(saa.note, '')::text,
			jsonb_build_object(
				'absence_id', sa.id, 'absence_type', sa.absence_type,
				'absence_type_label', sat.name,
				'date_start', to_char(sa.date_start, 'YYYY-MM-DD'),
				'date_end', to_char(sa.date_end, 'YYYY-MM-DD'),
				'from_status', saa.from_status, 'to_status', saa.to_status,
				'from_absence_type', saa.from_absence_type,
				'to_absence_type', saa.to_absence_type,
				'from_absence_type_label', sat_from.name,
				'to_absence_type_label', sat_to.name
			)
		FROM active.staff_absence_audit saa
		JOIN active.staff_absences sa ON sa.id = saa.absence_id AND sa.tenant_id = saa.tenant_id
		LEFT JOIN active.staff_absence_types sat
			ON sat.tenant_id = sa.tenant_id AND sat.id = sa.absence_type_id
		LEFT JOIN active.staff_absence_types sat_from
			ON sat_from.tenant_id = saa.tenant_id AND sat_from.id = saa.from_absence_type_id
		LEFT JOIN active.staff_absence_types sat_to
			ON sat_to.tenant_id = saa.tenant_id AND sat_to.id = saa.to_absence_type_id
		LEFT JOIN users.persons p ON p.account_id = saa.actor_id
		LEFT JOIN users.staff_school_memberships st ON st.person_id = p.id
		UNION ALL
		SELECT ba.tenant_id, ba.decided_at, 'adjustment'::text, ba.id, ba.staff_id,
			ARRAY[ba.staff_id], ba.decided_by, FALSE, ba.note::text,
			jsonb_build_object(
				'adjustment_id', ba.id, 'type', ba.type,
				'minutes_delta', ba.minutes_delta,
				'effective_date', to_char(ba.effective_date, 'YYYY-MM-DD')
			)
		FROM active.staff_balance_adjustments ba
		UNION ALL
		SELECT s.tenant_id, s.closed_at, 'month_close'::text, MIN(s.id), NULL::bigint,
			array_agg(s.staff_id), s.closed_by, FALSE, s.close_reason::text,
			jsonb_build_object('year', s.year, 'month', s.month, 'account_count', COUNT(*))
		FROM active.staff_month_balance_snapshots s
		GROUP BY s.tenant_id, s.closed_at, s.closed_by, s.year, s.month, s.close_reason
		UNION ALL
		SELECT s.tenant_id, s.reopened_at, 'month_reopen'::text, s.id, s.staff_id,
			ARRAY[s.staff_id], s.reopened_by, FALSE, s.reopen_reason::text,
			jsonb_build_object(
				'year', s.year, 'month', s.month,
				'closing_balance_minutes', s.closing_balance_minutes
			)
		FROM active.staff_month_balance_snapshots s
		WHERE s.reopened_at IS NOT NULL
		UNION ALL
		SELECT d.tenant_id, d.occurred_at, 'deletion'::text, d.id, d.staff_id,
			ARRAY[d.staff_id], d.deleted_by, FALSE, d.note::text,
			jsonb_build_object(
				'deleted_source', d.source, 'source_id', d.source_id, 'payload', d.payload
			)
		FROM audit.time_tracking_deletions d
		UNION ALL
		SELECT vo.tenant_id, vo.decided_at, 'vacation_opening'::text, vo.id, vo.staff_id,
			ARRAY[vo.staff_id], vo.decided_by, FALSE, vo.note::text,
			jsonb_build_object(
				'opening_id', vo.id, 'year', vo.year,
				'effective_date', to_char(vo.effective_date, 'YYYY-MM-DD'),
				'taken_before_days', vo.taken_before_days,
				'entered_remaining_days', vo.entered_remaining_days
			)
		FROM active.staff_vacation_openings vo
		UNION ALL
		SELECT c.tenant_id, c.created_at, 'absence_type_allowance'::text, c.id, c.staff_id,
			ARRAY[c.staff_id], c.changed_by, FALSE, c.reason::text,
			jsonb_build_object(
				'absence_type_id', c.absence_type_id, 'absence_type_label', sat.name,
				'year', c.year, 'old_entitled_days', c.old_entitled_days,
				'new_entitled_days', c.new_entitled_days
			)
		FROM active.staff_absence_type_allowance_changes c
		JOIN active.staff_absence_types sat
			ON sat.tenant_id = c.tenant_id AND sat.id = c.absence_type_id
		UNION ALL
		SELECT c.tenant_id, c.occurred_at, 'personnel_number'::text, c.id, c.staff_id,
			ARRAY[c.staff_id], c.changed_by, FALSE, c.note::text,
			jsonb_build_object('old_value', c.old_value, 'new_value', c.new_value)
		FROM audit.personnel_number_changes c
		UNION ALL
		SELECT q.tenant_id, q.created_at, 'vacation_quota'::text, q.id, q.staff_id,
			ARRAY[q.staff_id], q.changed_by, FALSE, q.reason::text,
			jsonb_build_object(
				'year', q.year,
				'old_entitled_days', q.old_entitled_days, 'new_entitled_days', q.new_entitled_days,
				'old_carryover_days', q.old_carryover_days, 'new_carryover_days', q.new_carryover_days
			)
		FROM active.staff_vacation_quota_changes q;

		GRANT SELECT ON audit.time_tracking_audit_log TO phoenix_tenant;
`

// repointStaffForeignKeys moves every foreign key that names a staff member
// from the old base table onto users.staff_school_memberships. The split
// preserved staff identity (membership id = staff id), so each reference keeps
// the number it already holds; without this the constraints would follow the
// rename onto the archive and reject every dependent row of a staff member who
// joins after Cutover.
//
// The list is spelled out rather than derived from pg_constraint at run time:
// the architecture ratchet cannot classify dynamic migration SQL, and a static
// list is also what makes this diff reviewable. The guard below fails the
// migration when the database holds a staff foreign key this list does not,
// so a constraint added after this was written cannot be silently left behind.
//
// The constraints are added NOT VALID so the switch does not scan every
// dependent table while it holds the write lock. They are enforced for new and
// changed rows immediately; ValidateStaffOwnerForeignKeys confirms the
// existing ones afterwards, outside the lock.
func repointStaffForeignKeys(ctx context.Context, tx bun.Tx) error {
	if _, err := tx.ExecContext(ctx, `
			ALTER TABLE active.activity_sessions DROP CONSTRAINT fk_activity_sessions_started_by;
			ALTER TABLE active.activity_sessions ADD CONSTRAINT fk_activity_sessions_started_by
				FOREIGN KEY (tenant_id, started_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE SET NULL (started_by) NOT VALID;
			ALTER TABLE active.attendance DROP CONSTRAINT fk_attendance_checked_in_by_tenant;
			ALTER TABLE active.attendance ADD CONSTRAINT fk_attendance_checked_in_by_tenant
				FOREIGN KEY (tenant_id, checked_in_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE RESTRICT NOT VALID;
			ALTER TABLE active.attendance DROP CONSTRAINT fk_attendance_checked_out_by_tenant;
			ALTER TABLE active.attendance ADD CONSTRAINT fk_attendance_checked_out_by_tenant
				FOREIGN KEY (tenant_id, checked_out_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE RESTRICT NOT VALID;
			ALTER TABLE active.group_supervisors DROP CONSTRAINT fk_supervision_staff_tenant;
			ALTER TABLE active.group_supervisors ADD CONSTRAINT fk_supervision_staff_tenant
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE active.scheduled_checkouts DROP CONSTRAINT fk_scheduled_checkouts_cancelled_by_tenant;
			ALTER TABLE active.scheduled_checkouts ADD CONSTRAINT fk_scheduled_checkouts_cancelled_by_tenant
				FOREIGN KEY (tenant_id, cancelled_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE SET NULL NOT VALID;
			ALTER TABLE active.scheduled_checkouts DROP CONSTRAINT fk_scheduled_checkouts_scheduled_by_tenant;
			ALTER TABLE active.scheduled_checkouts ADD CONSTRAINT fk_scheduled_checkouts_scheduled_by_tenant
				FOREIGN KEY (tenant_id, scheduled_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE RESTRICT NOT VALID;
			ALTER TABLE active.staff_absence_type_allowance_changes DROP CONSTRAINT fk_sat_allowance_change_actor;
			ALTER TABLE active.staff_absence_type_allowance_changes ADD CONSTRAINT fk_sat_allowance_change_actor
				FOREIGN KEY (tenant_id, changed_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE RESTRICT NOT VALID;
			ALTER TABLE active.staff_absence_type_allowance_changes DROP CONSTRAINT fk_sat_allowance_change_staff;
			ALTER TABLE active.staff_absence_type_allowance_changes ADD CONSTRAINT fk_sat_allowance_change_staff
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE active.staff_absence_type_allowances DROP CONSTRAINT fk_sat_allowance_staff;
			ALTER TABLE active.staff_absence_type_allowances ADD CONSTRAINT fk_sat_allowance_staff
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE active.staff_absences DROP CONSTRAINT fk_staff_absences_approved_by_tenant;
			ALTER TABLE active.staff_absences ADD CONSTRAINT fk_staff_absences_approved_by_tenant
				FOREIGN KEY (tenant_id, approved_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE active.staff_absences DROP CONSTRAINT fk_staff_absences_created_by_tenant;
			ALTER TABLE active.staff_absences ADD CONSTRAINT fk_staff_absences_created_by_tenant
				FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE active.staff_absences DROP CONSTRAINT fk_staff_absences_staff_tenant;
			ALTER TABLE active.staff_absences ADD CONSTRAINT fk_staff_absences_staff_tenant
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE active.staff_absences DROP CONSTRAINT staff_absences_substitute_staff_id_fkey;
			ALTER TABLE active.staff_absences ADD CONSTRAINT staff_absences_substitute_staff_id_fkey
				FOREIGN KEY (substitute_staff_id) REFERENCES users.staff_school_memberships(id) ON DELETE SET NULL NOT VALID;
			ALTER TABLE active.staff_balance_adjustments DROP CONSTRAINT fk_sba_decided_by_tenant;
			ALTER TABLE active.staff_balance_adjustments ADD CONSTRAINT fk_sba_decided_by_tenant
				FOREIGN KEY (tenant_id, decided_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE RESTRICT NOT VALID;
			ALTER TABLE active.staff_balance_adjustments DROP CONSTRAINT fk_sba_staff_tenant;
			ALTER TABLE active.staff_balance_adjustments ADD CONSTRAINT fk_sba_staff_tenant
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE active.staff_month_balance_snapshots DROP CONSTRAINT fk_smbs_closed_by_tenant;
			ALTER TABLE active.staff_month_balance_snapshots ADD CONSTRAINT fk_smbs_closed_by_tenant
				FOREIGN KEY (tenant_id, closed_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE RESTRICT NOT VALID;
			ALTER TABLE active.staff_month_balance_snapshots DROP CONSTRAINT fk_smbs_reopened_by_tenant;
			ALTER TABLE active.staff_month_balance_snapshots ADD CONSTRAINT fk_smbs_reopened_by_tenant
				FOREIGN KEY (tenant_id, reopened_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE RESTRICT NOT VALID;
			ALTER TABLE active.staff_month_balance_snapshots DROP CONSTRAINT fk_smbs_staff_tenant;
			ALTER TABLE active.staff_month_balance_snapshots ADD CONSTRAINT fk_smbs_staff_tenant
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE active.staff_vacation_openings DROP CONSTRAINT fk_svo_decided_by_tenant;
			ALTER TABLE active.staff_vacation_openings ADD CONSTRAINT fk_svo_decided_by_tenant
				FOREIGN KEY (tenant_id, decided_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE RESTRICT NOT VALID;
			ALTER TABLE active.staff_vacation_openings DROP CONSTRAINT fk_svo_staff_tenant;
			ALTER TABLE active.staff_vacation_openings ADD CONSTRAINT fk_svo_staff_tenant
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE active.staff_vacation_quota DROP CONSTRAINT staff_vacation_quota_staff_id_fkey;
			ALTER TABLE active.staff_vacation_quota ADD CONSTRAINT staff_vacation_quota_staff_id_fkey
				FOREIGN KEY (staff_id) REFERENCES users.staff_school_memberships(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE active.staff_vacation_quota_changes DROP CONSTRAINT fk_svq_change_actor;
			ALTER TABLE active.staff_vacation_quota_changes ADD CONSTRAINT fk_svq_change_actor
				FOREIGN KEY (tenant_id, changed_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE RESTRICT NOT VALID;
			ALTER TABLE active.staff_vacation_quota_changes DROP CONSTRAINT fk_svq_change_staff;
			ALTER TABLE active.staff_vacation_quota_changes ADD CONSTRAINT fk_svq_change_staff
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE active.work_sessions DROP CONSTRAINT fk_work_sessions_created_by_tenant;
			ALTER TABLE active.work_sessions ADD CONSTRAINT fk_work_sessions_created_by_tenant
				FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE active.work_sessions DROP CONSTRAINT fk_work_sessions_staff_tenant;
			ALTER TABLE active.work_sessions ADD CONSTRAINT fk_work_sessions_staff_tenant
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE active.work_sessions DROP CONSTRAINT fk_work_sessions_updated_by_tenant;
			ALTER TABLE active.work_sessions ADD CONSTRAINT fk_work_sessions_updated_by_tenant
				FOREIGN KEY (tenant_id, updated_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE activities.groups DROP CONSTRAINT fk_activity_groups_created_by_tenant;
			ALTER TABLE activities.groups ADD CONSTRAINT fk_activity_groups_created_by_tenant
				FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE activities.supervisors DROP CONSTRAINT fk_activity_supervisors_staff_tenant;
			ALTER TABLE activities.supervisors ADD CONSTRAINT fk_activity_supervisors_staff_tenant
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE audit.data_deletions DROP CONSTRAINT fk_data_deletions_staff;
			ALTER TABLE audit.data_deletions ADD CONSTRAINT fk_data_deletions_staff
				FOREIGN KEY (staff_id) REFERENCES users.staff_school_memberships(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE audit.deviation_events DROP CONSTRAINT deviation_events_related_staff_id_fkey;
			ALTER TABLE audit.deviation_events ADD CONSTRAINT deviation_events_related_staff_id_fkey
				FOREIGN KEY (related_staff_id) REFERENCES users.staff_school_memberships(id) ON DELETE SET NULL NOT VALID;
			ALTER TABLE audit.deviation_events DROP CONSTRAINT deviation_events_subject_staff_id_fkey;
			ALTER TABLE audit.deviation_events ADD CONSTRAINT deviation_events_subject_staff_id_fkey
				FOREIGN KEY (subject_staff_id) REFERENCES users.staff_school_memberships(id) ON DELETE SET NULL NOT VALID;
			ALTER TABLE calendar.appointment_recipients DROP CONSTRAINT appointment_recipients_staff_id_fkey;
			ALTER TABLE calendar.appointment_recipients ADD CONSTRAINT appointment_recipients_staff_id_fkey
				FOREIGN KEY (staff_id) REFERENCES users.staff_school_memberships(id) NOT VALID;
			ALTER TABLE calendar.appointments DROP CONSTRAINT appointments_organizer_staff_id_fkey;
			ALTER TABLE calendar.appointments ADD CONSTRAINT appointments_organizer_staff_id_fkey
				FOREIGN KEY (organizer_staff_id) REFERENCES users.staff_school_memberships(id) NOT VALID;
			ALTER TABLE calendar.staff_feed_tombstones DROP CONSTRAINT staff_feed_tombstones_staff_id_fkey;
			ALTER TABLE calendar.staff_feed_tombstones ADD CONSTRAINT staff_feed_tombstones_staff_id_fkey
				FOREIGN KEY (staff_id) REFERENCES users.staff_school_memberships(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE config.staff_work_schedules DROP CONSTRAINT staff_work_schedules_staff_id_fkey;
			ALTER TABLE config.staff_work_schedules ADD CONSTRAINT staff_work_schedules_staff_id_fkey
				FOREIGN KEY (staff_id) REFERENCES users.staff_school_memberships(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE education.class_teachers DROP CONSTRAINT fk_class_teachers_staff;
			ALTER TABLE education.class_teachers ADD CONSTRAINT fk_class_teachers_staff
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE education.grade_transition_class_teachers DROP CONSTRAINT fk_gtct_staff;
			ALTER TABLE education.grade_transition_class_teachers ADD CONSTRAINT fk_gtct_staff
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE education.group_substitution DROP CONSTRAINT fk_group_sub_regular_staff_tenant;
			ALTER TABLE education.group_substitution ADD CONSTRAINT fk_group_sub_regular_staff_tenant
				FOREIGN KEY (tenant_id, regular_staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE education.group_substitution DROP CONSTRAINT fk_group_sub_substitute_staff_tenant;
			ALTER TABLE education.group_substitution ADD CONSTRAINT fk_group_sub_substitute_staff_tenant
				FOREIGN KEY (tenant_id, substitute_staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.activity_exceptions DROP CONSTRAINT fk_activity_exceptions_created_by_tenant;
			ALTER TABLE schedule.activity_exceptions ADD CONSTRAINT fk_activity_exceptions_created_by_tenant
				FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE SET NULL (created_by) NOT VALID;
			ALTER TABLE schedule.activity_instances DROP CONSTRAINT fk_activity_instances_created_by_tenant;
			ALTER TABLE schedule.activity_instances ADD CONSTRAINT fk_activity_instances_created_by_tenant
				FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE SET NULL (created_by) NOT VALID;
			ALTER TABLE schedule.activity_instances DROP CONSTRAINT fk_activity_instances_started_by_tenant;
			ALTER TABLE schedule.activity_instances ADD CONSTRAINT fk_activity_instances_started_by_tenant
				FOREIGN KEY (tenant_id, started_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE SET NULL (started_by) NOT VALID;
			ALTER TABLE schedule.instance_staff DROP CONSTRAINT fk_instance_staff_staff_tenant;
			ALTER TABLE schedule.instance_staff ADD CONSTRAINT fk_instance_staff_staff_tenant
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE RESTRICT NOT VALID;
			ALTER TABLE schedule.staff_shift_series DROP CONSTRAINT fk_staff_shift_series_created_by;
			ALTER TABLE schedule.staff_shift_series ADD CONSTRAINT fk_staff_shift_series_created_by
				FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE schedule.staff_shift_series DROP CONSTRAINT fk_staff_shift_series_staff;
			ALTER TABLE schedule.staff_shift_series ADD CONSTRAINT fk_staff_shift_series_staff
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.staff_shift_series DROP CONSTRAINT fk_staff_shift_series_updated_by;
			ALTER TABLE schedule.staff_shift_series ADD CONSTRAINT fk_staff_shift_series_updated_by
				FOREIGN KEY (tenant_id, updated_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE schedule.staff_shift_series_exceptions DROP CONSTRAINT fk_staff_shift_series_exceptions_created_by;
			ALTER TABLE schedule.staff_shift_series_exceptions ADD CONSTRAINT fk_staff_shift_series_exceptions_created_by
				FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE schedule.staff_shifts DROP CONSTRAINT fk_staff_shifts_created_by;
			ALTER TABLE schedule.staff_shifts ADD CONSTRAINT fk_staff_shifts_created_by
				FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE schedule.staff_shifts DROP CONSTRAINT fk_staff_shifts_staff;
			ALTER TABLE schedule.staff_shifts ADD CONSTRAINT fk_staff_shifts_staff
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.staff_shifts DROP CONSTRAINT fk_staff_shifts_updated_by;
			ALTER TABLE schedule.staff_shifts ADD CONSTRAINT fk_staff_shifts_updated_by
				FOREIGN KEY (tenant_id, updated_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE schedule.student_arrival_exceptions DROP CONSTRAINT student_arrival_exceptions_created_by_fkey;
			ALTER TABLE schedule.student_arrival_exceptions ADD CONSTRAINT student_arrival_exceptions_created_by_fkey
				FOREIGN KEY (created_by) REFERENCES users.staff_school_memberships(id) NOT VALID;
			ALTER TABLE schedule.student_arrival_notes DROP CONSTRAINT student_arrival_notes_created_by_fkey;
			ALTER TABLE schedule.student_arrival_notes ADD CONSTRAINT student_arrival_notes_created_by_fkey
				FOREIGN KEY (created_by) REFERENCES users.staff_school_memberships(id) NOT VALID;
			ALTER TABLE schedule.student_arrival_schedules DROP CONSTRAINT student_arrival_schedules_created_by_fkey;
			ALTER TABLE schedule.student_arrival_schedules ADD CONSTRAINT student_arrival_schedules_created_by_fkey
				FOREIGN KEY (created_by) REFERENCES users.staff_school_memberships(id) NOT VALID;
			ALTER TABLE schedule.student_pickup_exceptions DROP CONSTRAINT fk_pickup_exception_excused_created_by_tenant;
			ALTER TABLE schedule.student_pickup_exceptions ADD CONSTRAINT fk_pickup_exception_excused_created_by_tenant
				FOREIGN KEY (tenant_id, excused_created_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE schedule.student_pickup_exceptions DROP CONSTRAINT fk_pickup_exceptions_created_by_tenant;
			ALTER TABLE schedule.student_pickup_exceptions ADD CONSTRAINT fk_pickup_exceptions_created_by_tenant
				FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE schedule.student_pickup_notes DROP CONSTRAINT fk_pickup_notes_created_by_tenant;
			ALTER TABLE schedule.student_pickup_notes ADD CONSTRAINT fk_pickup_notes_created_by_tenant
				FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE schedule.student_pickup_schedules DROP CONSTRAINT fk_pickup_schedules_created_by_tenant;
			ALTER TABLE schedule.student_pickup_schedules ADD CONSTRAINT fk_pickup_schedules_created_by_tenant
				FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff_school_memberships(tenant_id, id) NOT VALID;
			ALTER TABLE users.guests DROP CONSTRAINT fk_guests_staff_tenant;
			ALTER TABLE users.guests ADD CONSTRAINT fk_guests_staff_tenant
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.staff_documents DROP CONSTRAINT fk_staff_documents_staff;
			ALTER TABLE users.staff_documents ADD CONSTRAINT fk_staff_documents_staff
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.staff_financial_data DROP CONSTRAINT fk_staff_financial_data_staff;
			ALTER TABLE users.staff_financial_data ADD CONSTRAINT fk_staff_financial_data_staff
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.staff_master_data DROP CONSTRAINT fk_staff_master_data_staff;
			ALTER TABLE users.staff_master_data ADD CONSTRAINT fk_staff_master_data_staff
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.staff_qualifications DROP CONSTRAINT fk_staff_qualifications_staff;
			ALTER TABLE users.staff_qualifications ADD CONSTRAINT fk_staff_qualifications_staff
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.teachers DROP CONSTRAINT fk_teachers_staff_tenant;
			ALTER TABLE users.teachers ADD CONSTRAINT fk_teachers_staff_tenant
				FOREIGN KEY (tenant_id, staff_id) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE NOT VALID;
			DO $$ BEGIN
				IF EXISTS (SELECT 1 FROM pg_constraint
					WHERE confrelid = 'users.staff'::regclass AND contype = 'f') THEN
					RAISE EXCEPTION 'a staff foreign key was added after this migration was written; repoint it here before Cutover';
				END IF;
			END $$;
	`); err != nil {
		return fmt.Errorf("repoint staff foreign keys: %w", err)
	}
	return nil
}

// ValidateStaffOwnerForeignKeys confirms the repointed constraints against the
// rows that already exist. It runs after the switch transaction: VALIDATE
// takes a share-update-exclusive lock on the dependent table and a row-share
// lock on the memberships, so ordinary reads and writes continue while it
// scans. Validating a constraint that is already valid is a no-op, so an
// interrupted run is simply repeated by the next migrate.
func ValidateStaffOwnerForeignKeys(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("staff owner cutover: database is required")
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE active.activity_sessions VALIDATE CONSTRAINT fk_activity_sessions_started_by;
		ALTER TABLE active.attendance VALIDATE CONSTRAINT fk_attendance_checked_in_by_tenant;
		ALTER TABLE active.attendance VALIDATE CONSTRAINT fk_attendance_checked_out_by_tenant;
		ALTER TABLE active.group_supervisors VALIDATE CONSTRAINT fk_supervision_staff_tenant;
		ALTER TABLE active.scheduled_checkouts VALIDATE CONSTRAINT fk_scheduled_checkouts_cancelled_by_tenant;
		ALTER TABLE active.scheduled_checkouts VALIDATE CONSTRAINT fk_scheduled_checkouts_scheduled_by_tenant;
		ALTER TABLE active.staff_absence_type_allowance_changes VALIDATE CONSTRAINT fk_sat_allowance_change_actor;
		ALTER TABLE active.staff_absence_type_allowance_changes VALIDATE CONSTRAINT fk_sat_allowance_change_staff;
		ALTER TABLE active.staff_absence_type_allowances VALIDATE CONSTRAINT fk_sat_allowance_staff;
		ALTER TABLE active.staff_absences VALIDATE CONSTRAINT fk_staff_absences_approved_by_tenant;
		ALTER TABLE active.staff_absences VALIDATE CONSTRAINT fk_staff_absences_created_by_tenant;
		ALTER TABLE active.staff_absences VALIDATE CONSTRAINT fk_staff_absences_staff_tenant;
		ALTER TABLE active.staff_absences VALIDATE CONSTRAINT staff_absences_substitute_staff_id_fkey;
		ALTER TABLE active.staff_balance_adjustments VALIDATE CONSTRAINT fk_sba_decided_by_tenant;
		ALTER TABLE active.staff_balance_adjustments VALIDATE CONSTRAINT fk_sba_staff_tenant;
		ALTER TABLE active.staff_month_balance_snapshots VALIDATE CONSTRAINT fk_smbs_closed_by_tenant;
		ALTER TABLE active.staff_month_balance_snapshots VALIDATE CONSTRAINT fk_smbs_reopened_by_tenant;
		ALTER TABLE active.staff_month_balance_snapshots VALIDATE CONSTRAINT fk_smbs_staff_tenant;
		ALTER TABLE active.staff_vacation_openings VALIDATE CONSTRAINT fk_svo_decided_by_tenant;
		ALTER TABLE active.staff_vacation_openings VALIDATE CONSTRAINT fk_svo_staff_tenant;
		ALTER TABLE active.staff_vacation_quota VALIDATE CONSTRAINT staff_vacation_quota_staff_id_fkey;
		ALTER TABLE active.staff_vacation_quota_changes VALIDATE CONSTRAINT fk_svq_change_actor;
		ALTER TABLE active.staff_vacation_quota_changes VALIDATE CONSTRAINT fk_svq_change_staff;
		ALTER TABLE active.work_sessions VALIDATE CONSTRAINT fk_work_sessions_created_by_tenant;
		ALTER TABLE active.work_sessions VALIDATE CONSTRAINT fk_work_sessions_staff_tenant;
		ALTER TABLE active.work_sessions VALIDATE CONSTRAINT fk_work_sessions_updated_by_tenant;
		ALTER TABLE activities.groups VALIDATE CONSTRAINT fk_activity_groups_created_by_tenant;
		ALTER TABLE activities.supervisors VALIDATE CONSTRAINT fk_activity_supervisors_staff_tenant;
		ALTER TABLE audit.data_deletions VALIDATE CONSTRAINT fk_data_deletions_staff;
		ALTER TABLE audit.deviation_events VALIDATE CONSTRAINT deviation_events_related_staff_id_fkey;
		ALTER TABLE audit.deviation_events VALIDATE CONSTRAINT deviation_events_subject_staff_id_fkey;
		ALTER TABLE calendar.appointment_recipients VALIDATE CONSTRAINT appointment_recipients_staff_id_fkey;
		ALTER TABLE calendar.appointments VALIDATE CONSTRAINT appointments_organizer_staff_id_fkey;
		ALTER TABLE calendar.staff_feed_tombstones VALIDATE CONSTRAINT staff_feed_tombstones_staff_id_fkey;
		ALTER TABLE config.staff_work_schedules VALIDATE CONSTRAINT staff_work_schedules_staff_id_fkey;
		ALTER TABLE education.class_teachers VALIDATE CONSTRAINT fk_class_teachers_staff;
		ALTER TABLE education.grade_transition_class_teachers VALIDATE CONSTRAINT fk_gtct_staff;
		ALTER TABLE education.group_substitution VALIDATE CONSTRAINT fk_group_sub_regular_staff_tenant;
		ALTER TABLE education.group_substitution VALIDATE CONSTRAINT fk_group_sub_substitute_staff_tenant;
		ALTER TABLE schedule.activity_exceptions VALIDATE CONSTRAINT fk_activity_exceptions_created_by_tenant;
		ALTER TABLE schedule.activity_instances VALIDATE CONSTRAINT fk_activity_instances_created_by_tenant;
		ALTER TABLE schedule.activity_instances VALIDATE CONSTRAINT fk_activity_instances_started_by_tenant;
		ALTER TABLE schedule.instance_staff VALIDATE CONSTRAINT fk_instance_staff_staff_tenant;
		ALTER TABLE schedule.staff_shift_series VALIDATE CONSTRAINT fk_staff_shift_series_created_by;
		ALTER TABLE schedule.staff_shift_series VALIDATE CONSTRAINT fk_staff_shift_series_staff;
		ALTER TABLE schedule.staff_shift_series VALIDATE CONSTRAINT fk_staff_shift_series_updated_by;
		ALTER TABLE schedule.staff_shift_series_exceptions VALIDATE CONSTRAINT fk_staff_shift_series_exceptions_created_by;
		ALTER TABLE schedule.staff_shifts VALIDATE CONSTRAINT fk_staff_shifts_created_by;
		ALTER TABLE schedule.staff_shifts VALIDATE CONSTRAINT fk_staff_shifts_staff;
		ALTER TABLE schedule.staff_shifts VALIDATE CONSTRAINT fk_staff_shifts_updated_by;
		ALTER TABLE schedule.student_arrival_exceptions VALIDATE CONSTRAINT student_arrival_exceptions_created_by_fkey;
		ALTER TABLE schedule.student_arrival_notes VALIDATE CONSTRAINT student_arrival_notes_created_by_fkey;
		ALTER TABLE schedule.student_arrival_schedules VALIDATE CONSTRAINT student_arrival_schedules_created_by_fkey;
		ALTER TABLE schedule.student_pickup_exceptions VALIDATE CONSTRAINT fk_pickup_exception_excused_created_by_tenant;
		ALTER TABLE schedule.student_pickup_exceptions VALIDATE CONSTRAINT fk_pickup_exceptions_created_by_tenant;
		ALTER TABLE schedule.student_pickup_notes VALIDATE CONSTRAINT fk_pickup_notes_created_by_tenant;
		ALTER TABLE schedule.student_pickup_schedules VALIDATE CONSTRAINT fk_pickup_schedules_created_by_tenant;
		ALTER TABLE users.guests VALIDATE CONSTRAINT fk_guests_staff_tenant;
		ALTER TABLE users.staff_documents VALIDATE CONSTRAINT fk_staff_documents_staff;
		ALTER TABLE users.staff_financial_data VALIDATE CONSTRAINT fk_staff_financial_data_staff;
		ALTER TABLE users.staff_master_data VALIDATE CONSTRAINT fk_staff_master_data_staff;
		ALTER TABLE users.staff_qualifications VALIDATE CONSTRAINT fk_staff_qualifications_staff;
		ALTER TABLE users.teachers VALIDATE CONSTRAINT fk_teachers_staff_tenant;
	`); err != nil {
		return fmt.Errorf("staff owner cutover: validate repointed foreign keys: %w", err)
	}
	return nil
}
