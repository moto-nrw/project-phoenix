-- Historical test-only rollback mirror schema at migration 1.15.427.
-- Captured from a disposable local migration database with PostgreSQL pg_dump
-- (schema-only, users.students_guardians, its owned id sequence and the
-- compatibility counter) and pg_get_functiondef/pg_get_triggerdef for the
-- mirror, routing, single-primary and pre-Cutover grant invalidation functions
-- and the owner mirror triggers. No data included. Session settings, owners and
-- psql directives removed; constraints, RLS and grants retained.
CREATE OR REPLACE FUNCTION users.enforce_single_primary_student_guardian()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
		BEGIN
			-- If this is being set as primary
			IF NEW.is_primary = TRUE THEN
				-- Set any other relationships for this student to non-primary
				UPDATE users.students_guardians
				SET is_primary = FALSE
				WHERE student_id = NEW.student_id
				AND id != NEW.id;
			END IF;
			RETURN NEW;
		END;
		$function$
;
CREATE OR REPLACE FUNCTION meta.invalidate_parent_student_consent_permission_grant()
 RETURNS trigger
 LANGUAGE plpgsql
 SECURITY DEFINER
 SET search_path TO 'pg_catalog', 'meta'
AS $function$
		BEGIN
			DELETE FROM meta.parent_student_consent_permission_grants
			WHERE student_guardian_id = OLD.id;
			RETURN NEW;
		END;
		$function$
;
CREATE OR REPLACE FUNCTION meta.invalidate_meal_participation_permission_grant()
 RETURNS trigger
 LANGUAGE plpgsql
 SECURITY DEFINER
 SET search_path TO 'pg_catalog', 'meta'
AS $function$
		BEGIN
			DELETE FROM meta.meal_participation_permission_grants
			WHERE student_guardian_id = OLD.id;
			RETURN NEW;
		END;
		$function$
;
CREATE OR REPLACE FUNCTION users.route_students_guardians_compatibility()
 RETURNS trigger
 LANGUAGE plpgsql
 SET search_path TO 'pg_catalog'
