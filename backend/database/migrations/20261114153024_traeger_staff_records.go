package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerStaffRecordsVersion = "20261114153024"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerStaffRecordsVersion,
		Description: "Personalakte des Trägers über alle Schulen mit Freigabe „Personal beim Träger“ (#4705, ADR 0089)",
		DependsOn: []string{
			traegerPortalVersion,         // the Träger the release and the records belong to (1.15.514)
			bankDataPlaintextDropVersion, // staff bank data only sealed, the shape the records take over (1.15.900)
		},
	})
	Migrations.MustRegister(traegerStaffRecordsUp, traegerStaffRecordsDown)
}

// traegerStaffRecordsUpSQL creates the Personalstamm of ADR 0089 (#4705).
//
//   - workforce.staff_releases is the release „Personal beim Träger“
//     the moto team grants (ADR 0089 section 2). It keeps which of the two
//     statements the Träger confirmed: an agreement with the works council
//     or employee representation, or no representation. Only the operator
//     writes it, in the administrative transaction with its operator audit
//     entry; the Träger reads its own row.
//   - workforce.staff_records is one person the Träger employs, over all its
//     schools: names, birthday, the Träger's personnel number (unique per
//     Träger, may be empty) and the bank and tax values sealed with the bank
//     ring. A record exists only while its Träger holds the release: the
//     foreign key to the release refuses a withdrawal while records exist,
//     and an erased Träger takes release and records with it.
//   - workforce.staff_record_links joins a record to the employment
//     (users.staff_school_memberships) of one school, its Einsatzort. Like
//     the other Träger tables it names the school by a plain id without a
//     foreign key into the school's rows (ADR 0084, care_contract), so the
//     school erasure plan stays untouched; a link of an erased employment is
//     skipped on read. The school reads its own links, so it knows which of
//     its staff the Träger keeps.
//   - workforce.staff_record_changes is the trail of a record: the values
//     not chosen when linking, with their school (ADR 0089 section 5), and
//     every later change. Bank and tax values stay sealed there.
const traegerStaffRecordsUpSQL = `
CREATE SCHEMA IF NOT EXISTS workforce;
GRANT USAGE ON SCHEMA workforce TO phoenix_tenant, phoenix_admin;

CREATE TABLE workforce.staff_releases (
	organization_id BIGINT PRIMARY KEY REFERENCES platform.organizations(id) ON DELETE CASCADE,
	representation VARCHAR(16) NOT NULL CONSTRAINT chk_workforce_staff_releases_representation CHECK (representation IN ('agreement', 'none')),
	released_by_operator_id BIGINT NOT NULL,
	released_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE workforce.staff_releases IS
	'The release „Personal beim Träger“ of ADR 0089 section 2 (#4705): granted by the moto team after the Träger confirmed an agreement with its employee representation (agreement) or that it has none (none).';

CREATE TABLE workforce.staff_records (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	first_name VARCHAR(255) NOT NULL,
	last_name VARCHAR(255) NOT NULL,
	birthday DATE,
	personnel_number VARCHAR(9),
	iban_sealed BYTEA,
	iban_masked VARCHAR(48),
	tax_id_sealed BYTEA,
	social_security_number_sealed BYTEA,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT uq_workforce_staff_records_organization_id UNIQUE (organization_id, id),
	CONSTRAINT fk_workforce_staff_records_release FOREIGN KEY (organization_id)
		REFERENCES workforce.staff_releases (organization_id),
	CONSTRAINT chk_workforce_staff_records_names CHECK (btrim(first_name) <> '' AND btrim(last_name) <> ''),
	CONSTRAINT chk_workforce_staff_records_personnel_number CHECK (personnel_number ~ '^[0-9]{1,9}$')
);
CREATE UNIQUE INDEX uq_workforce_staff_records_personnel_number ON workforce.staff_records (organization_id, personnel_number)
	WHERE personnel_number IS NOT NULL;
COMMENT ON TABLE workforce.staff_records IS
	'Personalstamm (ADR 0089 section 1, #4705): one person the Träger employs over all its schools, with the Träger''s personnel number and the sealed bank and tax values.';

CREATE TABLE workforce.staff_record_links (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	staff_record_id BIGINT NOT NULL,
	school_id BIGINT NOT NULL,
	membership_id BIGINT NOT NULL,
	linked_by_account_id BIGINT NOT NULL,
	linked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT fk_workforce_staff_record_links_record FOREIGN KEY (organization_id, staff_record_id)
		REFERENCES workforce.staff_records (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_workforce_staff_record_links_membership UNIQUE (school_id, membership_id),
	CONSTRAINT uq_workforce_staff_record_links_school UNIQUE (staff_record_id, school_id)
);
CREATE INDEX idx_workforce_staff_record_links_record ON workforce.staff_record_links (organization_id, staff_record_id);
COMMENT ON TABLE workforce.staff_record_links IS
	'The Einsatzorte of a Personalstamm (#4705): one employment of one school per row, by plain id like the other Träger tables.';

CREATE TABLE workforce.staff_record_changes (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	staff_record_id BIGINT NOT NULL,
	kind VARCHAR(16) NOT NULL CONSTRAINT chk_workforce_staff_record_changes_kind CHECK (kind IN ('linked', 'not_chosen', 'changed')),
	field VARCHAR(32) NOT NULL CONSTRAINT chk_workforce_staff_record_changes_field CHECK (field IN (
		'school', 'first_name', 'last_name', 'birthday', 'personnel_number', 'iban', 'tax_id', 'social_security_number')),
	value_text TEXT,
	value_sealed BYTEA,
	value_masked VARCHAR(48),
	school_id BIGINT,
	account_id BIGINT NOT NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT fk_workforce_staff_record_changes_record FOREIGN KEY (organization_id, staff_record_id)
		REFERENCES workforce.staff_records (organization_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_workforce_staff_record_changes_record ON workforce.staff_record_changes (organization_id, staff_record_id, occurred_at DESC, id DESC);
COMMENT ON TABLE workforce.staff_record_changes IS
	'Trail of a Personalstamm (#4705): values not chosen when linking with their school (ADR 0089 section 5) and later changes; bank and tax values stay sealed.';

CREATE TRIGGER trg_workforce_staff_records_updated_at BEFORE UPDATE ON workforce.staff_records
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

ALTER TABLE workforce.staff_releases ENABLE ROW LEVEL SECURITY;
ALTER TABLE workforce.staff_releases FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_reads_workforce_staff_releases ON workforce.staff_releases FOR SELECT
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

ALTER TABLE workforce.staff_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE workforce.staff_records FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_workforce_staff_records ON workforce.staff_records FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

ALTER TABLE workforce.staff_record_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE workforce.staff_record_links FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_workforce_staff_record_links ON workforce.staff_record_links FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
-- A school reads only which of its own employments the Träger keeps
-- (ADR 0089 section 3), never another school's links or a record.
CREATE POLICY school_reads_workforce_staff_record_links ON workforce.staff_record_links FOR SELECT
	USING (school_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

ALTER TABLE workforce.staff_record_changes ENABLE ROW LEVEL SECURITY;
ALTER TABLE workforce.staff_record_changes FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_workforce_staff_record_changes ON workforce.staff_record_changes FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

-- The Träger's personnel number is the one of every Einsatzort (ADR 0232
-- section 5): a school's profile of a linked employment takes no other
-- number, whatever path writes it (staff admin, import, membership). The
-- function reads the record the school's role cannot see, so it runs as its
-- owner and answers only whether the number differs.
CREATE FUNCTION workforce.refuse_number_kept_by_traeger() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
BEGIN
	IF EXISTS (
		SELECT FROM workforce.staff_record_links AS link
		JOIN workforce.staff_records AS record ON record.organization_id = link.organization_id AND record.id = link.staff_record_id
		WHERE link.school_id = NEW.tenant_id AND link.membership_id = NEW.membership_id
		  AND record.personnel_number IS DISTINCT FROM NEW.personnel_number
	) THEN
		RAISE EXCEPTION 'the Träger keeps the personnel number of this employment' USING ERRCODE = '23514',
			CONSTRAINT = 'chk_staff_personnel_number_kept_by_traeger';
	END IF;
	RETURN NEW;
END;
$$;
REVOKE ALL ON FUNCTION workforce.refuse_number_kept_by_traeger() FROM PUBLIC;
CREATE TRIGGER trg_staff_personnel_number_kept_by_traeger
	BEFORE UPDATE OF personnel_number ON users.staff_employment_profiles
	FOR EACH ROW WHEN (OLD.personnel_number IS DISTINCT FROM NEW.personnel_number)
	EXECUTE FUNCTION workforce.refuse_number_kept_by_traeger();

REVOKE ALL ON workforce.staff_releases, workforce.staff_records, workforce.staff_record_links, workforce.staff_record_changes FROM PUBLIC;
GRANT SELECT ON workforce.staff_releases TO phoenix_tenant;
GRANT SELECT, INSERT, UPDATE, DELETE ON workforce.staff_records, workforce.staff_record_links TO phoenix_tenant;
-- The trail is append-only for the request role.
GRANT SELECT, INSERT ON workforce.staff_record_changes TO phoenix_tenant;
GRANT USAGE ON SEQUENCE workforce.staff_records_id_seq, workforce.staff_record_links_id_seq, workforce.staff_record_changes_id_seq TO phoenix_tenant;
GRANT ALL ON workforce.staff_releases, workforce.staff_records, workforce.staff_record_links, workforce.staff_record_changes TO phoenix_admin;
GRANT USAGE ON SEQUENCE workforce.staff_records_id_seq, workforce.staff_record_links_id_seq, workforce.staff_record_changes_id_seq TO phoenix_admin;
`

const traegerStaffRecordsDownSQL = `
DROP TRIGGER IF EXISTS trg_staff_personnel_number_kept_by_traeger ON users.staff_employment_profiles;
DROP FUNCTION IF EXISTS workforce.refuse_number_kept_by_traeger();
DROP TABLE IF EXISTS workforce.staff_record_changes;
DROP TABLE IF EXISTS workforce.staff_record_links;
DROP TABLE IF EXISTS workforce.staff_records;
DROP TABLE IF EXISTS workforce.staff_releases;
DROP SCHEMA IF EXISTS workforce;
`

func traegerStaffRecordsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, traegerStaffRecordsUpSQL); err != nil {
			return fmt.Errorf("create traeger staff records: %w", err)
		}
		return nil
	})
}

func traegerStaffRecordsDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, traegerStaffRecordsDownSQL); err != nil {
			return fmt.Errorf("drop traeger staff records: %w", err)
		}
		return nil
	})
}
