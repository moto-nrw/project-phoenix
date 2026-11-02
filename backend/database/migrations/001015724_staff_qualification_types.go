package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffQualificationTypesVersion     = "1.15.724"
	staffQualificationTypesDescription = "Nachweisarten der Schule, Zuordnung der Qualifikationen und Erinnerungsstand (#4808)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffQualificationTypesVersion,
		Description: staffQualificationTypesDescription,
		DependsOn: []string{
			"1.15.723", // the last migration before this one; extends the 1.15.721 erasure functions
		},
	})
	Migrations.MustRegister(staffQualificationTypesUp, staffQualificationTypesDown)
}

// staffQualificationTypesUpSQL adds the Nachweisarten (#4808).
//
//   - users.staff_qualification_types holds the kinds of proof a school
//     tracks: the standard list (standard_key set) and the school's own
//     kinds. A kind is archived, never deleted, because qualifications keep
//     pointing at it. validity_months suggests the expiry date,
//     reminder_lead_weeks says how early the reminder goes, mandatory says
//     every staff member needs one.
//   - users.staff_qualifications.type_id assigns a qualification to a kind;
//     NULL is "Sonstiges" with the free-text name. Existing rows stay NULL:
//     a free-text name equal to a standard kind is not reinterpreted.
//   - lead_reminded_for / expiry_reminded_for hold the expiry date the
//     reminder before and on the expiry day went out for. A changed expiry
//     date arms both reminders again.
//
// Only kind, dates and status are stored, never the content of a
// certificate (§ 72a SGB VIII).
const staffQualificationTypesUpSQL = `
		CREATE TABLE IF NOT EXISTS users.staff_qualification_types (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			standard_key        TEXT,
			name                TEXT NOT NULL,
			validity_months     INTEGER,
			reminder_lead_weeks INTEGER NOT NULL DEFAULT 8,
			mandatory           BOOLEAN NOT NULL DEFAULT FALSE,
			sort_order          INTEGER NOT NULL DEFAULT 0,
			archived_at         TIMESTAMPTZ,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_staff_qualification_types_name CHECK (btrim(name) <> '' AND char_length(name) <= 100),
			CONSTRAINT chk_staff_qualification_types_standard_key
				CHECK (standard_key IS NULL OR standard_key IN ('first_aid_child', 'rescue_ability', 'extended_police_certificate')),
			CONSTRAINT chk_staff_qualification_types_validity CHECK (validity_months IS NULL OR validity_months BETWEEN 1 AND 240),
			CONSTRAINT chk_staff_qualification_types_lead CHECK (reminder_lead_weeks BETWEEN 0 AND 52),
			CONSTRAINT uq_staff_qualification_types_tenant_id UNIQUE (tenant_id, id)
		);
		CREATE UNIQUE INDEX IF NOT EXISTS uq_staff_qualification_types_standard
			ON users.staff_qualification_types (tenant_id, standard_key) WHERE standard_key IS NOT NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS uq_staff_qualification_types_live_name
			ON users.staff_qualification_types (tenant_id, lower(btrim(name))) WHERE archived_at IS NULL;
		COMMENT ON TABLE users.staff_qualification_types IS
			'Nachweisarten der Schule (#4808): Standardliste und eigene Arten mit Gültigkeitsdauer, Erinnerungsvorlauf und Pflicht. Archiviert statt gelöscht.';

		DROP TRIGGER IF EXISTS update_staff_qualification_types_updated_at ON users.staff_qualification_types;
		CREATE TRIGGER update_staff_qualification_types_updated_at
		BEFORE UPDATE ON users.staff_qualification_types
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		REVOKE ALL ON users.staff_qualification_types FROM PUBLIC, phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE ON users.staff_qualification_types TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.staff_qualification_types_id_seq TO phoenix_tenant;

		ALTER TABLE users.staff_qualifications
			ADD COLUMN IF NOT EXISTS type_id BIGINT,
			ADD COLUMN IF NOT EXISTS lead_reminded_for DATE,
			ADD COLUMN IF NOT EXISTS expiry_reminded_for DATE,
			ADD CONSTRAINT fk_staff_qualifications_type
				FOREIGN KEY (tenant_id, type_id)
				REFERENCES users.staff_qualification_types (tenant_id, id);
		CREATE INDEX IF NOT EXISTS idx_staff_qualifications_type
			ON users.staff_qualifications (tenant_id, type_id) WHERE type_id IS NOT NULL;
`

const staffQualificationTypesDownSQL = `
		DROP INDEX IF EXISTS users.idx_staff_qualifications_type;
		ALTER TABLE users.staff_qualifications
			DROP CONSTRAINT IF EXISTS fk_staff_qualifications_type,
			DROP COLUMN IF EXISTS type_id,
			DROP COLUMN IF EXISTS lead_reminded_for,
			DROP COLUMN IF EXISTS expiry_reminded_for;
		DROP TABLE IF EXISTS users.staff_qualification_types;
`

func staffQualificationTypesUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, staffQualificationTypesUpSQL); err != nil {
			return fmt.Errorf("create the staff qualification types: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, "users.staff_qualification_types"); err != nil {
			return err
		}
		// The school erasure deletes the kinds with the school, after the
		// qualifications that point at them.
		for _, step := range []func(context.Context, bun.Tx) error{
			installStaffQualificationTypeErasurePlan, installStaffQualificationTypeErasurePreflight, installStaffQualificationTypeErasureResidual,
			installStaffQualificationTypeErasureDelete, installStaffQualificationTypeErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("install the staff qualification type school erasure: %w", err)
			}
		}
		return nil
	})
}

// staffQualificationTypesDown restores the 1.15.721 erasure functions before
// it drops the table, so the installed plan never names a missing table.
func staffQualificationTypesDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []func(context.Context, bun.Tx) error{
			installSchoolVacationErasurePlan, installSchoolVacationErasurePreflight, installSchoolVacationErasureResidual,
			installSchoolVacationErasureDelete, installSchoolVacationErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("restore the school vacation school erasure: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, staffQualificationTypesDownSQL); err != nil {
			return fmt.Errorf("drop the staff qualification types: %w", err)
		}
		return nil
	})
}