AS $function$
	BEGIN
		IF coalesce(current_setting('users.guardian_compat_mirror', true), '') = 'on' THEN
			RETURN NULL;
		END IF;
		-- The mirror's student foreign key cascades after the child row is
		-- gone, beside the relationship's own cascade. That delete is not a
		-- previous-image statement; the relationship follows its child anyway.
		IF TG_OP = 'DELETE' AND NOT EXISTS (
			SELECT 1 FROM users.student_profiles AS s WHERE s.tenant_id = OLD.tenant_id AND s.id = OLD.student_id) THEN
			RETURN NULL;
		END IF;
		IF TG_OP = 'UPDATE' AND (NEW.id, NEW.tenant_id) IS DISTINCT FROM (OLD.id, OLD.tenant_id) THEN
			RAISE EXCEPTION 'student guardian identity is immutable' USING ERRCODE = '23514';
		END IF;
		PERFORM nextval('users.students_guardians_compatibility_writes');
		PERFORM set_config('users.guardian_compat_routing', 'on', true);
		IF TG_OP = 'DELETE' THEN
			-- The pickup permission and the access row follow through their
			-- foreign-key cascade.
			DELETE FROM users.student_guardian_relationships WHERE tenant_id = OLD.tenant_id AND id = OLD.id;
		ELSE
			INSERT INTO users.student_guardian_relationships AS r
				(id, tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role,
				 is_primary, is_emergency_contact, emergency_priority, is_payer, created_at, updated_at)
			VALUES (NEW.id, NEW.tenant_id, NEW.student_id, NEW.guardian_profile_id, NEW.relationship_type, NEW.guardian_role,
				NEW.is_primary, NEW.is_emergency_contact, NEW.emergency_priority, NEW.is_payer, NEW.created_at, NEW.updated_at)
			ON CONFLICT (id) DO UPDATE SET
				student_id = EXCLUDED.student_id, guardian_profile_id = EXCLUDED.guardian_profile_id,
				relationship_type = EXCLUDED.relationship_type, guardian_role = EXCLUDED.guardian_role,
				is_primary = EXCLUDED.is_primary, is_emergency_contact = EXCLUDED.is_emergency_contact,
				emergency_priority = EXCLUDED.emergency_priority, is_payer = EXCLUDED.is_payer
			WHERE ROW(r.student_id, r.guardian_profile_id, r.relationship_type, r.guardian_role,
			          r.is_primary, r.is_emergency_contact, r.emergency_priority, r.is_payer)
			      IS DISTINCT FROM ROW(EXCLUDED.student_id, EXCLUDED.guardian_profile_id, EXCLUDED.relationship_type,
			          EXCLUDED.guardian_role, EXCLUDED.is_primary, EXCLUDED.is_emergency_contact,
			          EXCLUDED.emergency_priority, EXCLUDED.is_payer);
			INSERT INTO users.student_guardian_pickup_permissions AS p
				(tenant_id, relationship_id, can_pickup, pickup_notes, created_at, updated_at)
			VALUES (NEW.tenant_id, NEW.id, NEW.can_pickup, NEW.pickup_notes, NEW.created_at, NEW.updated_at)
			ON CONFLICT (tenant_id, relationship_id) DO UPDATE SET
				can_pickup = EXCLUDED.can_pickup, pickup_notes = EXCLUDED.pickup_notes
			WHERE ROW(p.can_pickup, p.pickup_notes) IS DISTINCT FROM ROW(EXCLUDED.can_pickup, EXCLUDED.pickup_notes);
			INSERT INTO auth.guardian_student_access AS a
				(tenant_id, relationship_id, account_id, permissions, created_at, updated_at)
			SELECT NEW.tenant_id, NEW.id, g.account_id, NEW.permissions, NEW.created_at, NEW.updated_at
			FROM users.guardian_profiles AS g
			WHERE g.tenant_id = NEW.tenant_id AND g.id = NEW.guardian_profile_id
			ON CONFLICT (tenant_id, relationship_id) DO UPDATE SET
				account_id = EXCLUDED.account_id, permissions = EXCLUDED.permissions
			WHERE ROW(a.account_id, a.permissions) IS DISTINCT FROM ROW(EXCLUDED.account_id, EXCLUDED.permissions);
		END IF;
		PERFORM set_config('users.guardian_compat_routing', 'off', true);
		RETURN NULL;
	END
	$function$
;
CREATE OR REPLACE FUNCTION users.mirror_student_guardian_relationship()
 RETURNS trigger
 LANGUAGE plpgsql
 SET search_path TO 'pg_catalog'
AS $function$
	BEGIN
		IF coalesce(current_setting('users.guardian_compat_routing', true), '') = 'on' THEN
			RETURN NULL;
		END IF;
		PERFORM set_config('users.guardian_compat_mirror', 'on', true);
		IF TG_OP = 'DELETE' THEN
			DELETE FROM users.students_guardians WHERE tenant_id = OLD.tenant_id AND id = OLD.id;
		ELSIF TG_OP = 'INSERT' THEN
			INSERT INTO users.students_guardians
				(id, tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role,
				 is_primary, is_emergency_contact, emergency_priority, is_payer, created_at, updated_at)
			VALUES (NEW.id, NEW.tenant_id, NEW.student_id, NEW.guardian_profile_id, NEW.relationship_type, NEW.guardian_role,
				NEW.is_primary, NEW.is_emergency_contact, NEW.emergency_priority, NEW.is_payer, NEW.created_at, NEW.updated_at);
		ELSE
			UPDATE users.students_guardians AS sg SET
				student_id = NEW.student_id, guardian_profile_id = NEW.guardian_profile_id,
				relationship_type = NEW.relationship_type, guardian_role = NEW.guardian_role,
				is_primary = NEW.is_primary, is_emergency_contact = NEW.is_emergency_contact,
				emergency_priority = NEW.emergency_priority, is_payer = NEW.is_payer
			WHERE sg.tenant_id = OLD.tenant_id AND sg.id = OLD.id
			  AND ROW(sg.student_id, sg.guardian_profile_id, sg.relationship_type, sg.guardian_role,
			          sg.is_primary, sg.is_emergency_contact, sg.emergency_priority, sg.is_payer)
			      IS DISTINCT FROM ROW(NEW.student_id, NEW.guardian_profile_id, NEW.relationship_type, NEW.guardian_role,
			          NEW.is_primary, NEW.is_emergency_contact, NEW.emergency_priority, NEW.is_payer);
		END IF;
		PERFORM set_config('users.guardian_compat_mirror', 'off', true);
		RETURN NULL;
	END
	$function$
