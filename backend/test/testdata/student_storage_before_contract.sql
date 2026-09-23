-- Historical test-only archive schema at migration 1.15.398.
-- Captured from a disposable local migration template with PostgreSQL 17 pg_dump,
-- schema-only, users.students_legacy and its owned sequence. No data included.
-- Session settings and psql directives removed; constraints, RLS and grants retained.
CREATE TABLE users.students_legacy (
    id bigint NOT NULL,
    person_id bigint NOT NULL,
    school_class text NOT NULL,
    guardian_name text,
    guardian_contact text,
    guardian_email text,
    guardian_phone text,
    group_id bigint,
    extra_info text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    supervisor_notes text,
    health_info text,
    pickup_status text,
    sick boolean DEFAULT false,
    sick_since timestamp without time zone,
    tenant_id bigint NOT NULL,
    excused boolean DEFAULT false,
    excused_since timestamp without time zone,
    photo_path text,
    photo_consent_given_at timestamp with time zone,
    photo_consent_given_by bigint,
    status text DEFAULT 'active'::text NOT NULL,
    enrolled_from date,
    enrolled_until date,
    agb_accepted_at timestamp with time zone,
    data_processing_accepted_at timestamp with time zone,
    email_contact_accepted_at timestamp with time zone,
    bus_days jsonb DEFAULT '{}'::jsonb NOT NULL,
    pickup_days jsonb DEFAULT '{}'::jsonb NOT NULL,
    departure_days jsonb DEFAULT '{}'::jsonb NOT NULL,
    allowed_departure_modes jsonb DEFAULT '{}'::jsonb NOT NULL,
    departure_companion_note text,
    address_street text,
    address_city text,
    address_postal_code text,
    CONSTRAINT check_students_allowed_departure_modes CHECK (users.is_valid_allowed_departure_modes(allowed_departure_modes)),
    CONSTRAINT check_students_bus_days CHECK (users.is_valid_bus_days(bus_days)),
    CONSTRAINT check_students_departure_days CHECK (users.is_valid_departure_days(departure_days)),
    CONSTRAINT check_students_pickup_days CHECK (users.is_valid_pickup_days(pickup_days)),
    CONSTRAINT chk_students_tenant_id_not_null CHECK ((tenant_id IS NOT NULL)),
    CONSTRAINT chk_valid_guardian_email CHECK (((guardian_email IS NULL) OR public.is_valid_email(guardian_email))),
    CONSTRAINT chk_valid_guardian_phone CHECK (((guardian_phone IS NULL) OR public.is_valid_phone(guardian_phone))),
    CONSTRAINT students_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'active'::text, 'inactive'::text, 'alumnus'::text])))
);
ALTER TABLE ONLY users.students_legacy FORCE ROW LEVEL SECURITY;
COMMENT ON TABLE users.students_legacy IS 'Rollback-only archive of the pre-Cutover users.students (#2759). The three owner tables are authoritative; only the compatibility view writes this table.';
COMMENT ON COLUMN users.students_legacy.supervisor_notes IS 'Notes about the student that can be edited by supervisors (Betreuernotizen)';
COMMENT ON COLUMN users.students_legacy.health_info IS 'Static health and medical information about the student (Gesundheitsinfos)';
COMMENT ON COLUMN users.students_legacy.address_street IS 'Child address street and house number';
COMMENT ON COLUMN users.students_legacy.address_city IS 'Child address city';
COMMENT ON COLUMN users.students_legacy.address_postal_code IS 'Child address postal code';
CREATE SEQUENCE users.students_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE users.students_id_seq OWNED BY users.students_legacy.id;
ALTER TABLE ONLY users.students_legacy ALTER COLUMN id SET DEFAULT nextval('users.students_id_seq'::regclass);
ALTER TABLE ONLY users.students_legacy
    ADD CONSTRAINT students_pkey PRIMARY KEY (id);
CREATE INDEX idx_students_excused ON users.students_legacy USING btree (excused) WHERE (excused = true);
CREATE INDEX idx_students_group_id ON users.students_legacy USING btree (group_id);
CREATE INDEX idx_students_guardian_email ON users.students_legacy USING btree (guardian_email);
CREATE INDEX idx_students_guardian_phone ON users.students_legacy USING btree (guardian_phone);
CREATE INDEX idx_students_person_id ON users.students_legacy USING btree (person_id);
CREATE INDEX idx_students_photo_consent ON users.students_legacy USING btree (tenant_id) WHERE (photo_consent_given_at IS NOT NULL);
CREATE INDEX idx_students_pickup_status ON users.students_legacy USING btree (pickup_status);
CREATE INDEX idx_students_school_class ON users.students_legacy USING btree (school_class);
CREATE INDEX idx_students_school_class_lower ON users.students_legacy USING btree (lower(school_class));
COMMENT ON INDEX users.idx_students_school_class_lower IS 'Case-insensitive index for school class filtering';
CREATE INDEX idx_students_sick ON users.students_legacy USING btree (sick) WHERE (sick = true);
CREATE INDEX idx_students_tenant ON users.students_legacy USING btree (tenant_id);
CREATE INDEX idx_students_tenant_class ON users.students_legacy USING btree (tenant_id, school_class);
CREATE INDEX idx_students_tenant_enrolled_from_pending ON users.students_legacy USING btree (tenant_id, enrolled_from) WHERE (status = 'pending'::text);
CREATE INDEX idx_students_tenant_enrolled_until_active ON users.students_legacy USING btree (tenant_id, enrolled_until) WHERE (status = 'active'::text);
CREATE INDEX idx_students_tenant_enrolled_until_ended ON users.students_legacy USING btree (tenant_id, enrolled_until) WHERE ((enrolled_until IS NOT NULL) AND (status <> 'alumnus'::text));
CREATE UNIQUE INDEX idx_students_tenant_person ON users.students_legacy USING btree (tenant_id, person_id);
CREATE UNIQUE INDEX idx_students_tenant_pk ON users.students_legacy USING btree (tenant_id, id);
CREATE INDEX idx_students_tenant_school_class_normalized ON users.students_legacy USING btree (tenant_id, lower(btrim(school_class)));
CREATE INDEX idx_students_tenant_status ON users.students_legacy USING btree (tenant_id, status);
CREATE TRIGGER update_students_updated_at BEFORE UPDATE ON users.students_legacy FOR EACH ROW EXECUTE FUNCTION public.update_modified_column();
ALTER TABLE ONLY users.students_legacy
    ADD CONSTRAINT fk_students_group_tenant FOREIGN KEY (tenant_id, group_id) REFERENCES education.groups(tenant_id, id) ON DELETE SET NULL;
ALTER TABLE ONLY users.students_legacy
    ADD CONSTRAINT fk_students_person FOREIGN KEY (person_id) REFERENCES users.persons(id) ON DELETE CASCADE;
ALTER TABLE ONLY users.students_legacy
    ADD CONSTRAINT students_photo_consent_given_by_fkey FOREIGN KEY (photo_consent_given_by) REFERENCES auth.accounts(id) ON DELETE SET NULL;
ALTER TABLE ONLY users.students_legacy
    ADD CONSTRAINT students_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES platform.schools(id);
ALTER TABLE users.students_legacy ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_users_students ON users.students_legacy USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::bigint)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::bigint));
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE users.students_legacy TO phoenix_tenant;
GRANT ALL ON TABLE users.students_legacy TO phoenix_admin;
GRANT USAGE ON SEQUENCE users.students_id_seq TO phoenix_tenant;
GRANT ALL ON SEQUENCE users.students_id_seq TO phoenix_admin;
