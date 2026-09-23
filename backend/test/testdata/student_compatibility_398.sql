-- Frozen migration 1.15.398 compatibility definitions, test-only.
-- Source: student_care_compatibility.go at 4aa36c315fc19e8ae8dda50193ffa34843e25575.
CREATE SEQUENCE users.student_compatibility_reads;
CREATE SEQUENCE users.student_compatibility_writes;
GRANT USAGE ON SEQUENCE users.student_compatibility_reads, users.student_compatibility_writes TO phoenix_tenant;
GRANT USAGE, SELECT ON SEQUENCE users.student_compatibility_reads, users.student_compatibility_writes TO phoenix_admin;

	CREATE OR REPLACE VIEW users.students WITH (security_invoker = true) AS
	SELECT b.id, b.person_id, b.school_class,
		(b.archived).guardian_name, (b.archived).guardian_contact,
		(b.archived).guardian_email, (b.archived).guardian_phone,
		b.group_id, b.extra_info, b.created_at, b.updated_at,
		b.supervisor_notes, b.health_info, b.pickup_status,
		b.sick, b.sick_since,
		b.tenant_id,
		b.excused, b.excused_since,
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
			c.sick, c.sick_since, c.excused, c.excused_since,
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
	WHERE pc.expires_at < CURRENT_TIMESTAMP AND pc.accepted = true AND pc.renewal_required = true;

	CREATE OR REPLACE FUNCTION users.route_student_compatibility()
	RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
	DECLARE
		owning_membership bigint;
		current_status text;
		archived users.students_legacy%ROWTYPE;
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
				 pickup_days, bus_days, sick, sick_since, excused, excused_since, created_at, updated_at)
			VALUES (owning_membership, NEW.tenant_id, NEW.supervisor_notes, NEW.health_info,
				NEW.pickup_status, COALESCE(NEW.departure_days, '{}'::jsonb),
				COALESCE(NEW.allowed_departure_modes, '{}'::jsonb), NEW.departure_companion_note,
				COALESCE(NEW.pickup_days, '{}'::jsonb), COALESCE(NEW.bus_days, '{}'::jsonb),
				COALESCE(NEW.sick, false), NEW.sick_since, COALESCE(NEW.excused, false), NEW.excused_since,
				NEW.created_at, NEW.updated_at);
		ELSE
			-- Lock profile, then the archive, then live membership and care.
			-- Profile first matches SELECT ... FOR UPDATE on the view so the
			-- pair cannot deadlock. The live enrollment is still the row that
			-- can disappear under a concurrent retirement: if it is gone after
			-- the profile lock, return no row without writing.
			PERFORM 1 FROM users.student_profiles AS profile
			WHERE profile.tenant_id = OLD.tenant_id AND profile.id = OLD.id
			FOR UPDATE;
			IF NOT FOUND THEN RETURN NULL; END IF;
			-- Guardian copies live only on the archive. NEW still
			-- holds the pre-wait snapshot of them; merge unassigned columns
			-- from the locked row so a later extra_info write cannot restore
			-- a committed UpdateLiveStatus or guardian SET.
			SELECT * INTO archived
			FROM users.students_legacy AS legacy
			WHERE legacy.tenant_id = OLD.tenant_id AND legacy.id = OLD.id
			FOR UPDATE;
			IF FOUND THEN
				IF NEW.guardian_name IS NOT DISTINCT FROM OLD.guardian_name THEN
					NEW.guardian_name := archived.guardian_name;
				END IF;
				IF NEW.guardian_contact IS NOT DISTINCT FROM OLD.guardian_contact THEN
					NEW.guardian_contact := archived.guardian_contact;
				END IF;
				IF NEW.guardian_email IS NOT DISTINCT FROM OLD.guardian_email THEN
					NEW.guardian_email := archived.guardian_email;
				END IF;
				IF NEW.guardian_phone IS NOT DISTINCT FROM OLD.guardian_phone THEN
					NEW.guardian_phone := archived.guardian_phone;
				END IF;
			END IF;
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
				CASE WHEN NEW.bus_days IS DISTINCT FROM OLD.bus_days THEN NEW.bus_days ELSE care.bus_days END,
				CASE WHEN NEW.sick IS DISTINCT FROM OLD.sick THEN NEW.sick ELSE care.sick END,
				CASE WHEN NEW.sick_since IS DISTINCT FROM OLD.sick_since THEN NEW.sick_since ELSE care.sick_since END,
				CASE WHEN NEW.excused IS DISTINCT FROM OLD.excused THEN NEW.excused ELSE care.excused END,
				CASE WHEN NEW.excused_since IS DISTINCT FROM OLD.excused_since THEN NEW.excused_since ELSE care.excused_since END
			INTO
				NEW.person_id, NEW.address_street, NEW.address_city, NEW.address_postal_code,
				NEW.extra_info, NEW.photo_path, NEW.photo_consent_given_at, NEW.photo_consent_given_by,
				NEW.agb_accepted_at, NEW.data_processing_accepted_at, NEW.email_contact_accepted_at,
				NEW.created_at, NEW.school_class, NEW.group_id, NEW.status, NEW.enrolled_from,
				NEW.enrolled_until, NEW.supervisor_notes, NEW.health_info, NEW.pickup_status,
				NEW.departure_days, NEW.allowed_departure_modes, NEW.departure_companion_note,
				NEW.pickup_days, NEW.bus_days, NEW.sick, NEW.sick_since, NEW.excused, NEW.excused_since
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
				sick = COALESCE(NEW.sick, false), sick_since = NEW.sick_since,
				excused = COALESCE(NEW.excused, false), excused_since = NEW.excused_since,
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
		FOR EACH ROW EXECUTE FUNCTION users.route_student_compatibility();
