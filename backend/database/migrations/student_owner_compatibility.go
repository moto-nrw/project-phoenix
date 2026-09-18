package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

// installStudentOwnerCompatibility runs inside the final-delta lock and
// transaction. It moves every student foreign key onto the People profile,
// archives the old base table and republishes its name as the compatibility
// view. The shape exists for previous-image rollback only: no current provider
// reads or writes it, and #2760 removes it after the rollback window.
func installStudentOwnerCompatibility(ctx context.Context, tx bun.Tx) error {
	if err := repointStudentForeignKeys(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		ALTER TABLE users.students RENAME TO students_legacy;
		COMMENT ON TABLE users.students_legacy IS
			'Rollback-only archive of the pre-Cutover users.students (#2759). The three owner tables are authoritative; only the compatibility view writes this table.';

		-- The targets take over the updated_at maintenance users.students had,
		-- so a write that leaves the column alone still bumps it exactly as
		-- before. They are created after the final delta on purpose: the copy
		-- carries the source timestamps and a bumping trigger would have
		-- rewritten them out from under the checksum.
		CREATE TRIGGER update_student_profiles_updated_at BEFORE UPDATE ON users.student_profiles
			FOR EACH ROW EXECUTE FUNCTION update_modified_column();
		CREATE TRIGGER update_student_school_memberships_updated_at BEFORE UPDATE ON users.student_school_memberships
			FOR EACH ROW EXECUTE FUNCTION update_modified_column();
		CREATE TRIGGER update_student_care_profiles_updated_at BEFORE UPDATE ON users.student_care_profiles
			FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		CREATE SEQUENCE users.student_compatibility_reads;
		CREATE SEQUENCE users.student_compatibility_writes;
		COMMENT ON SEQUENCE users.student_compatibility_reads IS
			'Queries served by the rollback-only users.students view (#2759). Must trend to zero before #2760.';
		COMMENT ON SEQUENCE users.student_compatibility_writes IS
			'Rows routed through the rollback-only users.students view (#2759). Must trend to zero before #2760.';
		GRANT USAGE ON SEQUENCE users.student_compatibility_reads, users.student_compatibility_writes TO phoenix_tenant;
		GRANT USAGE, SELECT ON SEQUENCE users.student_compatibility_reads, users.student_compatibility_writes TO phoenix_admin;
	`); err != nil {
		return fmt.Errorf("archive users.students: %w", err)
	}
	return refreshStudentOwnerCompatibility(ctx, tx)
}

// refreshStudentOwnerCompatibility redefines the view, its routing and the one
// dependent view without touching data or the hit counters, so a repair can
// restore the rollback shape from the authoritative targets.
func refreshStudentOwnerCompatibility(ctx context.Context, tx bun.Tx) error {
	if _, err := tx.ExecContext(ctx, studentOwnerCompatibilityView); err != nil {
		return fmt.Errorf("install student compatibility view: %w", err)
	}
	if _, err := tx.ExecContext(ctx, studentOwnerCompatibilityRouting); err != nil {
		return fmt.Errorf("install student compatibility routing: %w", err)
	}
	return nil
}

// studentOwnerCompatibilityView republishes the old column set from the three
// owner tables.
//
// The joins are inner on purpose: every profile owns exactly one live
// membership and every membership one care profile, and a row-locking read
// (SELECT ... FOR UPDATE, which the old providers use) cannot be applied to the
// nullable side of an outer join. The legacy columns without a target are read
// from the archive through a scalar subquery for the same reason.
//
// A soft-deleted membership leaves the view: users.students had no soft
// deletion, so a retired enrollment must read as a row that is gone rather than
// as a child who is still enrolled.
//
// updated_at is the newest of the three, so a write to any one owner still
// moves the single timestamp the old shape exposed.
const studentOwnerCompatibilityView = `
	CREATE OR REPLACE VIEW users.students WITH (security_invoker = true) AS
	SELECT b.id, b.person_id, b.school_class,
		(b.archived).guardian_name, (b.archived).guardian_contact,
		(b.archived).guardian_email, (b.archived).guardian_phone,
		b.group_id, b.extra_info, b.created_at, b.updated_at,
		b.supervisor_notes, b.health_info, b.pickup_status,
		COALESCE((b.archived).sick, false) AS sick, (b.archived).sick_since,
		b.tenant_id,
		COALESCE((b.archived).excused, false) AS excused, (b.archived).excused_since,
		b.photo_path, b.photo_consent_given_at, b.photo_consent_given_by,
		b.status, b.enrolled_from, b.enrolled_until,
		b.agb_accepted_at, b.data_processing_accepted_at, b.email_contact_accepted_at,
		b.bus_days, b.pickup_days, b.departure_days, b.allowed_departure_modes,
		b.departure_companion_note, b.address_street, b.address_city, b.address_postal_code
	FROM (
		SELECT p.id, p.tenant_id, p.person_id, p.address_street, p.address_city,
			p.address_postal_code, p.extra_info, p.photo_path, p.photo_consent_given_at,
			p.photo_consent_given_by, p.agb_accepted_at, p.data_processing_accepted_at,
			p.email_contact_accepted_at, p.created_at,
			GREATEST(p.updated_at, m.updated_at, c.updated_at) AS updated_at,
			m.school_class, m.group_id, m.status, m.enrolled_from, m.enrolled_until,
			c.supervisor_notes, c.health_info, c.pickup_status, c.departure_days,
			c.allowed_departure_modes, c.departure_companion_note, c.pickup_days, c.bus_days,
			(SELECT archived FROM users.students_legacy archived
				WHERE archived.tenant_id = p.tenant_id AND archived.id = p.id) AS archived
		FROM users.student_profiles AS p
		JOIN users.student_school_memberships AS m
			ON m.tenant_id = p.tenant_id AND m.student_profile_id = p.id AND m.deleted_at IS NULL
		JOIN users.student_care_profiles AS c
			ON c.tenant_id = m.tenant_id AND c.membership_id = m.id
	) b
	WHERE (SELECT nextval('users.student_compatibility_reads')) > 0;

	ALTER VIEW users.students ALTER COLUMN id SET DEFAULT nextval('users.student_profiles_id_seq');
	ALTER VIEW users.students ALTER COLUMN created_at SET DEFAULT NOW();
	ALTER VIEW users.students ALTER COLUMN updated_at SET DEFAULT NOW();
	ALTER VIEW users.students ALTER COLUMN status SET DEFAULT 'active';
	ALTER VIEW users.students ALTER COLUMN sick SET DEFAULT false;
	ALTER VIEW users.students ALTER COLUMN excused SET DEFAULT false;
	ALTER VIEW users.students ALTER COLUMN bus_days SET DEFAULT '{}'::jsonb;
	ALTER VIEW users.students ALTER COLUMN pickup_days SET DEFAULT '{}'::jsonb;
	ALTER VIEW users.students ALTER COLUMN departure_days SET DEFAULT '{}'::jsonb;
	ALTER VIEW users.students ALTER COLUMN allowed_departure_modes SET DEFAULT '{}'::jsonb;
	GRANT SELECT, INSERT, UPDATE, DELETE ON users.students TO phoenix_tenant, phoenix_admin;

	-- The one database view over users.students would have followed the rename
	-- onto the archive and stopped seeing children enrolled after Cutover.
	-- It keeps its shape and reads the authoritative storage instead.
	CREATE OR REPLACE VIEW users.expired_privacy_consents WITH (security_invoker = true) AS
	SELECT pc.id, pc.student_id, pc.policy_version, pc.accepted, pc.accepted_at,
		pc.expires_at, pc.duration_days, pc.renewal_required, pc.data_retention_days,
		pc.details, pc.created_at, pc.updated_at, pc.tenant_id, p.person_id,
		(SELECT archived.guardian_name FROM users.students_legacy archived
			WHERE archived.tenant_id = p.tenant_id AND archived.id = p.id) AS guardian_name,
		(SELECT archived.guardian_email FROM users.students_legacy archived
			WHERE archived.tenant_id = p.tenant_id AND archived.id = p.id) AS guardian_email,
		(SELECT archived.guardian_phone FROM users.students_legacy archived
			WHERE archived.tenant_id = p.tenant_id AND archived.id = p.id) AS guardian_phone
	FROM users.privacy_consents pc
	JOIN users.student_profiles p ON pc.student_id = p.id
	WHERE pc.expires_at < CURRENT_TIMESTAMP AND pc.accepted = true AND pc.renewal_required = true;`

// studentOwnerCompatibilityRouting sends every write on the view to the owner
// that holds the column, and mirrors the whole old row into the archive so a
// previous image finds its legacy guardian and absence columns unchanged.
// Routed UPDATE locks profile, live membership and care in that join order,
// then applies only assigned columns onto the locked rows.
//
// Membership identity is not preserved for rows created here: nothing
// references a membership id, the profile id is what every foreign key names,
// and letting the membership sequence allocate keeps the two independent.
const studentOwnerCompatibilityRouting = `
	CREATE OR REPLACE FUNCTION users.route_student_compatibility()
	RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
	DECLARE
		owning_membership bigint;
		current_status text;
	BEGIN
		PERFORM nextval('users.student_compatibility_writes');
		IF TG_OP = 'DELETE' THEN
			DELETE FROM users.student_profiles
			WHERE tenant_id = OLD.tenant_id AND id = OLD.id;
			IF NOT FOUND THEN RETURN NULL; END IF;
			DELETE FROM users.students_legacy WHERE tenant_id = OLD.tenant_id AND id = OLD.id;
			RETURN OLD;
		END IF;
		IF TG_OP = 'UPDATE' AND (NEW.id, NEW.tenant_id) IS DISTINCT FROM (OLD.id, OLD.tenant_id) THEN
			RAISE EXCEPTION 'student identity is immutable' USING ERRCODE = '23514';
		END IF;
		IF TG_OP = 'INSERT' THEN
			INSERT INTO users.student_profiles
				(id, tenant_id, person_id, address_street, address_city, address_postal_code,
				 extra_info, photo_path, photo_consent_given_at, photo_consent_given_by,
				 agb_accepted_at, data_processing_accepted_at, email_contact_accepted_at,
				 created_at, updated_at)
			VALUES (NEW.id, NEW.tenant_id, NEW.person_id, NEW.address_street, NEW.address_city,
				NEW.address_postal_code, NEW.extra_info, NEW.photo_path, NEW.photo_consent_given_at,
				NEW.photo_consent_given_by, NEW.agb_accepted_at, NEW.data_processing_accepted_at,
				NEW.email_contact_accepted_at, NEW.created_at, NEW.updated_at);
			INSERT INTO users.student_school_memberships
				(tenant_id, student_profile_id, school_class, group_id, status,
				 enrolled_from, enrolled_until, created_at, updated_at)
			VALUES (NEW.tenant_id, NEW.id, NEW.school_class, NEW.group_id,
				COALESCE(NEW.status, 'active'), NEW.enrolled_from, NEW.enrolled_until,
				NEW.created_at, NEW.updated_at)
			RETURNING id INTO owning_membership;
			INSERT INTO users.student_care_profiles
				(membership_id, tenant_id, supervisor_notes, health_info, pickup_status,
				 departure_days, allowed_departure_modes, departure_companion_note,
				 pickup_days, bus_days, created_at, updated_at)
			VALUES (owning_membership, NEW.tenant_id, NEW.supervisor_notes, NEW.health_info,
				NEW.pickup_status, COALESCE(NEW.departure_days, '{}'::jsonb),
				COALESCE(NEW.allowed_departure_modes, '{}'::jsonb), NEW.departure_companion_note,
				COALESCE(NEW.pickup_days, '{}'::jsonb), COALESCE(NEW.bus_days, '{}'::jsonb),
				NEW.created_at, NEW.updated_at);
		ELSE
			-- Lock the three owners in the view's join order (profile, live
			-- membership, care) so SELECT ... FOR UPDATE on the view cannot
			-- deadlock against a routed UPDATE. The live enrollment is still
			-- the row that can disappear under a concurrent retirement: if it
			-- is gone after the profile lock, return no row without writing.
			PERFORM 1 FROM users.student_profiles AS profile
			WHERE profile.tenant_id = OLD.tenant_id AND profile.id = OLD.id
			FOR UPDATE;
			IF NOT FOUND THEN RETURN NULL; END IF;
			SELECT membership.id, membership.status INTO owning_membership, current_status
			FROM users.student_school_memberships AS membership
			WHERE membership.tenant_id = OLD.tenant_id AND membership.student_profile_id = OLD.id
			  AND membership.deleted_at IS NULL
			FOR UPDATE;
			IF NOT FOUND THEN RETURN NULL; END IF;
			PERFORM 1 FROM users.student_care_profiles AS care
			WHERE care.tenant_id = OLD.tenant_id AND care.membership_id = owning_membership
			FOR UPDATE;
			IF NOT FOUND THEN RETURN NULL; END IF;
			-- Heap EvalPlanQual would recheck WHERE status = OLD.status after
			-- waiting. INSTEAD OF has no WHERE, so a SET of status against a
			-- membership whose status moved is no row updated.
			IF NEW.status IS DISTINCT FROM OLD.status
				AND current_status IS DISTINCT FROM OLD.status THEN
				RETURN NULL;
			END IF;
			-- NEW was built from an unlocked snapshot. Apply only columns the
			-- UPDATE assigned; keep concurrently committed values for the rest.
			SELECT
				CASE WHEN NEW.person_id IS DISTINCT FROM OLD.person_id THEN NEW.person_id ELSE profile.person_id END,
				CASE WHEN NEW.address_street IS DISTINCT FROM OLD.address_street THEN NEW.address_street ELSE profile.address_street END,
				CASE WHEN NEW.address_city IS DISTINCT FROM OLD.address_city THEN NEW.address_city ELSE profile.address_city END,
				CASE WHEN NEW.address_postal_code IS DISTINCT FROM OLD.address_postal_code THEN NEW.address_postal_code ELSE profile.address_postal_code END,
				CASE WHEN NEW.extra_info IS DISTINCT FROM OLD.extra_info THEN NEW.extra_info ELSE profile.extra_info END,
				CASE WHEN NEW.photo_path IS DISTINCT FROM OLD.photo_path THEN NEW.photo_path ELSE profile.photo_path END,
				CASE WHEN NEW.photo_consent_given_at IS DISTINCT FROM OLD.photo_consent_given_at THEN NEW.photo_consent_given_at ELSE profile.photo_consent_given_at END,
				CASE WHEN NEW.photo_consent_given_by IS DISTINCT FROM OLD.photo_consent_given_by THEN NEW.photo_consent_given_by ELSE profile.photo_consent_given_by END,
				CASE WHEN NEW.agb_accepted_at IS DISTINCT FROM OLD.agb_accepted_at THEN NEW.agb_accepted_at ELSE profile.agb_accepted_at END,
				CASE WHEN NEW.data_processing_accepted_at IS DISTINCT FROM OLD.data_processing_accepted_at THEN NEW.data_processing_accepted_at ELSE profile.data_processing_accepted_at END,
				CASE WHEN NEW.email_contact_accepted_at IS DISTINCT FROM OLD.email_contact_accepted_at THEN NEW.email_contact_accepted_at ELSE profile.email_contact_accepted_at END,
				CASE WHEN NEW.created_at IS DISTINCT FROM OLD.created_at THEN NEW.created_at ELSE profile.created_at END,
				CASE WHEN NEW.school_class IS DISTINCT FROM OLD.school_class THEN NEW.school_class ELSE membership.school_class END,
				CASE WHEN NEW.group_id IS DISTINCT FROM OLD.group_id THEN NEW.group_id ELSE membership.group_id END,
				CASE WHEN NEW.status IS DISTINCT FROM OLD.status THEN NEW.status ELSE membership.status END,
				CASE WHEN NEW.enrolled_from IS DISTINCT FROM OLD.enrolled_from THEN NEW.enrolled_from ELSE membership.enrolled_from END,
				CASE WHEN NEW.enrolled_until IS DISTINCT FROM OLD.enrolled_until THEN NEW.enrolled_until ELSE membership.enrolled_until END,
				CASE WHEN NEW.supervisor_notes IS DISTINCT FROM OLD.supervisor_notes THEN NEW.supervisor_notes ELSE care.supervisor_notes END,
				CASE WHEN NEW.health_info IS DISTINCT FROM OLD.health_info THEN NEW.health_info ELSE care.health_info END,
				CASE WHEN NEW.pickup_status IS DISTINCT FROM OLD.pickup_status THEN NEW.pickup_status ELSE care.pickup_status END,
				CASE WHEN NEW.departure_days IS DISTINCT FROM OLD.departure_days THEN NEW.departure_days ELSE care.departure_days END,
				CASE WHEN NEW.allowed_departure_modes IS DISTINCT FROM OLD.allowed_departure_modes THEN NEW.allowed_departure_modes ELSE care.allowed_departure_modes END,
				CASE WHEN NEW.departure_companion_note IS DISTINCT FROM OLD.departure_companion_note THEN NEW.departure_companion_note ELSE care.departure_companion_note END,
				CASE WHEN NEW.pickup_days IS DISTINCT FROM OLD.pickup_days THEN NEW.pickup_days ELSE care.pickup_days END,
				CASE WHEN NEW.bus_days IS DISTINCT FROM OLD.bus_days THEN NEW.bus_days ELSE care.bus_days END
			INTO
				NEW.person_id, NEW.address_street, NEW.address_city, NEW.address_postal_code,
				NEW.extra_info, NEW.photo_path, NEW.photo_consent_given_at, NEW.photo_consent_given_by,
				NEW.agb_accepted_at, NEW.data_processing_accepted_at, NEW.email_contact_accepted_at,
				NEW.created_at, NEW.school_class, NEW.group_id, NEW.status, NEW.enrolled_from,
				NEW.enrolled_until, NEW.supervisor_notes, NEW.health_info, NEW.pickup_status,
				NEW.departure_days, NEW.allowed_departure_modes, NEW.departure_companion_note,
				NEW.pickup_days, NEW.bus_days
			FROM users.student_profiles AS profile
			JOIN users.student_school_memberships AS membership
				ON membership.tenant_id = profile.tenant_id AND membership.id = owning_membership
			JOIN users.student_care_profiles AS care
				ON care.tenant_id = membership.tenant_id AND care.membership_id = membership.id
			WHERE profile.tenant_id = OLD.tenant_id AND profile.id = OLD.id;
			UPDATE users.student_profiles SET
				person_id = NEW.person_id, address_street = NEW.address_street,
				address_city = NEW.address_city, address_postal_code = NEW.address_postal_code,
				extra_info = NEW.extra_info, photo_path = NEW.photo_path,
				photo_consent_given_at = NEW.photo_consent_given_at,
				photo_consent_given_by = NEW.photo_consent_given_by,
				agb_accepted_at = NEW.agb_accepted_at,
				data_processing_accepted_at = NEW.data_processing_accepted_at,
				email_contact_accepted_at = NEW.email_contact_accepted_at,
				created_at = NEW.created_at, updated_at = NEW.updated_at
			WHERE tenant_id = OLD.tenant_id AND id = OLD.id
			RETURNING updated_at INTO NEW.updated_at;
			UPDATE users.student_school_memberships SET
				school_class = NEW.school_class, group_id = NEW.group_id,
				status = COALESCE(NEW.status, 'active'), enrolled_from = NEW.enrolled_from,
				enrolled_until = NEW.enrolled_until, updated_at = NEW.updated_at
			WHERE tenant_id = OLD.tenant_id AND id = owning_membership;
			UPDATE users.student_care_profiles SET
				supervisor_notes = NEW.supervisor_notes, health_info = NEW.health_info,
				pickup_status = NEW.pickup_status,
				departure_days = COALESCE(NEW.departure_days, '{}'::jsonb),
				allowed_departure_modes = COALESCE(NEW.allowed_departure_modes, '{}'::jsonb),
				departure_companion_note = NEW.departure_companion_note,
				pickup_days = COALESCE(NEW.pickup_days, '{}'::jsonb),
				bus_days = COALESCE(NEW.bus_days, '{}'::jsonb),
				updated_at = NEW.updated_at
			WHERE tenant_id = OLD.tenant_id AND membership_id = owning_membership;
		END IF;

		-- Only the rollback interface writes the archive. A stale row of a
		-- child the current providers deleted still claims the per-person
		-- unique key, so it is released here rather than rejecting the write.
		DELETE FROM users.students_legacy
		WHERE tenant_id = NEW.tenant_id AND person_id = NEW.person_id AND id <> NEW.id;
		INSERT INTO users.students_legacy
			(id, tenant_id, person_id, school_class, guardian_name, guardian_contact,
			 guardian_email, guardian_phone, group_id, extra_info, created_at, updated_at,
			 supervisor_notes, health_info, pickup_status, sick, sick_since, excused,
			 excused_since, photo_path, photo_consent_given_at, photo_consent_given_by,
			 status, enrolled_from, enrolled_until, agb_accepted_at,
			 data_processing_accepted_at, email_contact_accepted_at, bus_days, pickup_days,
			 departure_days, allowed_departure_modes, departure_companion_note,
			 address_street, address_city, address_postal_code)
		VALUES (NEW.id, NEW.tenant_id, NEW.person_id, NEW.school_class, NEW.guardian_name,
			NEW.guardian_contact, NEW.guardian_email, NEW.guardian_phone, NEW.group_id,
			NEW.extra_info, NEW.created_at, NEW.updated_at, NEW.supervisor_notes,
			NEW.health_info, NEW.pickup_status, COALESCE(NEW.sick, false), NEW.sick_since,
			COALESCE(NEW.excused, false), NEW.excused_since, NEW.photo_path,
			NEW.photo_consent_given_at, NEW.photo_consent_given_by,
			COALESCE(NEW.status, 'active'), NEW.enrolled_from, NEW.enrolled_until,
			NEW.agb_accepted_at, NEW.data_processing_accepted_at, NEW.email_contact_accepted_at,
			COALESCE(NEW.bus_days, '{}'::jsonb), COALESCE(NEW.pickup_days, '{}'::jsonb),
			COALESCE(NEW.departure_days, '{}'::jsonb),
			COALESCE(NEW.allowed_departure_modes, '{}'::jsonb), NEW.departure_companion_note,
			NEW.address_street, NEW.address_city, NEW.address_postal_code)
		ON CONFLICT (id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id, person_id = EXCLUDED.person_id,
			school_class = EXCLUDED.school_class, guardian_name = EXCLUDED.guardian_name,
			guardian_contact = EXCLUDED.guardian_contact, guardian_email = EXCLUDED.guardian_email,
			guardian_phone = EXCLUDED.guardian_phone, group_id = EXCLUDED.group_id,
			extra_info = EXCLUDED.extra_info, created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at, supervisor_notes = EXCLUDED.supervisor_notes,
			health_info = EXCLUDED.health_info, pickup_status = EXCLUDED.pickup_status,
			sick = EXCLUDED.sick, sick_since = EXCLUDED.sick_since, excused = EXCLUDED.excused,
			excused_since = EXCLUDED.excused_since, photo_path = EXCLUDED.photo_path,
			photo_consent_given_at = EXCLUDED.photo_consent_given_at,
			photo_consent_given_by = EXCLUDED.photo_consent_given_by,
			status = EXCLUDED.status, enrolled_from = EXCLUDED.enrolled_from,
			enrolled_until = EXCLUDED.enrolled_until, agb_accepted_at = EXCLUDED.agb_accepted_at,
			data_processing_accepted_at = EXCLUDED.data_processing_accepted_at,
			email_contact_accepted_at = EXCLUDED.email_contact_accepted_at,
			bus_days = EXCLUDED.bus_days, pickup_days = EXCLUDED.pickup_days,
			departure_days = EXCLUDED.departure_days,
			allowed_departure_modes = EXCLUDED.allowed_departure_modes,
			departure_companion_note = EXCLUDED.departure_companion_note,
			address_street = EXCLUDED.address_street, address_city = EXCLUDED.address_city,
			address_postal_code = EXCLUDED.address_postal_code;
		RETURN NEW;
	END
	$function$;
	CREATE OR REPLACE TRIGGER student_compatibility_write
		INSTEAD OF INSERT OR UPDATE OR DELETE ON users.students
		FOR EACH ROW EXECUTE FUNCTION users.route_student_compatibility();`

// repointStudentForeignKeys moves every foreign key that names a student from
// the old base table onto users.student_profiles. The split preserved student
// identity, so each reference keeps the number it already holds; without this
// the constraints would follow the rename onto the archive and reject every
// dependent row of a child enrolled after Cutover.
//
// The list is spelled out rather than derived from pg_constraint at run time:
// the architecture ratchet cannot classify dynamic migration SQL, and a static
// list is also what makes this diff reviewable. The guard below fails the
// migration when the database holds a student foreign key this list does not,
// so a constraint added after this was written cannot be silently left behind.
//
// The constraints are added NOT VALID so the switch does not scan every
// dependent table while it holds the write lock. They are enforced for new and
// changed rows immediately; ValidateStudentOwnerForeignKeys confirms the
// existing ones afterwards, outside the lock.
func repointStudentForeignKeys(ctx context.Context, tx bun.Tx) error {
	if _, err := tx.ExecContext(ctx, `
			ALTER TABLE active.attendance DROP CONSTRAINT fk_attendance_student_tenant;
			ALTER TABLE active.attendance ADD CONSTRAINT fk_attendance_student_tenant
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE active.excused_absence_requests DROP CONSTRAINT excused_absence_requests_student_id_fkey;
			ALTER TABLE active.excused_absence_requests ADD CONSTRAINT excused_absence_requests_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE active.scheduled_checkouts DROP CONSTRAINT fk_scheduled_checkouts_student_tenant;
			ALTER TABLE active.scheduled_checkouts ADD CONSTRAINT fk_scheduled_checkouts_student_tenant
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE active.student_status_days DROP CONSTRAINT student_status_days_student_id_fkey;
			ALTER TABLE active.student_status_days ADD CONSTRAINT student_status_days_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE active.visits DROP CONSTRAINT fk_visits_student;
			ALTER TABLE active.visits ADD CONSTRAINT fk_visits_student
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE activities.student_enrollments DROP CONSTRAINT fk_enrollments_student_tenant;
			ALTER TABLE activities.student_enrollments ADD CONSTRAINT fk_enrollments_student_tenant
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE audit.attendance_corrections DROP CONSTRAINT attendance_corrections_student_id_fkey;
			ALTER TABLE audit.attendance_corrections ADD CONSTRAINT attendance_corrections_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE audit.data_access_log DROP CONSTRAINT data_access_log_student_id_fkey;
			ALTER TABLE audit.data_access_log ADD CONSTRAINT data_access_log_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE SET NULL NOT VALID;
			ALTER TABLE audit.enrollment_offering_adjustments DROP CONSTRAINT enrollment_offering_adjustments_student_id_fkey;
			ALTER TABLE audit.enrollment_offering_adjustments ADD CONSTRAINT enrollment_offering_adjustments_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE audit.guardian_changes DROP CONSTRAINT guardian_changes_student_id_fkey;
			ALTER TABLE audit.guardian_changes ADD CONSTRAINT guardian_changes_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE audit.student_consent_changes DROP CONSTRAINT student_consent_changes_student_id_fkey;
			ALTER TABLE audit.student_consent_changes ADD CONSTRAINT student_consent_changes_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE audit.student_field_edits DROP CONSTRAINT student_field_edits_student_id_fkey;
			ALTER TABLE audit.student_field_edits ADD CONSTRAINT student_field_edits_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE auth.guardian_invitations DROP CONSTRAINT fk_guardian_invitation_student;
			ALTER TABLE auth.guardian_invitations ADD CONSTRAINT fk_guardian_invitation_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE calendar.appointment_recipient_students DROP CONSTRAINT appointment_recipient_students_student_id_fkey;
			ALTER TABLE calendar.appointment_recipient_students ADD CONSTRAINT appointment_recipient_students_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE enrollment.offering_change_requests DROP CONSTRAINT offering_change_requests_student_id_fkey;
			ALTER TABLE enrollment.offering_change_requests ADD CONSTRAINT offering_change_requests_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE enrollment.request_children DROP CONSTRAINT request_children_created_student_id_fkey;
			ALTER TABLE enrollment.request_children ADD CONSTRAINT request_children_created_student_id_fkey
				FOREIGN KEY (created_student_id) REFERENCES users.student_profiles(id) ON DELETE SET NULL NOT VALID;
			ALTER TABLE enrollment.request_children DROP CONSTRAINT request_children_matched_student_id_fkey;
			ALTER TABLE enrollment.request_children ADD CONSTRAINT request_children_matched_student_id_fkey
				FOREIGN KEY (matched_student_id) REFERENCES users.student_profiles(id) ON DELETE SET NULL NOT VALID;
			ALTER TABLE feedback.entries DROP CONSTRAINT fk_feedback_student_tenant;
			ALTER TABLE feedback.entries ADD CONSTRAINT fk_feedback_student_tenant
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.care_schedule_change_requests DROP CONSTRAINT care_schedule_change_requests_student_id_fkey;
			ALTER TABLE schedule.care_schedule_change_requests ADD CONSTRAINT care_schedule_change_requests_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.grade_transition_roster_removals DROP CONSTRAINT fk_roster_removals_student;
			ALTER TABLE schedule.grade_transition_roster_removals ADD CONSTRAINT fk_roster_removals_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.instance_students DROP CONSTRAINT fk_instance_students_student_tenant;
			ALTER TABLE schedule.instance_students ADD CONSTRAINT fk_instance_students_student_tenant
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE RESTRICT NOT VALID;
			ALTER TABLE schedule.meal_participation_overrides DROP CONSTRAINT fk_meal_participation_override_student;
			ALTER TABLE schedule.meal_participation_overrides ADD CONSTRAINT fk_meal_participation_override_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.meal_participation_schedules DROP CONSTRAINT fk_meal_participation_schedule_student;
			ALTER TABLE schedule.meal_participation_schedules ADD CONSTRAINT fk_meal_participation_schedule_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.meal_sickness_status_history DROP CONSTRAINT fk_meal_sickness_history_student;
			ALTER TABLE schedule.meal_sickness_status_history ADD CONSTRAINT fk_meal_sickness_history_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.pickup_extension_tasks DROP CONSTRAINT fk_pickup_extension_task_student;
			ALTER TABLE schedule.pickup_extension_tasks ADD CONSTRAINT fk_pickup_extension_task_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.student_arrival_exceptions DROP CONSTRAINT student_arrival_exceptions_student_id_fkey;
			ALTER TABLE schedule.student_arrival_exceptions ADD CONSTRAINT student_arrival_exceptions_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.student_arrival_notes DROP CONSTRAINT student_arrival_notes_student_id_fkey;
			ALTER TABLE schedule.student_arrival_notes ADD CONSTRAINT student_arrival_notes_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.student_arrival_schedules DROP CONSTRAINT student_arrival_schedules_student_id_fkey;
			ALTER TABLE schedule.student_arrival_schedules ADD CONSTRAINT student_arrival_schedules_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.student_pickup_exceptions DROP CONSTRAINT fk_pickup_exceptions_student_tenant;
			ALTER TABLE schedule.student_pickup_exceptions ADD CONSTRAINT fk_pickup_exceptions_student_tenant
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.student_pickup_notes DROP CONSTRAINT fk_pickup_notes_student_tenant;
			ALTER TABLE schedule.student_pickup_notes ADD CONSTRAINT fk_pickup_notes_student_tenant
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE schedule.student_pickup_schedules DROP CONSTRAINT fk_pickup_schedules_student_tenant;
			ALTER TABLE schedule.student_pickup_schedules ADD CONSTRAINT fk_pickup_schedules_student_tenant
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.care_withdrawal_completions DROP CONSTRAINT fk_care_withdrawal_completion_student;
			ALTER TABLE users.care_withdrawal_completions ADD CONSTRAINT fk_care_withdrawal_completion_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE SET NULL (student_id) NOT VALID;
			ALTER TABLE users.parent_announcement_responses DROP CONSTRAINT parent_announcement_responses_student_id_fkey;
			ALTER TABLE users.parent_announcement_responses ADD CONSTRAINT parent_announcement_responses_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.parent_message_threads DROP CONSTRAINT parent_message_threads_student_id_fkey;
			ALTER TABLE users.parent_message_threads ADD CONSTRAINT parent_message_threads_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.parent_messages DROP CONSTRAINT parent_messages_student_id_fkey;
			ALTER TABLE users.parent_messages ADD CONSTRAINT parent_messages_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.parent_request_events DROP CONSTRAINT parent_request_events_student_id_fkey;
			ALTER TABLE users.parent_request_events ADD CONSTRAINT parent_request_events_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.parent_request_share_events DROP CONSTRAINT parent_request_share_events_student_id_fkey;
			ALTER TABLE users.parent_request_share_events ADD CONSTRAINT parent_request_share_events_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.privacy_consents DROP CONSTRAINT fk_privacy_consents_student_tenant;
			ALTER TABLE users.privacy_consents ADD CONSTRAINT fk_privacy_consents_student_tenant
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.student_care_exit_removals DROP CONSTRAINT fk_care_exit_removal_student;
			ALTER TABLE users.student_care_exit_removals ADD CONSTRAINT fk_care_exit_removal_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.student_care_exit_source_removals DROP CONSTRAINT fk_care_exit_source_removal_student;
			ALTER TABLE users.student_care_exit_source_removals ADD CONSTRAINT fk_care_exit_source_removal_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.student_care_exits DROP CONSTRAINT fk_student_care_exits_student;
			ALTER TABLE users.student_care_exits ADD CONSTRAINT fk_student_care_exits_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.student_companions DROP CONSTRAINT fk_student_companions_high;
			ALTER TABLE users.student_companions ADD CONSTRAINT fk_student_companions_high
				FOREIGN KEY (tenant_id, student_high_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.student_companions DROP CONSTRAINT fk_student_companions_low;
			ALTER TABLE users.student_companions ADD CONSTRAINT fk_student_companions_low
				FOREIGN KEY (tenant_id, student_low_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.student_data_change_requests DROP CONSTRAINT student_data_change_requests_student_id_fkey;
			ALTER TABLE users.student_data_change_requests ADD CONSTRAINT student_data_change_requests_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.student_documents DROP CONSTRAINT fk_student_documents_student;
			ALTER TABLE users.student_documents ADD CONSTRAINT fk_student_documents_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.student_family_protection_events DROP CONSTRAINT student_family_protection_events_student_id_fkey;
			ALTER TABLE users.student_family_protection_events ADD CONSTRAINT student_family_protection_events_student_id_fkey
				FOREIGN KEY (student_id) REFERENCES users.student_profiles(id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.student_guardian_relationships DROP CONSTRAINT fk_student_guardian_relationships_student;
			ALTER TABLE users.student_guardian_relationships ADD CONSTRAINT fk_student_guardian_relationships_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			ALTER TABLE users.students_guardians DROP CONSTRAINT fk_students_guardians_student_tenant;
			ALTER TABLE users.students_guardians ADD CONSTRAINT fk_students_guardians_student_tenant
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE NOT VALID;
			DO $$ BEGIN
				IF EXISTS (SELECT 1 FROM pg_constraint
					WHERE confrelid = 'users.students'::regclass AND contype = 'f') THEN
					RAISE EXCEPTION 'a student foreign key was added after this migration was written; repoint it here before Cutover';
				END IF;
			END $$;
	`); err != nil {
		return fmt.Errorf("repoint student foreign keys: %w", err)
	}
	return nil
}

// ValidateStudentOwnerForeignKeys confirms the repointed constraints against
// the rows that already exist. It runs after the switch transaction:
// VALIDATE takes a share-update-exclusive lock on the dependent table and a
// row-share lock on the profiles, so ordinary reads and writes continue while
// it scans. Validating a constraint that is already valid is a no-op, so an
// interrupted run is simply repeated — the compatibility command repeats it.
func ValidateStudentOwnerForeignKeys(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("student owner cutover: database is required")
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE active.attendance VALIDATE CONSTRAINT fk_attendance_student_tenant;
		ALTER TABLE active.excused_absence_requests VALIDATE CONSTRAINT excused_absence_requests_student_id_fkey;
		ALTER TABLE active.scheduled_checkouts VALIDATE CONSTRAINT fk_scheduled_checkouts_student_tenant;
		ALTER TABLE active.student_status_days VALIDATE CONSTRAINT student_status_days_student_id_fkey;
		ALTER TABLE active.visits VALIDATE CONSTRAINT fk_visits_student;
		ALTER TABLE activities.student_enrollments VALIDATE CONSTRAINT fk_enrollments_student_tenant;
		ALTER TABLE audit.attendance_corrections VALIDATE CONSTRAINT attendance_corrections_student_id_fkey;
		ALTER TABLE audit.data_access_log VALIDATE CONSTRAINT data_access_log_student_id_fkey;
		ALTER TABLE audit.enrollment_offering_adjustments VALIDATE CONSTRAINT enrollment_offering_adjustments_student_id_fkey;
		ALTER TABLE audit.guardian_changes VALIDATE CONSTRAINT guardian_changes_student_id_fkey;
		ALTER TABLE audit.student_consent_changes VALIDATE CONSTRAINT student_consent_changes_student_id_fkey;
		ALTER TABLE audit.student_field_edits VALIDATE CONSTRAINT student_field_edits_student_id_fkey;
		ALTER TABLE auth.guardian_invitations VALIDATE CONSTRAINT fk_guardian_invitation_student;
		ALTER TABLE calendar.appointment_recipient_students VALIDATE CONSTRAINT appointment_recipient_students_student_id_fkey;
		ALTER TABLE enrollment.offering_change_requests VALIDATE CONSTRAINT offering_change_requests_student_id_fkey;
		ALTER TABLE enrollment.request_children VALIDATE CONSTRAINT request_children_created_student_id_fkey;
		ALTER TABLE enrollment.request_children VALIDATE CONSTRAINT request_children_matched_student_id_fkey;
		ALTER TABLE feedback.entries VALIDATE CONSTRAINT fk_feedback_student_tenant;
		ALTER TABLE schedule.care_schedule_change_requests VALIDATE CONSTRAINT care_schedule_change_requests_student_id_fkey;
		ALTER TABLE schedule.grade_transition_roster_removals VALIDATE CONSTRAINT fk_roster_removals_student;
		ALTER TABLE schedule.instance_students VALIDATE CONSTRAINT fk_instance_students_student_tenant;
		ALTER TABLE schedule.meal_participation_overrides VALIDATE CONSTRAINT fk_meal_participation_override_student;
		ALTER TABLE schedule.meal_participation_schedules VALIDATE CONSTRAINT fk_meal_participation_schedule_student;
		ALTER TABLE schedule.meal_sickness_status_history VALIDATE CONSTRAINT fk_meal_sickness_history_student;
		ALTER TABLE schedule.pickup_extension_tasks VALIDATE CONSTRAINT fk_pickup_extension_task_student;
		ALTER TABLE schedule.student_arrival_exceptions VALIDATE CONSTRAINT student_arrival_exceptions_student_id_fkey;
		ALTER TABLE schedule.student_arrival_notes VALIDATE CONSTRAINT student_arrival_notes_student_id_fkey;
		ALTER TABLE schedule.student_arrival_schedules VALIDATE CONSTRAINT student_arrival_schedules_student_id_fkey;
		ALTER TABLE schedule.student_pickup_exceptions VALIDATE CONSTRAINT fk_pickup_exceptions_student_tenant;
		ALTER TABLE schedule.student_pickup_notes VALIDATE CONSTRAINT fk_pickup_notes_student_tenant;
		ALTER TABLE schedule.student_pickup_schedules VALIDATE CONSTRAINT fk_pickup_schedules_student_tenant;
		ALTER TABLE users.care_withdrawal_completions VALIDATE CONSTRAINT fk_care_withdrawal_completion_student;
		ALTER TABLE users.parent_announcement_responses VALIDATE CONSTRAINT parent_announcement_responses_student_id_fkey;
		ALTER TABLE users.parent_message_threads VALIDATE CONSTRAINT parent_message_threads_student_id_fkey;
		ALTER TABLE users.parent_messages VALIDATE CONSTRAINT parent_messages_student_id_fkey;
		ALTER TABLE users.parent_request_events VALIDATE CONSTRAINT parent_request_events_student_id_fkey;
		ALTER TABLE users.parent_request_share_events VALIDATE CONSTRAINT parent_request_share_events_student_id_fkey;
		ALTER TABLE users.privacy_consents VALIDATE CONSTRAINT fk_privacy_consents_student_tenant;
		ALTER TABLE users.student_care_exit_removals VALIDATE CONSTRAINT fk_care_exit_removal_student;
		ALTER TABLE users.student_care_exit_source_removals VALIDATE CONSTRAINT fk_care_exit_source_removal_student;
		ALTER TABLE users.student_care_exits VALIDATE CONSTRAINT fk_student_care_exits_student;
		ALTER TABLE users.student_companions VALIDATE CONSTRAINT fk_student_companions_high;
		ALTER TABLE users.student_companions VALIDATE CONSTRAINT fk_student_companions_low;
		ALTER TABLE users.student_data_change_requests VALIDATE CONSTRAINT student_data_change_requests_student_id_fkey;
		ALTER TABLE users.student_documents VALIDATE CONSTRAINT fk_student_documents_student;
		ALTER TABLE users.student_family_protection_events VALIDATE CONSTRAINT student_family_protection_events_student_id_fkey;
		ALTER TABLE users.student_guardian_relationships VALIDATE CONSTRAINT fk_student_guardian_relationships_student;
		ALTER TABLE users.students_guardians VALIDATE CONSTRAINT fk_students_guardians_student_tenant;
	`); err != nil {
		return fmt.Errorf("student owner cutover: validate repointed foreign keys: %w", err)
	}
	return nil
}