;
CREATE OR REPLACE FUNCTION users.mirror_student_guardian_pickup_permission()
 RETURNS trigger
 LANGUAGE plpgsql
 SET search_path TO 'pg_catalog'
AS $function$
	BEGIN
		IF TG_OP = 'DELETE' OR coalesce(current_setting('users.guardian_compat_routing', true), '') = 'on' THEN
			RETURN NULL;
		END IF;
		PERFORM set_config('users.guardian_compat_mirror', 'on', true);
		UPDATE users.students_guardians AS sg SET can_pickup = NEW.can_pickup, pickup_notes = NEW.pickup_notes
		WHERE sg.tenant_id = NEW.tenant_id AND sg.id = NEW.relationship_id
		  AND ROW(sg.can_pickup, sg.pickup_notes) IS DISTINCT FROM ROW(NEW.can_pickup, NEW.pickup_notes);
		PERFORM set_config('users.guardian_compat_mirror', 'off', true);
		RETURN NULL;
	END
	$function$
;
CREATE OR REPLACE FUNCTION auth.mirror_guardian_student_access()
 RETURNS trigger
 LANGUAGE plpgsql
 SET search_path TO 'pg_catalog'
AS $function$
	BEGIN
		IF TG_OP = 'DELETE' OR coalesce(current_setting('users.guardian_compat_routing', true), '') = 'on' THEN
			RETURN NULL;
		END IF;
		PERFORM set_config('users.guardian_compat_mirror', 'on', true);
		UPDATE users.students_guardians AS sg SET permissions = NEW.permissions
		WHERE sg.tenant_id = NEW.tenant_id AND sg.id = NEW.relationship_id
		  AND sg.permissions IS DISTINCT FROM NEW.permissions;
		PERFORM set_config('users.guardian_compat_mirror', 'off', true);
		RETURN NULL;
	END
	$function$
;
REVOKE ALL ON FUNCTION meta.invalidate_parent_student_consent_permission_grant() FROM PUBLIC;
REVOKE ALL ON FUNCTION meta.invalidate_meal_participation_permission_grant() FROM PUBLIC;
CREATE TABLE users.students_guardians (
    id bigint DEFAULT nextval('users.student_guardian_relationships_id_seq'::regclass) NOT NULL,
    student_id bigint NOT NULL,
    guardian_profile_id bigint NOT NULL,
    relationship_type text NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    is_emergency_contact boolean DEFAULT false NOT NULL,
    can_pickup boolean DEFAULT false NOT NULL,
    pickup_notes text,
    emergency_priority integer DEFAULT 1,
    permissions jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    tenant_id bigint NOT NULL,
    guardian_role text DEFAULT 'custom'::text NOT NULL,
    is_payer boolean DEFAULT false NOT NULL,
    CONSTRAINT chk_students_guardians_tenant_id_not_null CHECK ((tenant_id IS NOT NULL))
);
ALTER TABLE ONLY users.students_guardians FORCE ROW LEVEL SECURITY;
COMMENT ON TABLE users.students_guardians IS 'Rollback-only mirror of users.student_guardian_relationships, users.student_guardian_pickup_permissions and auth.guardian_student_access (#2756). Kept by triggers; nothing current reads or writes it. #2757 removes it.';
COMMENT ON COLUMN users.students_guardians.is_payer IS 'This guardian''s bank account is charged for this child (#2608); at most one per child';
CREATE SEQUENCE users.students_guardians_compatibility_writes
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
COMMENT ON SEQUENCE users.students_guardians_compatibility_writes IS 'Rows the previous image wrote into the rollback-only users.students_guardians mirror and the routing trigger copied into the owner tables (#2756). Must trend to zero before #2757.';
CREATE SEQUENCE users.students_guardians_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE users.students_guardians_id_seq OWNED BY users.students_guardians.id;
ALTER TABLE ONLY users.students_guardians
    ADD CONSTRAINT students_guardians_pkey PRIMARY KEY (id);
