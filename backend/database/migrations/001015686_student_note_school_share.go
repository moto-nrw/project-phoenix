package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentNoteSchoolShareVersion     = "1.15.686"
	studentNoteSchoolShareDescription = "Kindnotiz für die Schule freigeben: Lehrkraft der Klasse liest freigegebene Beobachtungen der OGS (#4797)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentNoteSchoolShareVersion,
		Description: studentNoteSchoolShareDescription,
		DependsOn: []string{
			schoolAbsenceReportsVersion, // the last migration before this one
			studentNotesVersion,         // the table the share is a column of
			schoolDayHintsVersion,       // adds origin 'school', which is never shared back
		},
	})
	Migrations.MustRegister(studentNoteSchoolShareUp, studentNoteSchoolShareDown)
}

// studentNoteSchoolShareUpSQL lets the OGS share one note with the school
// (#4797). The Lehrkraft of the child's class reads the shared notes in moto
// schule; the Schulverwaltung does not.
//
//   - shared_with_school is off by default, so every existing note stays
//     invisible for the school until somebody of the OGS shares it.
//   - Only a note an OGS staff member wrote can be shared: a carried-over
//     master-data hint has no author to stand behind it, and a day hint
//     from moto schule already comes from the school.
//   - The partial index serves the one read of the school: the live shared
//     notes of one child.
const studentNoteSchoolShareUpSQL = `
		ALTER TABLE users.student_notes
			ADD COLUMN IF NOT EXISTS shared_with_school BOOLEAN NOT NULL DEFAULT FALSE;

		COMMENT ON COLUMN users.student_notes.shared_with_school IS
			'Für die Schule freigegeben (#4797): die Lehrkraft der Klasse des Kindes liest die Notiz in moto schule. Nur die OGS setzt und nimmt die Freigabe zurück.';

		ALTER TABLE users.student_notes
			DROP CONSTRAINT IF EXISTS chk_student_notes_school_share;
		ALTER TABLE users.student_notes
			ADD CONSTRAINT chk_student_notes_school_share CHECK (
				NOT shared_with_school OR origin = 'staff'
			);

		CREATE INDEX IF NOT EXISTS idx_student_notes_school_shared
			ON users.student_notes (tenant_id, student_id, created_at DESC)
			WHERE shared_with_school AND deleted_at IS NULL;
`

const studentNoteSchoolShareDownSQL = `
		DROP INDEX IF EXISTS users.idx_student_notes_school_shared;
		ALTER TABLE users.student_notes
			DROP CONSTRAINT IF EXISTS chk_student_notes_school_share;
		ALTER TABLE users.student_notes
			DROP COLUMN IF EXISTS shared_with_school;
`

func studentNoteSchoolShareUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, studentNoteSchoolShareUpSQL); err != nil {
			return fmt.Errorf("add shared_with_school to users.student_notes: %w", err)
		}
		return nil
	})
}

// studentNoteSchoolShareDown refuses while notes are shared: dropping the
// column would silently withdraw what the OGS released for the school.
func studentNoteSchoolShareDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		var shared int
		if err := tx.NewRaw(`SELECT COUNT(*) FROM users.student_notes WHERE shared_with_school`).Scan(ctx, &shared); err != nil {
			return fmt.Errorf("count shared student notes: %w", err)
		}
		if shared > 0 {
			return fmt.Errorf("users.student_notes holds %d notes shared with the school; withdraw them before rolling back %s", shared, studentNoteSchoolShareVersion)
		}
		if _, err := tx.ExecContext(ctx, studentNoteSchoolShareDownSQL); err != nil {
			return fmt.Errorf("drop shared_with_school: %w", err)
		}
		return nil
	})
}
