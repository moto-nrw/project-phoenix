-- Historical test-only base table at migration 1.15.384, before Cutover #2714.
-- Captured from a disposable local migration template with PostgreSQL 17 pg_dump
-- of enrollment.request_child_offerings_legacy, renamed back to its pre-cutover
-- name with the overlap exclusion the cutover dropped. No data included.
-- Session settings and psql directives removed; constraints, RLS and grants retained.
CREATE TABLE enrollment.request_child_offerings (
    id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    request_child_id bigint NOT NULL,
    care_offering_id bigint NOT NULL,
    selected_days jsonb,
    notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    manual_selected_days jsonb,
    automatic_selected_days jsonb,
    valid_from date,
    valid_until date,
    CONSTRAINT request_child_offerings_nonempty_validity CHECK (((valid_from IS NULL) OR (valid_until IS NULL) OR (valid_from < valid_until))),
    CONSTRAINT request_child_offerings_non_overlapping_validity EXCLUDE USING gist (
        request_child_id WITH =, care_offering_id WITH =,
        daterange(COALESCE(valid_from, '-infinity'::date), COALESCE(valid_until, 'infinity'::date), '[)') WITH &&
    )
);
ALTER TABLE ONLY enrollment.request_child_offerings FORCE ROW LEVEL SECURITY;
CREATE SEQUENCE enrollment.request_child_offerings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE enrollment.request_child_offerings_id_seq OWNED BY enrollment.request_child_offerings.id;
ALTER TABLE ONLY enrollment.request_child_offerings ALTER COLUMN id SET DEFAULT nextval('enrollment.request_child_offerings_id_seq'::regclass);
ALTER TABLE ONLY enrollment.request_child_offerings
    ADD CONSTRAINT request_child_offerings_pkey PRIMARY KEY (id);
CREATE INDEX idx_request_child_offerings_active_validity ON enrollment.request_child_offerings USING btree (care_offering_id, valid_from, valid_until);
CREATE INDEX idx_request_child_offerings_offering ON enrollment.request_child_offerings USING btree (care_offering_id);
ALTER TABLE ONLY enrollment.request_child_offerings
    ADD CONSTRAINT request_child_offerings_care_offering_id_fkey FOREIGN KEY (care_offering_id) REFERENCES enrollment.care_offerings(id) ON DELETE RESTRICT;
ALTER TABLE ONLY enrollment.request_child_offerings
    ADD CONSTRAINT request_child_offerings_request_child_id_fkey FOREIGN KEY (request_child_id) REFERENCES enrollment.request_children(id) ON DELETE CASCADE;
ALTER TABLE ONLY enrollment.request_child_offerings
    ADD CONSTRAINT request_child_offerings_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES platform.schools(id) ON DELETE CASCADE;
ALTER TABLE enrollment.request_child_offerings ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_enrollment_request_child_offerings ON enrollment.request_child_offerings USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::bigint)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::bigint));
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE enrollment.request_child_offerings TO phoenix_tenant;
GRANT ALL ON TABLE enrollment.request_child_offerings TO phoenix_admin;
GRANT USAGE ON SEQUENCE enrollment.request_child_offerings_id_seq TO phoenix_tenant;
GRANT ALL ON SEQUENCE enrollment.request_child_offerings_id_seq TO phoenix_admin;