CREATE INDEX idx_students_guardians_can_pickup ON users.students_guardians USING btree (can_pickup);
CREATE INDEX idx_students_guardians_guardian_profile_id ON users.students_guardians USING btree (guardian_profile_id);
CREATE INDEX idx_students_guardians_is_emergency_contact ON users.students_guardians USING btree (is_emergency_contact);
CREATE INDEX idx_students_guardians_is_primary ON users.students_guardians USING btree (is_primary);
CREATE INDEX idx_students_guardians_relationship_type ON users.students_guardians USING btree (relationship_type);
CREATE INDEX idx_students_guardians_student_id ON users.students_guardians USING btree (student_id);
CREATE UNIQUE INDEX idx_students_guardians_tenant ON users.students_guardians USING btree (tenant_id, student_id, guardian_profile_id);
CREATE INDEX idx_students_guardians_tenant_id ON users.students_guardians USING btree (tenant_id, id);
CREATE UNIQUE INDEX uq_students_guardians_payer ON users.students_guardians USING btree (tenant_id, student_id) WHERE is_payer;
CREATE TRIGGER enforce_single_primary_student_guardian_trigger BEFORE INSERT OR UPDATE OF is_primary ON users.students_guardians FOR EACH ROW WHEN ((new.is_primary = true)) EXECUTE FUNCTION users.enforce_single_primary_student_guardian();
CREATE TRIGGER students_guardians_route_compatibility AFTER INSERT OR DELETE OR UPDATE ON users.students_guardians FOR EACH ROW EXECUTE FUNCTION users.route_students_guardians_compatibility();
CREATE TRIGGER update_students_guardians_updated_at BEFORE UPDATE ON users.students_guardians FOR EACH ROW EXECUTE FUNCTION public.update_modified_column();
ALTER TABLE ONLY users.students_guardians
    ADD CONSTRAINT fk_students_guardians_guardian_tenant FOREIGN KEY (tenant_id, guardian_profile_id) REFERENCES users.guardian_profiles(tenant_id, id) ON DELETE RESTRICT;
ALTER TABLE ONLY users.students_guardians
    ADD CONSTRAINT fk_students_guardians_student_tenant FOREIGN KEY (tenant_id, student_id) REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE;
ALTER TABLE ONLY users.students_guardians
    ADD CONSTRAINT students_guardians_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES platform.schools(id);
ALTER TABLE users.students_guardians ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_users_students_guardians ON users.students_guardians USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::bigint)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::bigint));
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE users.students_guardians TO phoenix_tenant;
GRANT ALL ON TABLE users.students_guardians TO phoenix_admin;
GRANT SELECT ON TABLE users.students_guardians TO phoenix_auth;
GRANT USAGE ON SEQUENCE users.students_guardians_compatibility_writes TO phoenix_tenant;
GRANT ALL ON SEQUENCE users.students_guardians_compatibility_writes TO phoenix_admin;
GRANT USAGE ON SEQUENCE users.students_guardians_id_seq TO phoenix_tenant;
GRANT ALL ON SEQUENCE users.students_guardians_id_seq TO phoenix_admin;
CREATE TRIGGER guardian_student_access_mirror AFTER INSERT OR DELETE OR UPDATE ON auth.guardian_student_access FOR EACH ROW EXECUTE FUNCTION auth.mirror_guardian_student_access();
CREATE TRIGGER student_guardian_pickup_permissions_mirror AFTER INSERT OR DELETE OR UPDATE ON users.student_guardian_pickup_permissions FOR EACH ROW EXECUTE FUNCTION users.mirror_student_guardian_pickup_permission();
CREATE TRIGGER student_guardian_relationships_mirror AFTER INSERT OR DELETE OR UPDATE ON users.student_guardian_relationships FOR EACH ROW EXECUTE FUNCTION users.mirror_student_guardian_relationship();
