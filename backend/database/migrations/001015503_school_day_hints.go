package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	schoolDayHintsVersion     = "1.15.503"
	schoolDayHintsDescription = "Tageshinweis der Lehrkraft: Tagesnotiz aus moto schule für die OGS (#4795)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     schoolDayHintsVersion,
		Description: schoolDayHintsDescription,
		DependsOn: []string{
			schoolPortalClassTripsVersion, // the last migration before this one (#4871, stack parent)
			studentNotesVersion,           // the table the hints are rows of
			lehrkraftRoleVersion,          // the role the write permission goes to
		},
	})
	Migrations.MustRegister(schoolDayHintsUp, schoolDayHintsDown)
}

// schoolDayHintsUpSQL lets users.student_notes carry the Lehrkraft's hint for
// the OGS for one day (#4795). The hint is a dated note like every other
// Tagesnotiz, with origin 'school'; it needs no table of its own, and it is
// kept and removed with the child like every note.
//
//   - A hint for one child is one row. A hint for the whole class is one row
//     per child of the class that day, each carrying the class in
//     school_class, so every OGS view reads it at the child without knowing
//     classes, and the hint goes with each child's note card.
//   - One live hint per author, child, day and target: a second write of the
//     same Lehrkraft for the same child (or the same class) and day replaces
//     the wording instead of adding a row. The unique index is the target of
//     that upsert.
//   - A school hint is always a dated journal entry readable by all staff,
//     without a group reference and with at most 500 characters.
const schoolDayHintsUpSQL = `
		ALTER TABLE users.student_notes
			ADD COLUMN IF NOT EXISTS school_class TEXT;

		COMMENT ON COLUMN users.student_notes.school_class IS
			'Klasse eines Tageshinweises der Lehrkraft für die ganze Klasse (origin = school); NULL bei einem Hinweis für ein Kind.';
		COMMENT ON COLUMN users.student_notes.origin IS
			'staff = von einer Person geschrieben; master_data = aus den früheren Betreuernotizen oder einem Import übernommen, daher ohne Autor; school = Tageshinweis der Lehrkraft aus moto schule.';

		ALTER TABLE users.student_notes
			DROP CONSTRAINT IF EXISTS chk_student_notes_origin;
		ALTER TABLE users.student_notes
			ADD CONSTRAINT chk_student_notes_origin CHECK (origin IN ('staff', 'master_data', 'school'));

		ALTER TABLE users.student_notes
			DROP CONSTRAINT IF EXISTS chk_student_notes_school_hint;
		ALTER TABLE users.student_notes
			ADD CONSTRAINT chk_student_notes_school_hint CHECK (
				origin <> 'school' OR (
					kind = 'journal'
					AND visibility = 'all_staff'
					AND subject_date IS NOT NULL
					AND activity_group_id IS NULL
					AND education_group_id IS NULL
					AND char_length(body) <= 500
				)
			);

		ALTER TABLE users.student_notes
			DROP CONSTRAINT IF EXISTS chk_student_notes_school_class;
		ALTER TABLE users.student_notes
			ADD CONSTRAINT chk_student_notes_school_class CHECK (
				school_class IS NULL OR (origin = 'school' AND btrim(school_class) <> '')
			);

		CREATE UNIQUE INDEX IF NOT EXISTS uniq_student_notes_school_day_hint
			ON users.student_notes (
				tenant_id, student_id, subject_date, author_account_id,
				(LOWER(BTRIM(COALESCE(school_class, ''))))
			)
			WHERE origin = 'school' AND deleted_at IS NULL;

		-- The OGS views read the hints of one day for many children at once.
		CREATE INDEX IF NOT EXISTS idx_student_notes_school_day
			ON users.student_notes (tenant_id, subject_date, student_id)
			WHERE origin = 'school' AND deleted_at IS NULL;
`

const schoolDayHintsDownSQL = `
		DROP INDEX IF EXISTS users.idx_student_notes_school_day;
		DROP INDEX IF EXISTS users.uniq_student_notes_school_day_hint;
		ALTER TABLE users.student_notes
			DROP CONSTRAINT IF EXISTS chk_student_notes_school_class;
		ALTER TABLE users.student_notes
			DROP CONSTRAINT IF EXISTS chk_student_notes_school_hint;
		ALTER TABLE users.student_notes
			DROP CONSTRAINT IF EXISTS chk_student_notes_origin;
		ALTER TABLE users.student_notes
			ADD CONSTRAINT chk_student_notes_origin CHECK (origin IN ('staff', 'master_data'));
		ALTER TABLE users.student_notes
			DROP COLUMN IF EXISTS school_class;
		COMMENT ON COLUMN users.student_notes.origin IS
			'staff = von einer Person geschrieben; master_data = aus den früheren Betreuernotizen oder einem Import übernommen, daher ohne Autor.';
`

// schoolDayHintsUp extends the notes and grants class_day:day_hint_write to
// the lehrkraft role. Like class_day:arrival_exception_write it opens a door
// without widening a scope: the route re-checks the class assignment, and the
// school's setting operations.school_portal_write_scope decides whether the
// door is unlocked.
func schoolDayHintsUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSchoolDayHints(ctx, tx)
	if _, err := tx.ExecContext(ctx, schoolDayHintsUpSQL); err != nil {
		return fmt.Errorf("extend users.student_notes by school day hints: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        "class_day:day_hint_write",
		Description: "Tageshinweis für die OGS zu einer zugewiesenen Klasse oder einem ihrer Kinder schreiben (moto schule)",
		Resource:    "class_day",
		Action:      "day_hint_write",
	}, "lehrkraft")
}

// schoolDayHintsDown refuses while school hints exist: the restored origin
// check would reject them, and dropping them silently would lose what a
// Lehrkraft told the OGS.
func schoolDayHintsDown(ctx context.Context, db *bun.DB) error {
	var stored int
	if err := db.NewRaw(`SELECT COUNT(*) FROM users.student_notes WHERE origin = 'school'`).Scan(ctx, &stored); err != nil {
		return fmt.Errorf("check school day hints: %w", err)
	}
	if stored > 0 {
		return fmt.Errorf("users.student_notes holds %d school day hints; remove them before rolling back %s", stored, schoolDayHintsVersion)
	}
	if err := dropPermission(ctx, db, "class_day:day_hint_write"); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSchoolDayHints(ctx, tx)
	if _, err := tx.ExecContext(ctx, schoolDayHintsDownSQL); err != nil {
		return fmt.Errorf("remove school day hints from users.student_notes: %w", err)
	}
	return tx.Commit()
}

func rollbackSchoolDayHints(ctx context.Context, tx bun.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logRollbackFailure(ctx, err)
	}
}
