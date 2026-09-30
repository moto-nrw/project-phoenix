-- Historical test-only archive schema at migration 1.15.426.
-- Captured from a disposable local migration database with PostgreSQL pg_dump,
-- schema-only, users.staff_legacy and its owned sequence. No data included.
-- Session settings, owners and psql directives removed; constraints, RLS and grants retained.
CREATE TABLE users.staff_legacy (
    id bigint NOT NULL,
    person_id bigint NOT NULL,
    staff_notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    employment_type character varying(20),
    tenant_id bigint NOT NULL,
    work_time_model_id bigint,
    rotation_anchor_date date,
    deleted_at timestamp with time zone,
    personnel_number text,
    birthday_display_opt_out boolean DEFAULT false NOT NULL,
    CONSTRAINT chk_staff_employment_type CHECK (((employment_type IS NULL) OR ((employment_type)::text = ANY ((ARRAY['full_time'::character varying, 'part_time'::character varying, 'minijob'::character varying])::text[])))),
    CONSTRAINT chk_staff_tenant_id_not_null CHECK ((tenant_id IS NOT NULL))
);
ALTER TABLE ONLY users.staff_legacy FORCE ROW LEVEL SECURITY;
COMMENT ON TABLE users.staff_legacy IS 'Rollback-only archive of the pre-Cutover users.staff (#2753). users.staff_school_memberships and users.staff_employment_profiles are authoritative; nothing writes this table.';
CREATE SEQUENCE users.staff_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE users.staff_id_seq OWNED BY users.staff_legacy.id;
ALTER TABLE ONLY users.staff_legacy ALTER COLUMN id SET DEFAULT nextval('users.staff_id_seq'::regclass);
ALTER TABLE ONLY users.staff_legacy
    ADD CONSTRAINT staff_pkey PRIMARY KEY (id);
CREATE UNIQUE INDEX idx_staff_legacy_tenant_person ON users.staff_legacy USING btree (tenant_id, person_id) WHERE (deleted_at IS NULL);
CREATE INDEX idx_staff_person_id ON users.staff_legacy USING btree (person_id);
CREATE INDEX idx_staff_tenant ON users.staff_legacy USING btree (tenant_id);
CREATE UNIQUE INDEX idx_staff_tenant_pk ON users.staff_legacy USING btree (tenant_id, id);
CREATE UNIQUE INDEX uq_staff_legacy_tenant_personnel_number ON users.staff_legacy USING btree (tenant_id, personnel_number) WHERE ((personnel_number IS NOT NULL) AND (deleted_at IS NULL));
CREATE TRIGGER update_staff_updated_at BEFORE UPDATE ON users.staff_legacy FOR EACH ROW EXECUTE FUNCTION public.update_modified_column();
ALTER TABLE ONLY users.staff_legacy
    ADD CONSTRAINT fk_staff_person_tenant FOREIGN KEY (tenant_id, person_id) REFERENCES users.persons(tenant_id, id) ON DELETE CASCADE;
ALTER TABLE ONLY users.staff_legacy
    ADD CONSTRAINT staff_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES platform.schools(id);
ALTER TABLE users.staff_legacy ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_users_staff ON users.staff_legacy USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::bigint)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::bigint));
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE users.staff_legacy TO phoenix_tenant;
GRANT ALL ON TABLE users.staff_legacy TO phoenix_admin;
GRANT USAGE ON SEQUENCE users.staff_id_seq TO phoenix_tenant;
GRANT ALL ON SEQUENCE users.staff_id_seq TO phoenix_admin;
