package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentAccidentReportsVersion     = "1.15.716"
	studentAccidentReportsDescription = "Unfallanzeige: Unfall am Kind erfassen, als PDF ablegen und unterschreiben, Rechte student_accidents:read und :manage (#4771)"

	studentAccidentsReadPermissionName   = "student_accidents:read"
	studentAccidentsManagePermissionName = "student_accidents:manage"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentAccidentReportsVersion,
		Description: studentAccidentReportsDescription,
		DependsOn: []string{
			parentMessageLeadershipVersion, // the last migration before this one
			studentDocumentsVersion,        // the document table this extends
			signaturesVersion,              // the signature store a digital signature lands in
		},
	})
	Migrations.MustRegister(studentAccidentReportsUp, studentAccidentReportsDown)
}

// studentAccidentReportsUpSQL keeps an accident report (Unfallanzeige) as a
// child document of the new category unfallanzeige (#4771). A report is a
// document row, so it lives, cascades and leaves with the child and the
// school exactly like every other document; no table of its own needs a
// place in the school erasure.
//
//   - accident_report holds what the team recorded, in the fields of the
//     official form for children in day care, pupils and students. Only an
//     unfallanzeige row carries it. The PDF is rendered from it on demand, so
//     a report without a paper copy stores no file: filename_stored may be
//     NULL, but only on a row with a report.
//   - A report is open until it is signed: digitally with the shared signature
//     store (accident_signature_id) or by uploading the signed paper copy,
//     which becomes the row's file. accident_signed_at marks both. A trigger
//     refuses every later change of what was signed, so a signed report stays
//     as it is; a correction is a new row naming its predecessor in
//     accident_supersedes_id, and each report has at most one live successor.
//   - The signature and the corrected report are named without foreign keys.
//     Care Plan sets both only from rows it read in the same tenant
//     transaction, and they leave only together with the child or the
//     school, which removes the report as well. A foreign key between two
//     erasure tables would change the school erasure's edge fingerprint and
//     need a reinstall of its functions for no extra protection.
const studentAccidentReportsUpSQL = `
		ALTER TABLE users.student_documents
			DROP CONSTRAINT IF EXISTS student_documents_category_check;
		ALTER TABLE users.student_documents
			ADD CONSTRAINT student_documents_category_check CHECK (category IN (
				'betreuungsvertrag', 'abholvollmacht', 'schwimmerlaubnis',
				'attest', 'impfnachweis', 'medikamentenplan',
				'sorgerecht', 'unfallanzeige', 'sonstiges'
			));

		ALTER TABLE users.student_documents
			ALTER COLUMN filename_stored DROP NOT NULL,
			ADD COLUMN IF NOT EXISTS accident_report JSONB,
			ADD COLUMN IF NOT EXISTS accident_supersedes_id BIGINT,
			ADD COLUMN IF NOT EXISTS accident_signature_id BIGINT,
			ADD COLUMN IF NOT EXISTS accident_signed_at TIMESTAMPTZ,
			ADD COLUMN IF NOT EXISTS accident_signed_by BIGINT;

		ALTER TABLE users.student_documents
			ADD CONSTRAINT chk_student_documents_accident_report CHECK (
				accident_report IS NULL
				OR (category = 'unfallanzeige' AND jsonb_typeof(accident_report) = 'object')
			),
			ADD CONSTRAINT chk_student_documents_file_or_report CHECK (
				filename_stored IS NOT NULL OR accident_report IS NOT NULL
			),
			ADD CONSTRAINT chk_student_documents_accident_lifecycle CHECK (
				(accident_supersedes_id IS NULL AND accident_signed_at IS NULL AND accident_signature_id IS NULL AND accident_signed_by IS NULL)
				OR accident_report IS NOT NULL
			),
			ADD CONSTRAINT chk_student_documents_accident_signed CHECK (
				(accident_signed_at IS NULL) = (accident_signed_by IS NULL)
				AND (accident_signature_id IS NULL OR accident_signed_at IS NOT NULL)
			);

		CREATE UNIQUE INDEX IF NOT EXISTS uq_student_documents_accident_successor
			ON users.student_documents (tenant_id, accident_supersedes_id)
			WHERE accident_supersedes_id IS NOT NULL AND deleted_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_student_documents_accident_signature
			ON users.student_documents (tenant_id, accident_signature_id)
			WHERE accident_signature_id IS NOT NULL;

		CREATE OR REPLACE FUNCTION users.keep_signed_accident_report() RETURNS trigger
		LANGUAGE plpgsql SET search_path = pg_catalog, pg_temp AS $$
		BEGIN
			IF OLD.accident_signed_at IS NOT NULL AND (
				NEW.accident_report IS DISTINCT FROM OLD.accident_report
				OR NEW.accident_signed_at IS DISTINCT FROM OLD.accident_signed_at
				OR NEW.accident_signed_by IS DISTINCT FROM OLD.accident_signed_by
				OR NEW.accident_signature_id IS DISTINCT FROM OLD.accident_signature_id
				OR NEW.accident_supersedes_id IS DISTINCT FROM OLD.accident_supersedes_id
				OR NEW.filename_stored IS DISTINCT FROM OLD.filename_stored
				OR NEW.deleted_at IS DISTINCT FROM OLD.deleted_at
			) THEN
				RAISE EXCEPTION 'signed accident report % cannot be changed', OLD.id
					USING ERRCODE = 'check_violation';
			END IF;
			RETURN NEW;
		END;
		$$;
		REVOKE ALL ON FUNCTION users.keep_signed_accident_report() FROM PUBLIC;

		DROP TRIGGER IF EXISTS keep_signed_accident_report ON users.student_documents;
		CREATE TRIGGER keep_signed_accident_report
			BEFORE UPDATE ON users.student_documents
			FOR EACH ROW EXECUTE FUNCTION users.keep_signed_accident_report();

		ALTER TABLE audit.signatures
			DROP CONSTRAINT IF EXISTS signatures_purpose_check;
		ALTER TABLE audit.signatures
			ADD CONSTRAINT signatures_purpose_check
			CHECK (purpose IN ('declaration','enrollment','pickup_permission','accident_report'));

		COMMENT ON COLUMN users.student_documents.accident_report IS
			'Unfallanzeige (#4771): the recorded accident in the fields of the official form; the PDF is rendered from it.';
`

const studentAccidentReportsDownSQL = `
		ALTER TABLE audit.signatures
			DROP CONSTRAINT IF EXISTS signatures_purpose_check;
		ALTER TABLE audit.signatures
			ADD CONSTRAINT signatures_purpose_check
			CHECK (purpose IN ('declaration','enrollment','pickup_permission'));

		DROP TRIGGER IF EXISTS keep_signed_accident_report ON users.student_documents;
		DROP FUNCTION IF EXISTS users.keep_signed_accident_report();
		DROP INDEX IF EXISTS users.idx_student_documents_accident_signature;
		DROP INDEX IF EXISTS users.uq_student_documents_accident_successor;

		ALTER TABLE users.student_documents
			DROP CONSTRAINT IF EXISTS chk_student_documents_accident_signed,
			DROP CONSTRAINT IF EXISTS chk_student_documents_accident_lifecycle,
			DROP CONSTRAINT IF EXISTS chk_student_documents_file_or_report,
			DROP CONSTRAINT IF EXISTS chk_student_documents_accident_report,
			DROP COLUMN IF EXISTS accident_signed_by,
			DROP COLUMN IF EXISTS accident_signed_at,
			DROP COLUMN IF EXISTS accident_signature_id,
			DROP COLUMN IF EXISTS accident_supersedes_id,
			DROP COLUMN IF EXISTS accident_report,
			ALTER COLUMN filename_stored SET NOT NULL;

		ALTER TABLE users.student_documents
			DROP CONSTRAINT IF EXISTS student_documents_category_check;
		ALTER TABLE users.student_documents
			ADD CONSTRAINT student_documents_category_check CHECK (category IN (
				'betreuungsvertrag', 'abholvollmacht', 'schwimmerlaubnis',
				'attest', 'impfnachweis', 'medikamentenplan',
				'sorgerecht', 'sonstiges'
			));
`

func studentAccidentReportsUp(ctx context.Context, db *bun.DB) error {
	if err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, studentAccidentReportsUpSQL); err != nil {
			return fmt.Errorf("add student accident reports: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        studentAccidentsReadPermissionName,
		Description: "Unfallanzeigen eines Kindes lesen und herunterladen (Gesundheitsdaten nach Art. 9 DSGVO)",
		Resource:    "student_accidents",
		Action:      "read",
	}, "admin"); err != nil {
		return err
	}
	return grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        studentAccidentsManagePermissionName,
		Description: "Unfallanzeigen eines Kindes erfassen, ändern, unterschreiben und korrigieren",
		Resource:    "student_accidents",
		Action:      "manage",
	}, "admin")
}

// studentAccidentReportsDown refuses while an accident report exists:
// dropping the category would erase recorded accidents and their signatures.
func studentAccidentReportsDown(ctx context.Context, db *bun.DB) error {
	var reports int
	if err := db.NewRaw(`SELECT COUNT(*) FROM users.student_documents WHERE category = 'unfallanzeige'`).
		Scan(ctx, &reports); err != nil {
		return fmt.Errorf("count accident reports: %w", err)
	}
	if reports > 0 {
		return errors.New("student accident reports: refusing rollback while accident reports exist")
	}
	var signatures int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.signatures WHERE purpose = 'accident_report'`).
		Scan(ctx, &signatures); err != nil {
		return fmt.Errorf("count accident report signatures: %w", err)
	}
	if signatures > 0 {
		return errors.New("student accident reports: refusing rollback while accident report signatures exist")
	}
	if err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, studentAccidentReportsDownSQL); err != nil {
			return fmt.Errorf("remove student accident reports: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := dropPermission(ctx, db, studentAccidentsManagePermissionName); err != nil {
		return err
	}
	return dropPermission(ctx, db, studentAccidentsReadPermissionName)
}
